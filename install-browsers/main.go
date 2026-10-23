// Command install-browsers puts the Playwright driver and its browsers in the
// user's cache directory, for the playwright browser driver to run:
//
//	go run github.com/hyz-is/arandu-playwright/install-browsers [--with-deps] [chromium] [firefox] [webkit]
//
// With no browser named it installs all three. --with-deps also installs the
// operating system packages the browsers need, which on Linux asks for root.
// What is already installed is left as it is, so it is safe to run on every
// CI build behind a cache of the two directories it prints.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	playwright "github.com/hyz-is/arandu-playwright"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "install-browsers:", err)
		os.Exit(1)
	}
}

func run() error {
	withDeps := flag.Bool("with-deps", false, "also install the operating system packages the browsers need")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: install-browsers [--with-deps] [chromium] [firefox] [webkit]")
		flag.PrintDefaults()
	}
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := playwright.Install(ctx, playwright.InstallOptions{
		Browsers: flag.Args(),
		WithDeps: *withDeps,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
	})
	if err != nil {
		return err
	}

	driver, err := playwright.DriverDir()
	if err != nil {
		return err
	}
	browsers, err := playwright.BrowsersDir()
	if err != nil {
		return err
	}
	installed, err := playwright.Installed()
	if err != nil {
		return err
	}
	fmt.Printf("driver:   %s\nbrowsers: %s (%s)\n", driver, browsers, strings.Join(installed, ", "))
	return nil
}
