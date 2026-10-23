<p align="center">
  <img src=".github/logo.svg" alt="Arandu" width="180">
</p>

# Arandu Playwright

A browser driver for the Arandu test kit that runs an application's E2E
journeys in **Chromium, Firefox and WebKit** through Playwright. It implements
`browser.Driver` from the framework's test kit,
`github.com/arandu-io/framework/test`, so a journey written against
`browser.Page` runs on it unchanged, and it passes the same conformance suite
as the default driver in all three engines.

The default driver, the kit's `browser/cdp`, drives Chrome over its debugging
pipe with the standard library alone, and is what most runs need. This module
is the optional second engine room:

- **cross-browser** runs, when a journey has to hold in Firefox and Safari's
  engine as well as in Chrome;
- **WebKit**, the closest a test on Linux or macOS gets to Safari;
- **nightly CI**, the full suite in the three engines, beside the `@smoke`
  run on every pull request with `cdp`.

## What it brings in

Read this before installing it.

- **playwright-go is community-maintained.** The bindings are
  `github.com/mxschmitt/playwright-go` (the `playwright-community/playwright-go`
  repository, whose module path moved there), not a Microsoft project.
- **It runs Node.** Playwright is a Node program. The bindings run it as a
  child process and speak to it over its standard input and output. The Node
  runtime, the `playwright-core` package and the browsers are downloaded at
  install time into the user's cache directory —
  `ms-playwright-go/1.62.1` and `ms-playwright` under `~/Library/Caches` on
  macOS, `~/.cache` on Linux, `%LOCALAPPDATA%` on Windows — **never into the
  project**. No `package.json`, no `node_modules`, nothing in the tree.
- **It is test tooling.** An application imports it from its tests only; none
  of it is linked into the binary it deploys.
- **`arandu.mod.toml` declares `network`, `filesystem` and `exec`**: the
  installer downloads, the driver and browsers live on disk, and Launch starts
  the Node driver, which starts the browsers.

## Install

```bash
go get github.com/hyz-is/arandu-playwright@v0.1.0
go run github.com/hyz-is/arandu-playwright/install-browsers
```

The second line puts the driver and the three browsers in the cache. Name the
engines to install fewer, and add `--with-deps` on a fresh Linux machine for
the system libraries the browsers need (it runs the package manager, as root):

```bash
go run github.com/hyz-is/arandu-playwright/install-browsers --with-deps webkit
```

It is what `aru e2e:install-browsers` runs. What is already installed is left
alone, so running it on every build costs a check. `Install`, `DriverDir`,
`BrowsersDir` and `Installed` are the same thing as a Go API.

The tests never download anything. A missing driver or browser makes `Launch`
answer `browser.ErrNoBrowser`, which skips the test on a developer's machine
and fails it in CI, with the command to run in the message.

## Register it

The drivers an application can run with are written in one line of
`tests/TestCase.go`. There is no registration by import:

```go
import (
	"github.com/arandu-io/framework/test/browser"
	"github.com/arandu-io/framework/test/browser/cdp"
	playwright "github.com/hyz-is/arandu-playwright"
)

// Drivers are the browser drivers the E2E journeys run with.
// ARANDU_E2E_DRIVER picks one, ARANDU_E2E_BROWSER the engine.
var Drivers = browser.Registry{"cdp": cdp.Launch, "playwright": playwright.Launch}
```

A journey opens its page through the registry and never names a driver:

```go
//go:build e2e

func TestInvoiceIsPaid(t *testing.T) {
	page := browser.Open(t, tests.Drivers, browser.BaseURL(server.URL), browser.Routes(server.Route))
	page.Visit("invoices.show", invoice.ID)
	page.ClickRole("button", "Pay")
	page.ExpectText("Paid")
}
```

## Run it

```bash
ARANDU_E2E_DRIVER=playwright ARANDU_E2E_BROWSER=webkit  go test -tags e2e ./tests/E2E/...
ARANDU_E2E_DRIVER=playwright ARANDU_E2E_BROWSER=firefox go test -tags e2e ./tests/E2E/...
```

| Variable | Values | |
| --- | --- | --- |
| `ARANDU_E2E_DRIVER` | `cdp`, `playwright` | the driver; `cdp` when empty and both are registered |
| `ARANDU_E2E_BROWSER` | `chromium`, `firefox`, `webkit` | the engine; `chrome` and `safari` are read as `chromium` and `webkit` |
| `ARANDU_E2E_HEADED` | `1` | shows the browser window |
| `ARANDU_E2E_SLOWMO` | `250ms`, `250` | delays every navigation, click and key |
| `PLAYWRIGHT_DRIVER_PATH`, `PLAYWRIGHT_BROWSERS_PATH` | a directory | move the driver and the browsers out of the default cache |

The trace a failing journey writes is the kit's own format, the same for every
driver: the steps with a PNG and the DOM, the network and the console.

## How it behaves

- **One browser per engine, one context per driver.** The first `Launch` in a
  test binary starts Playwright and the browser; every `Launch` opens a fresh
  browsing context in it, so two drivers share no cookie, storage or cache. A
  browser no driver uses closes after three seconds, and Playwright with the
  last one.
- **Events** are Playwright's, in the kit's vocabulary: console messages
  (`console-api` for the page's own `console.*`, the browser's notices
  otherwise), uncaught exceptions, every request with its response, end and
  failure, and the main frame's navigations.
- **Network failures carry Chromium's names.** Firefox's
  `NS_ERROR_CONNECTION_REFUSED` and WebKit's "Could not connect to the server"
  arrive as `net::ERR_CONNECTION_REFUSED (…)`, with the browser's words kept.
  A port the Fetch standard blocks is refused with `net::ERR_UNSAFE_PORT`
  before navigating, because Firefox and WebKit report it as an interrupted or
  a never-ending navigation.
- **Viewport.** Width and height change on every engine. A device scale other
  than 1, or mobile emulation, changes on a running Chromium page and is an
  error on Firefox and WebKit, which fix both when the context is created.
- **Keys.** `Press("Shift+Tab")` holds Shift while Tab is pressed, as a
  keyboard does, so the page sees a `keydown` for Shift first.

## CI

Cache the two directories and install before the tests; the install is a
no-op on a warm cache:

```yaml
- uses: actions/cache@v6
  with:
    path: |
      ~/.cache/ms-playwright
      ~/.cache/ms-playwright-go
    key: playwright-${{ runner.os }}-${{ hashFiles('go.sum') }}
- run: go run github.com/hyz-is/arandu-playwright/install-browsers --with-deps
- run: ARANDU_E2E_DRIVER=playwright ARANDU_E2E_BROWSER=webkit go test -tags e2e ./tests/E2E/...
```

`--with-deps` installs system packages, which the cache does not hold, so it
stays on the line even when the browsers are cached.

## Licence

MIT. See [LICENSE.md](LICENSE.md).
