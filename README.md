# hsts

[![GoDoc](https://godoc.org/go.tmthrgd.dev/hsts?status.svg)](https://godoc.org/go.tmthrgd.dev/hsts)
[![Build Status](https://travis-ci.com/tmthrgd/hsts.svg?branch=master)](https://travis-ci.com/tmthrgd/hsts)

Package hsts provides access to the Chromium HSTS preloaded list.

The list is manually converted into go code occasionally with `go generate`.

To request your site be added to the list, please visit
[hstspreload.org](https://hstspreload.org/).

## Usage

```go
import "go.tmthrgd.dev/hsts"
```

Hostnames (or domain names) can be checked for inclusion in the list by calling
[`IsPreloaded`](https://godoc.org/go.tmthrgd.dev/hsts#IsPreloaded):

```go
if hsts.IsPreloaded(hostname) {
	// ...
}
```

[`Transport`](https://godoc.org/go.tmthrgd.dev/hsts#Transport) is a
[`http.RoundTripper`](https://golang.org/pkg/net/http/#RoundTripper) that
automatically upgrades insecure http requests to hosts in the preload list into
secure https requests. It can be used by setting the `(*http.Client).Transport`
field as follows:

```go
client := &http.Client{
	Transport: &hsts.Transport{},
}
resp, err := client.Do(req)
```

## License

[BSD 3-Clause License](LICENSE)

The minimal perfect hashing code was taken from
[cespare/mph](https://github.com/cespare/mph/tree/ecff71bf0208e9325253df11d1cd0e26c7a7a813)
by [Caleb Spare](https://github.com/cespare) and is licensed under an
[MIT License](https://github.com/cespare/mph/blob/ecff71bf0208e9325253df11d1cd0e26c7a7a813/LICENSE.txt).
