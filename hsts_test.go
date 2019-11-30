package hsts

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPreloaded(t *testing.T) {
	for _, host := range []string{
		"very.long.domain.name.tomthorogood.uk",
		"www.tomthorogood.uk",
		"tomthorogood.uk",

		"tmthrgd.dev",
		"dev",

		"xn--7xa.google.com", // "φ.google.com"
		"φ.google.com",

		"www.g.co",
		"g.co",

		"zzw.ca",
		"www.zzw.ca",

		"1.0.0.1",
	} {
		assert.True(t, IsPreloaded(host), host)
	}

	for _, host := range []string{
		"www.example.uk",
		"example.uk",
		"uk",

		"www.example.com",
		"example.com",
		"com",

		"www.example.net",
		"example.net",
		"net",

		"test.g.co",

		"www.1.0.0.1",
	} {
		assert.False(t, IsPreloaded(host), host)
	}
}

func TestIsPreloadedAllocs(t *testing.T) {
	allocs := testing.AllocsPerRun(10, func() {
		for _, host := range []string{
			"very.long.domain.name.tomthorogood.uk",
			"www.tomthorogood.uk",
			"tomthorogood.uk",

			"tmthrgd.dev",
			"dev",

			"www.example.uk",
			"example.uk",
			"uk",

			"www.example.com",
			"example.com",
			"com",

			"www.example.net",
			"example.net",
			"net",

			"test.g.co",
			"www.g.co",
			"g.co",
		} {
			IsPreloaded(host)
		}
	})
	assert.Zero(t, allocs)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func TestTransport(t *testing.T) {
	var got *http.Request
	tr := &Transport{
		Base: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			got = r
			return nil, nil
		}),
	}

	for _, tc := range []struct {
		url, expect string
	}{
		{"http://tomthorogood.uk", "https://tomthorogood.uk"},
		{"http://www.tomthorogood.uk/path/to", "https://www.tomthorogood.uk/path/to"},
		{"http://tomthorogood.uk:80/example?example", "https://tomthorogood.uk/example?example"},
		{"http://user:pass@www.g.co/path/to?example", "https://user:pass@www.g.co/path/to?example"},
		{"http://xn--7xa.google.com", "https://xn--7xa.google.com"}, // "φ.google.com"
		{"http://%CF%86.google.com", "https://%CF%86.google.com"},   // "φ.google.com"

		{"https://tomthorogood.uk", "https://tomthorogood.uk"},
		{"https://www.tomthorogood.uk/path/to", "https://www.tomthorogood.uk/path/to"},
		{"https://tomthorogood.uk:443/example?example", "https://tomthorogood.uk:443/example?example"},
		{"https://tomthorogood.uk:8443/example?example", "https://tomthorogood.uk:8443/example?example"},
		{"https://user:pass@www.g.co/path/to?example", "https://user:pass@www.g.co/path/to?example"},
		{"https://xn--7xa.google.com", "https://xn--7xa.google.com"}, // "φ.google.com"
		{"https://%CF%86.google.com", "https://%CF%86.google.com"},   // "φ.google.com"

		{"http://tomthorogood.uk:8080", "http://tomthorogood.uk:8080"},
		{"http://example.com", "http://example.com"},
		{"http://test.g.co", "http://test.g.co"},
		{"http://test.g.co:80", "http://test.g.co:80"},
		{"http://user:pass@test.g.co:8080/path/to?example", "http://user:pass@test.g.co:8080/path/to?example"},

		{"ftp://tomthorogood.uk", "ftp://tomthorogood.uk"},

		{"file:///etc/hosts", "file:///etc/hosts"},
		{"file://host/etc/hosts", "file://host/etc/hosts"},
		{"file://tomthorogood.uk/etc/hosts", "file://tomthorogood.uk/etc/hosts"},
	} {
		req, err := http.NewRequest(http.MethodGet, tc.url, nil)
		require.NoError(t, err)

		tr.RoundTrip(req)
		if !assert.Equal(t, tc.expect, got.URL.String(), tc) {
			continue
		}

		if tc.url != tc.expect {
			assert.False(t, got == req, "*Transport didn't copy *http.Request")
			assert.False(t, got.URL == req.URL, "*Transport didn't copy *url.URL")
		}
	}
}

func BenchmarkIsPreloaded(b *testing.B) {
	for n := 0; n < b.N; n++ {
		IsPreloaded("tmthrgd.dev")
	}
}
