package playwright

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	pw "github.com/mxschmitt/playwright-go"
)

// modulePath is this module's import path, for the commands the errors name.
const modulePath = "github.com/hyz-is/arandu-playwright"

// driverVersion is the Playwright release the bindings drive, and so the one
// whose browsers have to be installed.
const driverVersion = "1.62.1"

// Engines are the browsers this driver drives, in the order Install installs
// them when none is named.
var Engines = []string{"chromium", "firefox", "webkit"}

// InstallOptions says what Install puts in place.
type InstallOptions struct {
	// Browsers are the engines to install: "chromium", "firefox", "webkit".
	// Empty means all three.
	Browsers []string
	// WithDeps also installs the operating system packages the browsers need,
	// which on Linux runs the system's package manager and asks for root.
	WithDeps bool
	// Stdout and Stderr receive the installer's progress; nil discards it.
	Stdout, Stderr io.Writer
}

// Install downloads the Playwright driver -- a Node runtime and the Playwright
// package -- and the browsers into the user's cache directory, and nothing
// into the project. It does nothing for what is already there, so CI can run
// it on every build behind a cache of BrowsersDir and DriverDir.
//
// The download hosts are the npm registry and nodejs.org for the driver, and
// Playwright's CDN for the browsers. PLAYWRIGHT_DRIVER_PATH,
// PLAYWRIGHT_BROWSERS_PATH, PLAYWRIGHT_GO_NPM_REGISTRY, NODE_MIRROR and
// PLAYWRIGHT_DOWNLOAD_HOST move them, as Playwright documents.
func Install(ctx context.Context, opts InstallOptions) error {
	asked := opts.Browsers
	if len(asked) == 0 {
		asked = Engines
	}
	browsers := make([]string, 0, len(asked))
	for _, name := range asked {
		engine, err := engineName(name)
		if err != nil {
			return err
		}
		browsers = append(browsers, engine)
	}
	stdout, stderr := opts.Stdout, opts.Stderr
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	driver, err := pw.NewDriver(&pw.RunOptions{Verbose: false, Stdout: stdout, Stderr: stderr, SkipInstallBrowsers: true})
	if err != nil {
		return fmt.Errorf("playwright: install: %w", err)
	}
	if err := do(ctx, driver.DownloadDriver); err != nil {
		return fmt.Errorf("playwright: install the driver %s: %w", driverVersion, err)
	}

	args := append([]string{"install"}, browsers...)
	if opts.WithDeps {
		args = append(args, "--with-deps")
	}
	// The driver's own command, bound to ctx so an install that is cancelled
	// stops downloading.
	cli := driver.Command(args...)
	cmd := exec.CommandContext(ctx, cli.Path, cli.Args[1:]...)
	cmd.Env = cli.Env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("playwright: install %s: %w", strings.Join(browsers, ", "), err)
	}
	return nil
}

// DriverDir is where the Playwright driver lives: PLAYWRIGHT_DRIVER_PATH, or
// ms-playwright-go/<version> under the user's cache directory.
func DriverDir() (string, error) {
	if dir := os.Getenv("PLAYWRIGHT_DRIVER_PATH"); dir != "" {
		return dir, nil
	}
	cache, err := cacheDir(false)
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "ms-playwright-go", driverVersion), nil
}

// BrowsersDir is where Playwright keeps its browsers: PLAYWRIGHT_BROWSERS_PATH,
// or ms-playwright under the user's cache directory.
func BrowsersDir() (string, error) {
	if dir := os.Getenv("PLAYWRIGHT_BROWSERS_PATH"); dir != "" && dir != "0" {
		return dir, nil
	}
	cache, err := cacheDir(true)
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "ms-playwright"), nil
}

// Installed lists the browser builds present in BrowsersDir, such as
// "chromium-1234" or "webkit-2336", sorted. A directory that does not exist
// is an empty list.
func Installed() ([]string, error) {
	dir, err := BrowsersDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("playwright: read %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		for _, engine := range Engines {
			if strings.HasPrefix(e.Name(), engine+"-") {
				out = append(out, e.Name())
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// cacheDir is the user's cache directory. Playwright honours XDG_CACHE_HOME on
// Linux for its browsers, and the bindings do not for the driver, so xdg says
// which of the two is asking.
func cacheDir(xdg bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("playwright: the user's home directory: %w", err)
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Caches"), nil
	case "windows":
		return filepath.Join(home, "AppData", "Local"), nil
	}
	if dir := os.Getenv("XDG_CACHE_HOME"); xdg && dir != "" {
		return dir, nil
	}
	return filepath.Join(home, ".cache"), nil
}
