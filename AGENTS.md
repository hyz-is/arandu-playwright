# Working on Arandu Playwright

This is an Arandu package with no route, no table and no policy: a browser
driver. It implements `browser.Driver` from the framework's test kit
(`framework/test/browser`) over playwright-go, for Chromium, Firefox and
WebKit, and an application registers
it in one line of `tests/TestCase.go`. Nothing of it reaches the binary an
application deploys.

Read `.agents/skills/` before writing code. Each skill is a procedure, and the
one you need is named by the situation you are in.

## The gates

Nothing is finished until all of them exit zero.

```sh
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
go vet -tags e2e ./...
for b in chromium firefox webkit; do ARANDU_E2E_BROWSER=$b go test -tags e2e -count=1 ./...; done
```

The last line is the driver's real proof: the kit's conformance suite
(`tests/E2E`) and the lifecycle tests beside the code, in the three engines. It
needs the driver and the browsers in the user's cache:

```sh
go run ./install-browsers
```

`GOWORK=off` is not a preference. This checkout may sit beside a Go workspace
that does not list it, and every command above then fails before compiling:
*"directory prefix . does not contain modules listed in go.work"*.

`aru doctor` reads applications and refuses this directory. What it would
check about the manifest is held by `tests/Unit/audit_test.go`.

## What this repository holds

```
playwright.go     Launch, Driver and the browser.Driver methods
events.go         Playwright's page events in the kit's vocabulary
runtime.go        the Playwright process and the browsers, shared per test binary
convert.go        keys, cookies, evaluation results, network error names, blocked ports
install.go        Install, DriverDir, BrowsersDir, Installed
install-browsers/ the command `go run` fetches to install
tests/Unit        manifest audit and the exported install API
tests/E2E         conformance.Run, under the e2e tag
```

## What does not exist here

| A model reaches for | What is here instead |
| --- | --- |
| registration by import, an `init()` that fills a registry | `playwright.Launch`, written into the application's `browser.Registry` by hand |
| a download inside `Launch` | `browser.ErrNoBrowser` naming the install command; only `Install` downloads |
| Node, `package.json` or a browser in the tree | the user's cache, through playwright-go |
| Playwright's own API in a journey | `browser.Page`; a test that calls one driver's API stops running on the others |
| `time.Sleep` to wait for the page | the page's own waits; the driver waits only for what it reports |
| Playwright's `slowMo` | the driver's own delay before navigation and input, so evaluations the page polls with stay fast |
| a call to Playwright from an event handler | nothing: handlers run on playwright-go's reading goroutine and only read what the event carries |

## The properties

1. **The contract, and nothing beyond it.** Every exported method is a
   `browser.Driver` method or install tooling. A capability the contract lacks
   is proposed to the kit, not added here.
2. **Every engine behaves alike.** Where Firefox and WebKit say a thing
   differently from Chromium — network errors, blocked ports — the driver
   answers in the kit's vocabulary and keeps the browser's words beside it.
3. **Launch downloads nothing.** `TestAMissingDriverSkipsRatherThanDownloads`
   holds it.
4. **`arandu.mod.toml` matches the code**, counting what playwright-go does on
   the package's behalf; `TestTheDeclaredCapabilitiesAreWhatTheCodeDoes` holds
   both directions.

## Writing code

Everything in the source is in English: identifiers, doc comments, comments,
errors and test names. Every exported symbol carries a doc comment that
documents the symbol and nothing else — no dates, issue numbers or other
repositories.

Tests go under `tests/`, in a capitalized category declaring a lowercase
external package (`tests/Unit` is `package unit_test`). A test that needs the
unexported goes beside the code as `*_internal_test.go`; one that starts a
browser carries the `e2e` build tag.
