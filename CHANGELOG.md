# Changelog

Everything worth knowing about a release of Arandu Playwright is recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A published module version is immutable: Go serves it from the proxy forever, so
a release is corrected by another release and never by moving a tag.

## [Unreleased]


## [0.1.0] - 2026-10-23

### Added

- `Launch`, a `browser.Driver` for the framework's test kit (`framework/test/browser`) over playwright-go (github.com/mxschmitt/playwright-go v0.6201.1, Playwright 1.62.1), driving Chromium, Firefox and WebKit chosen by `ARANDU_E2E_BROWSER`; passes `framework/test/browser/conformance` in the three engines.
- One Playwright process and one browser per engine per test binary, a fresh browsing context per driver, closed after a three-second idle period.
- Headed runs and slow motion from `browser.Options`; slow motion delays navigation and input only.
- Network failures in Firefox and WebKit carry Chromium's `net::ERR_` names with the browser's text; ports blocked by the Fetch standard fail with `net::ERR_UNSAFE_PORT` before navigating.
- `Install`, `DriverDir`, `BrowsersDir`, `Installed` and the `install-browsers` command, which install the driver and browsers into the user's cache; `Launch` downloads nothing and answers `browser.ErrNoBrowser` when they are missing.
- `arandu.mod.toml` declares network, filesystem and exec; requires Framework v0.63.0.
