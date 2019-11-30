// +build generate

package hsts

import _ "github.com/tmthrgd/go-filter" // used by generate.go

// MurmurHash computes the 32-bit Murmur3 hash of s using h as the seed.
func MurmurHash(h uint32, s string) uint32 {
	return murmurHash(h, s)
}
