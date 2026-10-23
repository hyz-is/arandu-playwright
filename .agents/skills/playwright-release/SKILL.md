---
name: playwright-release
description: Get this driver green, declared and published. Use when the request is to "run the gates", "the conformance suite fails", "test in the three browsers", "install the browsers for the tests", "go build says the directory prefix does not contain modules listed in go.work", "update arandu.mod.toml", "bump playwright-go", "bump the framework", "tag a release", "write the changelog", or before opening a pull request. Covers the gates and the e2e run in three engines, what the manifest declares and how it is audited, bumping the bindings, and the immutability of a published version.
license: MIT
---

# Getting it green and out

## The gates

```sh
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
go vet -tags e2e ./...
go run ./install-browsers
for b in chromium firefox webkit; do ARANDU_E2E_BROWSER=$b go test -tags e2e -count=1 ./...; done
```

The e2e line runs `conformance.Run` from the kit in `tests/E2E` and the
lifecycle tests beside the code. A failure in one engine is a failure: the
point of this driver is that the three behave alike. When an engine says
something differently, translate it into the kit's vocabulary in the driver
(see `convert.go`); when the suite itself assumes one engine, report it to the
kit rather than special-casing the test here.

CI runs the same, with the browser cache, in the `conformance` job.

## The manifest

```toml
[permissions]
network = true     # Install downloads the driver and the browsers
filesystem = true  # they live in the user's cache
exec = true        # Launch runs the Node driver, which runs the browsers
migrations = false
```

`tests/Unit/audit_test.go` compares it with the code by AST, both directions,
and counts the playwright-go entry points in its `delegated` table as the
capabilities they exercise. A new call into playwright-go that downloads,
writes or starts a process goes into that table in the same commit.

## Bumping playwright-go

The bindings pin one Playwright release (`playwrightCliVersion` in their
`run.go`), and browsers are built per Playwright release. A bump moves
`driverVersion` in `install.go` to the same value, and the conformance suite
runs in the three engines after `go run ./install-browsers` fetched the new
builds. The module path is `github.com/mxschmitt/playwright-go`.

## Publishing

A published Go module version is immutable: a mistake is corrected by another
release, never by moving a tag. Before a tag, the gates pass in the three
engines, `arandu.mod.toml` matches the code, and `CHANGELOG.md` has the entry
under its version. While the version starts with `v0.`, breaking is allowed
and every break is named in `UPGRADE.md`; the CI's API diff fails otherwise.
