// Package hsts provides access to the Chromium HSTS preloaded list.
package hsts

import (
	"net"
	"net/http"
	"strings"

	"golang.org/x/net/idna"
)

//go:generate go run generate.go
//go:generate go fmt hsts_preload.go

// IsPreloaded reports whether host appears in the HSTS preloaded list.
func IsPreloaded(host string) bool {
	if net.ParseIP(host) != nil {
		return false
	}

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
		subdomain, found := preloadList[host]
		if found {
			return subdomain || !truncated
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
		// WithContext currently copies the http.Request URL field and
		// is more lightweight than Clone. See golang.org/issue/23544.
		req = req.WithContext(req.Context())
		req.URL.Scheme = "https"
		req.URL.Host = hostname // Remove port from URL.
	}

	base := http.DefaultTransport
	if rt.Base != nil {
		base = rt.Base
	}

	return base.RoundTrip(req)
}
