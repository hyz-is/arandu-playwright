//go:build e2e

package playwright

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/framework/test/browser"
)

// engine is the engine ARANDU_E2E_BROWSER names, as the conformance run
// reads it.
func engine(t *testing.T) string {
	t.Helper()
	opts, err := browser.OptionsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return opts.Browser
}

// idle waits until no browser and no Playwright process is left, or fails.
func idle(t *testing.T, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		pool.mu.Lock()
		gone := pool.pw == nil && len(pool.browsers) == 0
		pool.mu.Unlock()
		if gone {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Playwright or a browser is still running %s after the last driver closed", within)
		}
		<-tick.C
	}
}

// TestDriversShareABrowserAndLeaveNothingBehind proves two drivers share one
// browser process with separate browsing contexts, and that the browser and
// Playwright stop once the last driver closed and the linger passed.
func TestDriversShareABrowserAndLeaveNothingBehind(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	opts := browser.Options{Browser: engine(t)}

	first, err := Launch(ctx, opts)
	if errors.Is(err, browser.ErrNoBrowser) && os.Getenv("CI") == "" {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	second, err := Launch(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	a, b := first.(*Driver), second.(*Driver)
	if a.shared != b.shared || a.context == b.context {
		t.Fatal("two drivers of one engine must share the browser and not the browsing context")
	}
	if info := a.Info(); info.Driver != Name || info.Browser != opts.Browser || info.Version == "" {
		t.Fatalf("info = %+v", info)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Evaluate(ctx, "1 + 1", nil); err != nil {
		t.Fatalf("closing one driver closed the other's page: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("a second Close: %v", err)
	}
	if err := b.Evaluate(ctx, "1", nil); err == nil {
		t.Fatal("a closed driver still answered")
	}
	idle(t, linger+10*time.Second)
}

// TestAMissingBrowserSkipsRatherThanDownloads proves Launch with no browser
// installed answers browser.ErrNoBrowser, naming the install command.
func TestAMissingBrowserSkipsRatherThanDownloads(t *testing.T) {
	idle(t, linger+10*time.Second)
	if _, err := DriverDir(); err != nil {
		t.Skip(err)
	}
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", t.TempDir())
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	d, err := Launch(ctx, browser.Options{Browser: engine(t)})
	if err == nil {
		_ = d.Close()
		t.Fatal("Launch found a browser in an empty directory")
	}
	if strings.Contains(err.Error(), "Playwright driver "+driverVersion+" is not installed") {
		t.Skip(err)
	}
	if !errors.Is(err, browser.ErrNoBrowser) || !strings.Contains(err.Error(), "arandu-playwright/install-browsers") {
		t.Fatalf("a missing browser must wrap browser.ErrNoBrowser and name the install command, got %v", err)
	}
	idle(t, 10*time.Second)
}
