package playwright

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/arandu-io/framework/test/browser"
	pw "github.com/mxschmitt/playwright-go"
)

// linger is how long a browser no driver uses stays open for the next one.
// Tests launch and close a driver each, one after another, and a browser that
// closed between them would be started again for every test.
const linger = 3 * time.Second

// sharedBrowser is one browser process, shared by the drivers of one engine
// and one window mode.
type sharedBrowser struct {
	key     string
	browser pw.Browser
	users   int
	idle    *time.Timer
}

// pool is the Playwright process and the browsers it started. There is one
// per test binary, because a Playwright process costs a Node start and a
// browser a second or more.
var pool struct {
	mu       sync.Mutex
	pw       *pw.Playwright
	browsers map[string]*sharedBrowser
}

// acquire returns the browser for engine and headed, starting Playwright and
// the browser when neither is running.
func acquire(ctx context.Context, engine string, headed bool) (*sharedBrowser, error) {
	key := fmt.Sprintf("%s/headed=%t", engine, headed)

	pool.mu.Lock()
	defer pool.mu.Unlock()

	if s, ok := pool.browsers[key]; ok && s.browser.IsConnected() {
		if s.idle != nil {
			s.idle.Stop()
			s.idle = nil
		}
		s.users++
		return s, nil
	}

	if pool.pw == nil {
		var started *pw.Playwright
		err := do(ctx, func() error {
			var err error
			started, err = pw.Run(&pw.RunOptions{Verbose: false, Stdout: io.Discard})
			return err
		})
		if err != nil && strings.Contains(message(err), "please install the driver") {
			return nil, fmt.Errorf("%w: the Playwright driver %s is not installed in %s; install it and the browsers with `go run %s/install-browsers`",
				browser.ErrNoBrowser, driverVersion, driverDirOrUnknown(), modulePath)
		}
		if err != nil {
			return nil, fmt.Errorf("playwright: the Playwright driver did not start: %s", firstLine(message(err)))
		}
		pool.pw = started
		pool.browsers = map[string]*sharedBrowser{}
	}

	var launched pw.Browser
	err := do(ctx, func() error {
		var err error
		launched, err = engineType(pool.pw, engine).Launch(pw.BrowserTypeLaunchOptions{Headless: pw.Bool(!headed)})
		return err
	})
	if err != nil {
		text := message(err)
		if strings.Contains(text, "Executable doesn't exist") || strings.Contains(text, "playwright install") {
			err = fmt.Errorf("%w: %s is not installed for Playwright %s; install it with `go run %s/install-browsers %s`",
				browser.ErrNoBrowser, engine, driverVersion, modulePath, engine)
		} else {
			err = fmt.Errorf("playwright: launch %s: %s", engine, text)
		}
		stopIfUnused()
		return nil, err
	}
	s := &sharedBrowser{key: key, browser: launched, users: 1}
	pool.browsers[key] = s
	return s, nil
}

// release gives a driver's share of its browser back, and closes the browser
// once nobody took it again within linger.
func release(s *sharedBrowser) {
	if s == nil {
		return
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	s.users--
	if s.users > 0 {
		return
	}
	var timer *time.Timer
	timer = time.AfterFunc(linger, func() {
		pool.mu.Lock()
		defer pool.mu.Unlock()
		if s.users > 0 || s.idle != timer {
			return
		}
		s.idle = nil
		if pool.browsers[s.key] == s {
			delete(pool.browsers, s.key)
		}
		// The process is gone either way, and nothing would read the reason.
		_ = s.browser.Close()
		stopIfUnused()
	})
	s.idle = timer
}

// stopIfUnused stops Playwright when no browser is left. The caller holds
// pool.mu.
func stopIfUnused() {
	if pool.pw == nil || len(pool.browsers) > 0 {
		return
	}
	_ = pool.pw.Stop()
	pool.pw = nil
	pool.browsers = nil
}

func engineType(p *pw.Playwright, engine string) pw.BrowserType {
	switch engine {
	case "firefox":
		return p.Firefox
	case "webkit":
		return p.WebKit
	}
	return p.Chromium
}

// driverDirOrUnknown names the driver directory for an error message.
func driverDirOrUnknown() string {
	if dir, err := DriverDir(); err == nil {
		return dir
	}
	return "the user's cache"
}
