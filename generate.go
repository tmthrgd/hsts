//go:build ignore
// +build ignore

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	filter "github.com/tmthrgd/go-filter"
	"go.tmthrgd.dev/hsts"
)

const jsonURL = "https://chromium.googlesource.com/chromium/src/net/+/main/http/transport_security_state_static.json?format=TEXT"

func main() {
	if err := main1(); err != nil {
		log.Fatal(err)
	}
}

func main1() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jsonURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned unexpected %s status code", resp.Status)
	}

	br := filter.NewReader(
		base64.NewDecoder(base64.StdEncoding, resp.Body),
		func(line []byte) bool {
			line = bytes.TrimSpace(line)
			return !(len(line) >= 2 && string(line[:2]) == "//")
		})

	var rules struct {
		Entries []struct {
			Name string

			IncludeSubdomains bool `json:"include_subdomains"`

			Mode string
			//  "force-https" iff covered names should require HTTPS.
		}
	}
	if err := json.NewDecoder(br).Decode(&rules); err != nil {
		return err
	}

	sort.Slice(rules.Entries, func(i, j int) bool {
		ei, ej := rules.Entries[i], rules.Entries[j]
		if ei.IncludeSubdomains != ej.IncludeSubdomains {
			return ei.IncludeSubdomains
		}

		return ei.Name < ej.Name
	})

	names := make([]string, 0, len(rules.Entries))
	nameIdxs := make([]uint32, 0, len(rules.Entries))
	var namesIdx, includeSubdomainsEnd, maxDots int
	for _, entry := range rules.Entries {
		if entry.Mode != "force-https" {
			continue
		}

		names = append(names, entry.Name)

		packed := packNameIndexLen(namesIdx, len(entry.Name))
		nameIdxs = append(nameIdxs, packed)
		namesIdx += len(entry.Name)

		if entry.IncludeSubdomains {
			includeSubdomainsEnd += len(entry.Name)
		}

		dots := strings.Count(entry.Name, ".")
		if dots > maxDots {
			maxDots = dots
		}
	}

	level0, level1 := build(names)

	f, err := os.Create("hsts_preload.go")
	if err != nil {
		return err
	}

	bw := bufio.NewWriter(f)
	fmt.Fprintln(bw, "// Code generated by go run generate.go. DO NOT EDIT.")
	fmt.Fprintln(bw, "//")
	fmt.Fprintln(bw, "// This file was generated from the Chromium HSTS preloaded list")
	fmt.Fprintf(bw, "// %s.\n", jsonURL)
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, "package hsts")
	fmt.Fprintln(bw)
	fmt.Fprintf(bw, "const names = %q\n", strings.Join(names, ""))
	fmt.Fprintln(bw)
	fmt.Fprintf(bw, "var level0 = %#v\n", level0)
	fmt.Fprintln(bw)
	fmt.Fprint(bw, "var level1 = []uint32{")
	for i, n := range level1 {
		if i > 0 {
			fmt.Fprint(bw, ", ")
		}
		fmt.Fprintf(bw, "%#x", nameIdxs[n])
	}
	fmt.Fprintln(bw, "}")
	fmt.Fprintln(bw)
	fmt.Fprintf(bw, "const includeSubdomainsEnd = %d\n", includeSubdomainsEnd)
	fmt.Fprintln(bw)
	fmt.Fprintf(bw, "const maxDots = %d\n", maxDots)

	if err := bw.Flush(); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	return f.Close()
}

func packNameIndexLen(idx, len int) uint32 {
	if int(uint32(idx)&0x00ffffff) != idx {
		panic("name index out of range")
	}
	if int(uint8(len)) != len {
		panic("name len out of range")
	}

	return uint32(idx) | uint32(len)<<24
}

// build builds a table from keys using the "Hash, displace, and compress"
// algorithm described in http://cmph.sourceforge.net/papers/esa09.pdf.
func build(keys []string) (level0 []uint16, level1 []uint32) {
	level0 = make([]uint16, nextPow2(len(keys)/4))
	level0Mask := len(level0) - 1
	level1 = make([]uint32, nextPow2(len(keys)))
	level1Mask := len(level1) - 1
	sparseBuckets := make([][]int, len(level0))
	for i, s := range keys {
		n := int(hsts.MurmurHash(0, s)) & level0Mask
		sparseBuckets[n] = append(sparseBuckets[n], i)
	}
	type indexBucket struct {
		n    int
		vals []int
	}
	var buckets []indexBucket
	for n, vals := range sparseBuckets {
		if len(vals) > 0 {
			buckets = append(buckets, indexBucket{n, vals})
		}
	}
	sort.Slice(buckets, func(i, j int) bool {
		return len(buckets[i].vals) > len(buckets[j].vals)
	})

	occ := make([]bool, len(level1))
	var tmpOcc []int
	for _, bucket := range buckets {
		var seed uint32
	trySeed:
		tmpOcc = tmpOcc[:0]
		for _, i := range bucket.vals {
			n := int(hsts.MurmurHash(seed, keys[i])) & level1Mask
			if occ[n] {
				for _, n := range tmpOcc {
					occ[n] = false
					level1[n] = 0
				}
				seed++
				goto trySeed
			}
			occ[n] = true
			tmpOcc = append(tmpOcc, n)
			level1[n] = uint32(i)
		}
		if uint32(uint16(seed)) != seed {
			panic(fmt.Sprintf("unable to find valid seed for table (found seed %d)", seed))
		}
		level0[bucket.n] = uint16(seed)
	}

	return level0, level1
}

func nextPow2(n int) int {
	for i := 1; i > 0; i *= 2 {
		if i >= n {
			return i
		}
	}

	panic("overflow")
}
