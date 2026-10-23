package unit_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/arandu-io/framework/test/browser"

	playwright "github.com/hyz-is/arandu-playwright"
)

func TestTheDriverAndBrowsersLiveInTheUserCache(t *testing.T) {
	t.Setenv("PLAYWRIGHT_DRIVER_PATH", "")
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", "")
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Skip(err)
	}
	home, _ := os.UserHomeDir()
	wd, _ := os.Getwd()
	for name, dir := range map[string]func() (string, error){"DriverDir": playwright.DriverDir, "BrowsersDir": playwright.BrowsersDir} {
		got, err := dir()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.HasPrefix(got, cache) && !strings.HasPrefix(got, filepath.Join(home, ".cache")) {
			t.Errorf("%s = %s, outside the user cache %s", name, got, cache)
		}
		if strings.HasPrefix(got, filepath.Dir(filepath.Dir(wd))) {
			t.Errorf("%s = %s, inside the project", name, got)
		}
	}
}

func TestTheEnvironmentMovesTheDirectories(t *testing.T) {
	t.Setenv("PLAYWRIGHT_DRIVER_PATH", "/opt/pw-driver")
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", "/opt/pw-browsers")
	if got, _ := playwright.DriverDir(); got != "/opt/pw-driver" {
		t.Errorf("DriverDir = %s", got)
	}
	if got, _ := playwright.BrowsersDir(); got != "/opt/pw-browsers" {
		t.Errorf("BrowsersDir = %s", got)
	}
}

func TestInstalledListsTheBrowserBuildsOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", dir)
	for _, name := range []string{"webkit-2336", "chromium-1234", "ffmpeg-1011", "firefox-1538"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := playwright.Installed()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"chromium-1234", "firefox-1538", "webkit-2336"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Installed = %v, want %v", got, want)
	}

	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", filepath.Join(dir, "missing"))
	if got, err := playwright.Installed(); err != nil || len(got) != 0 {
		t.Fatalf("a missing directory is an empty list: %v, %v", got, err)
	}
}

// TestAMissingDriverSkipsRatherThanDownloads proves Launch never fetches
// anything: with no driver where it looks, it answers browser.ErrNoBrowser,
// which skips a test locally and fails it in CI, and names the command that
// installs it.
func TestAMissingDriverSkipsRatherThanDownloads(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLAYWRIGHT_DRIVER_PATH", dir)
	_, err := playwright.Launch(t.Context(), browser.Options{Browser: "firefox"})
	if !errors.Is(err, browser.ErrNoBrowser) {
		t.Fatalf("a missing driver must wrap browser.ErrNoBrowser, got %v", err)
	}
	if !strings.Contains(err.Error(), "arandu-playwright/install-browsers") {
		t.Fatalf("the error must name the install command: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("Launch wrote into the driver directory: %v", entries)
	}
}

func TestAnEngineItCannotDriveIsRefused(t *testing.T) {
	_, err := playwright.Launch(t.Context(), browser.Options{Browser: "opera"})
	if err == nil || errors.Is(err, browser.ErrNoBrowser) || !strings.Contains(err.Error(), "opera") {
		t.Fatalf("an unknown engine is a configuration error, not a missing browser: %v", err)
	}
}
