// Package hsts provides access to the Chromium HSTS preloaded list.
package hsts

import (
	"math/bits"
	"net/http"
	"strings"

	"golang.org/x/net/idna"
)

//go:generate go run -tags generate generate.go
//go:generate go fmt hsts_preload.go

// murmurHash computes the 32-bit Murmur3 hash of s using h as the seed.
func murmurHash(h uint32, s string) uint32 {
	const (
		c1 = 0xcc9e2d51
		c2 = 0x1b873593
		m  = 5
		n  = 0xe6546b64
	)

	for i := 0; i+4 <= len(s); i += 4 {
		b := s[i : i+4]
		k := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
		k *= c1
		k = bits.RotateLeft32(k, 15)
		k *= c2
		h ^= k
		h = bits.RotateLeft32(h, 13)
		h = h*m + n
	}

	var k uint32
	ntail := len(s) & 3
	itail := len(s) - ntail
	switch ntail {
	case 3:
		k = uint32(s[itail+2]) << 16
		fallthrough
	case 2:
		k |= uint32(s[itail+1]) << 8
		fallthrough
	case 1:
		k |= uint32(s[itail])
		k *= c1
		k = bits.RotateLeft32(k, 15)
		k *= c2
		h ^= k
	}

	h ^= uint32(len(s))
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}

// lookup searches for s and returns whether it includes subdomains and whether
// it was found.
func lookup(s string) (subdomains, ok bool) {
	idx := murmurHash(0, s)
	if seed := level0[idx&uint32(len(level0)-1)]; seed > 0 {
		idx = murmurHash(uint32(seed), s)
	}
	n := level1[idx&uint32(len(level1)-1)]
	len := n >> 24
	n &= 0x00ffffff
	return n < includeSubdomainsEnd, s == names[n:n+len]
}

// IsPreloaded reports whether host appears in the HSTS preloaded list.
func IsPreloaded(host string) bool {
	host = strings.TrimSuffix(host, ".")
	host, _ = idna.ToASCII(host)

	var truncated bool
	for dots := strings.Count(host, "."); dots > maxDots; dots-- {
		host = host[strings.IndexByte(host, '.')+1:]
		truncated = true
	}

	if host == "" {
		return false
	}

	host = strings.ToLower(host)
	for {
		subdomains, found := lookup(host)
		if found {
			return subdomains || !truncated
		}

		idx := strings.IndexByte(host, '.')
		if idx < 0 {
			break
		}
		host = host[idx+1:]
		truncated = true
	}

	return false
}

// Transport is a http.RoundTripper that transparently upgrades insecure http
// requests to secure https requests for hosts that appear in the HSTS
// preloaded list.
type Transport struct {
	// Base is the underlying http.RoundTripper to use or
	// http.DefaultTransport if nil.
	Base http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (rt *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	hostname := req.URL.Hostname()
	port := req.URL.Port()
	if req.URL.Scheme == "http" &&
		(port == "" || port == "80") &&
		IsPreloaded(hostname) {
		// Copy the request and it's URL before modifying.
		r := *req
		u := *r.URL
		u.Scheme = "https"
		u.Host = hostname // Remove port from URL.
		r.URL = &u
		req = &r
	}

	base := http.DefaultTransport
	if rt.Base != nil {
		base = rt.Base
	}

	return base.RoundTrip(req)
}
