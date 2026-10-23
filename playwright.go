// Package playwright is a browser driver for the Arandu test kit that drives
// Chromium, Firefox and WebKit through Playwright, by way of the
// community-maintained playwright-go bindings.
//
// It implements browser.Driver, so a test written against browser.Page runs on
// it unchanged. It is registered in one line of the application's test case,
// beside the default driver:
//
//	var Drivers = browser.Registry{"cdp": cdp.Launch, "playwright": playwright.Launch}
//
// and chosen for a run with ARANDU_E2E_DRIVER=playwright, the engine with
// ARANDU_E2E_BROWSER=chromium, firefox or webkit.
//
// Playwright is a Node program. playwright-go runs it as a child process and
// talks to it over its standard input and output; the Node runtime, the
// Playwright package and the browsers live in the user's cache directory, never
// in the project. Launch downloads nothing: a missing driver or browser is an
// error wrapping browser.ErrNoBrowser, which skips the test on a developer's
// machine and fails it in CI. Install puts them in place.
//
// One Playwright process and one browser process per engine serve every
// driver launched in a test binary; each Launch opens a browsing context of its
// own in that browser, so two drivers share no cookie, storage or cache. A
// browser no driver uses is closed after a short idle period, and Playwright
// with the last of them.
package playwright

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/arandu-io/framework/test/browser"
	pw "github.com/mxschmitt/playwright-go"
)

// Name is the name the driver reports in browser.Info, and the name it is
// meant to be registered under.
const Name = "playwright"

// Driver is one browsing context with one page, in a browser shared with the
// other drivers of the same engine.
type Driver struct {
	engine  string
	version string
	slowmo  time.Duration
	shared  *sharedBrowser
	context pw.BrowserContext
	page    pw.Page

	// cdp is the DevTools session of a Chromium page, opened the first time a
	// viewport asks for what Playwright fixes when the context is created.
	cdpOnce sync.Once
	cdp     pw.CDPSession
	cdpErr  error

	events *events

	closeOnce sync.Once
	closeErr  error
}

var _ browser.Driver = (*Driver)(nil)

// Launch opens a browsing context in the engine opts.Browser names --
// "chromium" (the default), "firefox" or "webkit" -- and a page in it. ctx
// bounds the start only. Headed shows the browser window; SlowMo delays every
// navigation and input. It matches browser.Factory.
func Launch(ctx context.Context, opts browser.Options) (browser.Driver, error) {
	engine, err := engineName(opts.Browser)
	if err != nil {
		return nil, err
	}
	shared, err := acquire(ctx, engine, opts.Headed)
	if err != nil {
		return nil, err
	}
	d := &Driver{
		engine:  engine,
		version: shared.browser.Version(),
		slowmo:  opts.SlowMo,
		shared:  shared,
		events:  newEvents(),
	}
	err = do(ctx, func() error {
		bc, err := shared.browser.NewContext(pw.BrowserNewContextOptions{
			Viewport:          &pw.Size{Width: 1440, Height: 900},
			DeviceScaleFactor: pw.Float(1),
		})
		if err != nil {
			return err
		}
		d.context = bc
		page, err := bc.NewPage()
		if err != nil {
			return err
		}
		d.page = page
		return nil
	})
	if err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("playwright: open a page in %s: %w", engine, err)
	}
	d.events.attach(d.page)
	return d, nil
}

// engineName turns what ARANDU_E2E_BROWSER says into a Playwright engine.
func engineName(asked string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(asked)) {
	case "", "chromium", "chrome":
		return "chromium", nil
	case "firefox":
		return "firefox", nil
	case "webkit", "safari":
		return "webkit", nil
	}
	return "", fmt.Errorf("playwright: drives chromium, firefox and webkit, and ARANDU_E2E_BROWSER asks for %q", asked)
}

// Info names the driver, the engine and the browser's version.
func (d *Driver) Info() browser.Info {
	return browser.Info{Driver: Name, Browser: d.engine, Version: d.version}
}

// Listen hands every event from now on to fn.
func (d *Driver) Listen(fn func(browser.Event)) { d.events.listen(fn) }

// Navigate loads url and returns once the document's load event fired.
func (d *Driver) Navigate(ctx context.Context, url string) error {
	if err := d.slow(ctx); err != nil {
		return err
	}
	if port, blocked := blockedPort(url); blocked {
		return fmt.Errorf("navigate to %s: net::ERR_UNSAFE_PORT (port %s is blocked by the Fetch standard, and %s refuses it)", url, port, d.engine)
	}
	err := do(ctx, func() error {
		_, err := d.page.Goto(url, pw.PageGotoOptions{WaitUntil: pw.WaitUntilStateLoad, Timeout: timeout(ctx)})
		return err
	})
	if err != nil {
		d.awaitErrorDocument(ctx)
		return fmt.Errorf("navigate to %s: %s", url, networkError(firstLine(message(err))))
	}
	return nil
}

// awaitErrorDocument waits, briefly, for the error document Chromium commits
// after a refused navigation. An evaluation sent before it settles would land
// in the document it replaces. Firefox and WebKit commit none under
// Playwright.
func (d *Driver) awaitErrorDocument(ctx context.Context) {
	if d.engine != "chromium" {
		return
	}
	wait, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		var at struct {
			URL   string `json:"url"`
			Ready string `json:"ready"`
		}
		if d.Evaluate(wait, `({url: location.href, ready: document.readyState})`, &at) == nil &&
			strings.HasPrefix(at.URL, "chrome-error:") && at.Ready == "complete" {
			return
		}
		select {
		case <-tick.C:
		case <-wait.Done():
			return
		}
	}
}

// firstLine is the first line of a Playwright error, without the call log it
// appends.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return strings.TrimSpace(line)
}

// evaluation wraps an expression so the page answers with its JSON text, the
// way a DevTools evaluation by value does, and says apart a value that has no
// JSON form. A promise answers with what it resolves to. The expression runs
// synchronously first, so one that throws fails the evaluation without
// leaving a rejected promise behind, which Firefox would report as an
// exception the page did not catch.
const evaluation = "() => {\n" +
	"const envelope = v => { if (v === undefined) return { kind: 'undefined' }; const j = JSON.stringify(v); return j === undefined ? { kind: typeof v } : { kind: 'json', json: j }; };\n" +
	"const v = (\n%s\n);\n" +
	"return v !== null && typeof v === 'object' && typeof v.then === 'function' ? Promise.resolve(v).then(envelope) : envelope(v);\n}"

// documentGone are the errors Playwright answers an evaluation with when the
// document it ran in was replaced under it.
var documentGone = []string{
	"Execution context was destroyed",
	"Cannot find context with specified id",
	"Frame was detached",
	"frame got detached",
	"Inspected target navigated or closed",
}

// Evaluate runs expression in the page's main frame and decodes its JSON
// value into result.
func (d *Driver) Evaluate(ctx context.Context, expression string, result any) error {
	var answer any
	err := do(ctx, func() error {
		var err error
		answer, err = d.page.Evaluate(fmt.Sprintf(evaluation, expression))
		return err
	})
	if err != nil {
		text := message(err)
		for _, gone := range documentGone {
			if strings.Contains(text, gone) {
				return fmt.Errorf("%w: %s", browser.ErrDocumentChanged, text)
			}
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("javascript: %s", text)
	}
	return decode(answer, result)
}

// AddInitScript runs source in every document loaded from now on, before the
// document's own scripts.
func (d *Driver) AddInitScript(ctx context.Context, source string) error {
	return do(ctx, func() error { return d.page.AddInitScript(pw.Script{Content: pw.String(source)}) })
}

// Click moves the mouse to x, y, presses the left button and releases it.
func (d *Driver) Click(ctx context.Context, x, y float64) error {
	if err := d.slow(ctx); err != nil {
		return err
	}
	return do(ctx, func() error { return d.page.Mouse().Click(x, y) })
}

// InsertText types text into the focused element as one input.
func (d *Driver) InsertText(ctx context.Context, text string) error {
	if err := d.slow(ctx); err != nil {
		return err
	}
	return do(ctx, func() error { return d.page.Keyboard().InsertText(text) })
}

// Press presses and releases one key, with any modifiers named before it.
func (d *Driver) Press(ctx context.Context, key string) error {
	if err := d.slow(ctx); err != nil {
		return err
	}
	name, err := keyName(key)
	if err != nil {
		return err
	}
	return do(ctx, func() error { return d.page.Keyboard().Press(name) })
}

// SetViewport resizes the page's viewport. A device scale other than 1, or
// mobile emulation, is fixed by Firefox and WebKit when the browsing context
// is created, and asking either for it is an error; Chromium changes it on the
// live page.
func (d *Driver) SetViewport(ctx context.Context, v browser.Viewport) error {
	if v.Width <= 0 || v.Height <= 0 {
		return fmt.Errorf("playwright: a viewport of %dx%d has no area", v.Width, v.Height)
	}
	scale := v.Scale
	if scale == 0 {
		scale = 1
	}
	if scale == 1 && !v.Mobile {
		return do(ctx, func() error { return d.page.SetViewportSize(v.Width, v.Height) })
	}
	if d.engine != "chromium" {
		return fmt.Errorf("playwright: %s fixes the device scale and mobile emulation when the browsing context is created, so a viewport with scale %g and mobile %t cannot be set on a running page", d.engine, scale, v.Mobile)
	}
	return do(ctx, func() error {
		session, err := d.devtools()
		if err != nil {
			return err
		}
		_, err = session.Send("Emulation.setDeviceMetricsOverride", map[string]any{
			"width": v.Width, "height": v.Height, "deviceScaleFactor": scale, "mobile": v.Mobile,
		})
		return err
	})
}

// devtools opens the Chromium page's DevTools session once.
func (d *Driver) devtools() (pw.CDPSession, error) {
	d.cdpOnce.Do(func() { d.cdp, d.cdpErr = d.context.NewCDPSession(d.page) })
	return d.cdp, d.cdpErr
}

// EmulateMedia sets the media type and the preferences media queries read. An
// empty preference goes back to the browser's own.
func (d *Driver) EmulateMedia(ctx context.Context, m browser.Media) error {
	options := pw.PageEmulateMediaOptions{
		Media:         pw.MediaScreen,
		ColorScheme:   pw.ColorSchemeNoOverride,
		ReducedMotion: pw.ReducedMotionNoOverride,
	}
	switch m.Type {
	case "", "screen":
	case "print":
		options.Media = pw.MediaPrint
	default:
		return fmt.Errorf("playwright: media type %q is neither screen nor print", m.Type)
	}
	switch m.ColorScheme {
	case "":
	case "light":
		options.ColorScheme = pw.ColorSchemeLight
	case "dark":
		options.ColorScheme = pw.ColorSchemeDark
	default:
		return fmt.Errorf("playwright: prefers-color-scheme %q is neither light nor dark", m.ColorScheme)
	}
	switch m.ReducedMotion {
	case "":
	case "no-preference":
		options.ReducedMotion = pw.ReducedMotionNoPreference
	case "reduce":
		options.ReducedMotion = pw.ReducedMotionReduce
	default:
		return fmt.Errorf("playwright: prefers-reduced-motion %q is neither no-preference nor reduce", m.ReducedMotion)
	}
	return do(ctx, func() error { return d.page.EmulateMedia(options) })
}

// Screenshot captures the viewport as PNG.
func (d *Driver) Screenshot(ctx context.Context) ([]byte, error) {
	var shot []byte
	err := do(ctx, func() error {
		var err error
		shot, err = d.page.Screenshot(pw.PageScreenshotOptions{Type: pw.ScreenshotTypePng, Timeout: timeout(ctx)})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("playwright: screenshot: %s", message(err))
	}
	return shot, nil
}

// SetCookies stores cookies in this browsing context as if url had set them.
func (d *Driver) SetCookies(ctx context.Context, url string, cookies []*http.Cookie) error {
	list := make([]pw.OptionalCookie, 0, len(cookies))
	for _, c := range cookies {
		list = append(list, toPlaywright(url, c))
	}
	return do(ctx, func() error { return d.context.AddCookies(list) })
}

// Cookies returns the cookies this browsing context would send to url.
func (d *Driver) Cookies(ctx context.Context, url string) ([]*http.Cookie, error) {
	var got []pw.Cookie
	err := do(ctx, func() error {
		var err error
		got, err = d.context.Cookies(url)
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]*http.Cookie, 0, len(got))
	for _, c := range got {
		out = append(out, fromPlaywright(c))
	}
	return out, nil
}

// Close ends the browsing context and lets the shared browser go when no
// other driver uses it.
func (d *Driver) Close() error {
	d.closeOnce.Do(func() {
		d.events.listen(nil)
		if d.context != nil {
			if err := d.context.Close(); err != nil && !errors.Is(err, pw.ErrTargetClosed) {
				d.closeErr = fmt.Errorf("playwright: close the browsing context: %w", err)
			}
		}
		release(d.shared)
	})
	return d.closeErr
}

// slow waits the slow-motion delay, when there is one.
func (d *Driver) slow(ctx context.Context) error {
	if d.slowmo <= 0 {
		return nil
	}
	timer := time.NewTimer(d.slowmo)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
