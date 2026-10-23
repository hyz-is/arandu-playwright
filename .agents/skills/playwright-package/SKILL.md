---
name: playwright-package
description: Install and register the Arandu Playwright browser driver (Go, Arandu) in an application's tests. Use when the request is to "run the E2E tests in Firefox", "test in Safari", "test in WebKit", "cross-browser tests", "install arandu-playwright", "go get github.com/hyz-is/arandu-playwright", "ARANDU_E2E_DRIVER=playwright", "aru e2e:install-browsers", "install the Playwright browsers", "the e2e test skips with no browser was found", "cache the browsers in CI", or when a project's go.mod already requires github.com/hyz-is/arandu-playwright. Covers the install, the one line in tests/TestCase.go, the environment that picks the driver and the engine, CI caching, and what differs between engines.
license: MIT
metadata:
  audience: app
---

# Using Arandu Playwright

A browser driver for the Arandu test kit that runs the E2E journeys in
Chromium, Firefox and WebKit through Playwright. The default driver, `cdp`,
stays the default; this one is for cross-browser and nightly runs.

It is test tooling: imported from `tests/` only, never from `bootstrap/app.go`,
and nothing of it reaches the deployed binary.

## Install

```bash
go get github.com/hyz-is/arandu-playwright
go run github.com/hyz-is/arandu-playwright/install-browsers
```

The second line downloads the Playwright driver (Node and `playwright-core`)
and the three browsers into the user's cache, never into the project. Do not
add a `package.json` or install Playwright with npm: the module runs its own
copy. `--with-deps` installs the system libraries on Linux; naming engines
installs fewer.

## Register it

One line in `tests/TestCase.go`, beside `cdp`:

```go
import (
	"github.com/arandu-io/framework/test/browser"
	"github.com/arandu-io/framework/test/browser/cdp"
	playwright "github.com/hyz-is/arandu-playwright"
)

var Drivers = browser.Registry{"cdp": cdp.Launch, "playwright": playwright.Launch}
```

Journeys open pages with `browser.Open(t, tests.Drivers, ...)` and never call
Playwright directly. A journey that does stops running on `cdp`.

## Run

```bash
ARANDU_E2E_DRIVER=playwright ARANDU_E2E_BROWSER=webkit go test -tags e2e ./tests/E2E/...
```

`ARANDU_E2E_BROWSER` is `chromium`, `firefox` or `webkit`. `ARANDU_E2E_HEADED=1`
shows the window, `ARANDU_E2E_SLOWMO=250ms` slows navigation and input.

A test that says "no browser was found" on a laptop is skipped because the
driver or the browser is not installed: run the install line. In CI the same
condition fails the test, as it should.

## CI

Cache `~/.cache/ms-playwright` and `~/.cache/ms-playwright-go`, then run the
install line (with `--with-deps` on a fresh runner) before the tests. Run
Playwright in the nightly job; the pull-request job runs `@smoke` with `cdp`.

## What differs between engines

- Network failures read `net::ERR_…` on every engine, with the browser's own
  text in parentheses.
- A port the Fetch standard blocks fails with `net::ERR_UNSAFE_PORT` without
  navigating.
- A viewport with a device scale other than 1 or mobile emulation works on
  Chromium only; Firefox and WebKit fix both when the browsing context opens.
- `Press("Shift+Tab")` sends a `keydown` for Shift before Tab, as a keyboard
  does.
