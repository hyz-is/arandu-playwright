# Upgrade Guide

## v0.2.0


Go 1.26.10 is the floor: `go.mod` declares `go 1.26.10`, because every
vulnerability an application reaches through this module is in the standard
library and fixed in that release (crypto/x509, net/http HTTP/2 and CONNECT,
html/template, net/textproto, crypto/tls). Build with Go 1.26.10 or newer.

The module requires framework v0.66.0 and hesape v0.85.0, and the `framework`
floor in `arandu.mod.toml` rises to `>= 0.66`: an application on an older
framework moves to v0.66.0 in the same `go get`. Nothing in this module's API
changed, and `playwright.Launch` is registered in `tests/TestCase.go` as before.
