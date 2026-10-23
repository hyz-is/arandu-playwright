# Contributing

Thank you for considering a contribution.

## Before you open a pull request

Run the four gates. They are what CI runs, and a pull request that fails one of
them fails for everybody who pulls after it.

```bash
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

The two filters on `gofmt` are not preference. `gofmt` is the only tool in the
chain that ignores build tags, so it reads files the compiler deliberately
excludes; `testdata/` holds fixtures that are invalid on purpose.

A change to the driver also passes the conformance suite in the three
engines, which needs the driver and the browsers in the user's cache:

```bash
go run ./install-browsers
for b in chromium firefox webkit; do ARANDU_E2E_BROWSER=$b go test -tags e2e -count=1 ./...; done
```

## What a change has to keep true

1. **The contract, and nothing beyond it.** A capability `browser.Driver` does
   not have is proposed to the test kit, not added here.
2. **Every engine answers alike**, in the kit's vocabulary, with the browser's
   own words kept beside it.
3. **`Launch` downloads nothing.** Only `Install` does.
4. **`arandu.mod.toml` matches the code**, including what playwright-go does on
   this package's behalf.

## Style

Everything in the source is in English: identifiers, doc comments, internal
comments, error messages, log messages, and the names and messages of tests.
`pkg.go.dev` publishes the doc comments, and its readers are users of this
package.

Every exported symbol carries a doc comment saying what it does. The comment
documents the symbol and nothing else: no dates, no issue numbers, no
references to other repositories. Why a signature is what it is belongs there
when it is a fact about the code; anything else belongs in the pull request.

## Commits

One commit per logical change. Formatting is not mixed with behaviour, because
a diff that does both is a diff nobody reviews.

## Reporting a vulnerability

Do not open an issue. See [SECURITY.md](SECURITY.md).

## Licence

By contributing you agree that your contribution is licensed under the MIT
licence, the same as the rest of github.com/hyz-is/arandu-playwright.
