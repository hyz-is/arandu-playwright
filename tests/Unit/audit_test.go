package unit_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// What this package proves about itself, before anybody installs it.
//
// `aru doctor` audits the application it runs in and never opens an installed
// package, so the comparison between what arandu.mod.toml declares and what
// the code does happens here or nowhere. It reads this package's own Go files
// as syntax.
//
// This package does most of what it declares through playwright-go, which
// starts a Node process, downloads the driver and reads and writes the user's
// cache. A call into one of those entry points is counted as the capability
// it exercises, by the table in delegated. What the audit does not reach: a
// call hidden behind an interface or reached by reflection. A green run means
// nothing undeclared was found written down, not that nothing exists.

// capabilities is the [permissions] block under its four names.
type capabilities struct {
	network, filesystem, exec, migrations bool
}

// delegated are the playwright-go entry points this package calls, and what
// each does on its behalf.
var delegated = map[string]capabilities{
	// Run starts the Playwright driver, a Node process, after checking its
	// files in the user's cache.
	"pw.Run": {exec: true, filesystem: true},
	// DownloadDriver fetches the Playwright package and a Node runtime from
	// the npm registry and nodejs.org and writes them into the cache.
	"DownloadDriver": {network: true, filesystem: true},
}

func TestTheDeclaredCapabilitiesAreWhatTheCodeDoes(t *testing.T) {
	declared := declaredCapabilities(t)
	used := usedCapabilities(t)
	for _, c := range []struct {
		name           string
		declared, used bool
	}{
		{"network", declared.network, used.network},
		{"filesystem", declared.filesystem, used.filesystem},
		{"exec", declared.exec, used.exec},
		{"migrations", declared.migrations, used.migrations},
	} {
		switch {
		case c.used && !c.declared:
			t.Errorf("arandu.mod.toml declares %s = false and the code uses it: declare it, or remove what needs it", c.name)
		case c.declared && !c.used:
			t.Errorf("arandu.mod.toml declares %s = true and nothing uses it: set it to false", c.name)
		}
	}
}

// TestTheCodeHasNoRouteTableOrMigration holds what the manifest's
// migrations = false and the README promise: the driver is test tooling that
// imports the framework's test kit and nothing an application runs.
func TestTheCodeHasNoRouteTableOrMigration(t *testing.T) {
	for _, f := range linkedFiles(t) {
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			kit := path == "github.com/arandu-io/framework/test" || strings.HasPrefix(path, "github.com/arandu-io/framework/test/")
			if (strings.HasPrefix(path, "github.com/arandu-io/framework") && !kit) || strings.HasPrefix(path, "github.com/arandu-io/hesape/") {
				t.Errorf("%s imports %s; the driver reaches the test kit and nothing of the application's runtime", f.Name.Name, path)
			}
		}
	}
}

func usedCapabilities(t *testing.T) capabilities {
	t.Helper()
	var used capabilities
	for _, f := range linkedFiles(t) {
		for _, imp := range f.Imports {
			switch strings.Trim(imp.Path.Value, `"`) {
			case "os/exec":
				used.exec = true
			case "net/smtp", "net/rpc":
				used.network = true
			case "io/ioutil":
				used.filesystem = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			// A delegated entry point counts whether it is called or handed
			// on as a method value.
			if sel, ok := n.(*ast.SelectorExpr); ok {
				name := callName(sel)
				for entry, c := range delegated {
					if name == entry || strings.HasSuffix(name, "."+entry) {
						used.network = used.network || c.network
						used.filesystem = used.filesystem || c.filesystem
						used.exec = used.exec || c.exec
					}
				}
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := callName(call.Fun)
			switch name {
			case "http.Get", "http.Post", "http.Head", "http.PostForm", "http.NewRequest", "http.NewRequestWithContext", "net.Dial", "net.DialTimeout":
				used.network = true
			case "os.Open", "os.OpenFile", "os.Create", "os.ReadFile", "os.WriteFile", "os.Remove", "os.RemoveAll",
				"os.Mkdir", "os.MkdirAll", "os.Rename", "os.ReadDir", "filepath.Walk", "filepath.WalkDir":
				used.filesystem = true
			case "exec.Command", "exec.CommandContext", "syscall.Exec":
				used.exec = true
			}
			if name == "Migrations" || strings.HasSuffix(name, ".Migrations") {
				used.migrations = true
			}
			return true
		})
	}
	return used
}

// callName spells a call's function as written: "os.ReadDir", "driver.DownloadDriver".
func callName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if x := callName(f.X); x != "" {
			return x + "." + f.Sel.Name
		}
		return f.Sel.Name
	}
	return ""
}

func declaredCapabilities(t *testing.T) capabilities {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root(t), "arandu.mod.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var c capabilities
	seen := map[string]bool{}
	inside := false
	for _, line := range strings.Split(string(body), "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inside = line == "[permissions]"
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !inside || !ok {
			continue
		}
		key, on := strings.TrimSpace(key), strings.TrimSpace(value) == "true"
		seen[key] = true
		switch key {
		case "network":
			c.network = on
		case "filesystem":
			c.filesystem = on
		case "exec":
			c.exec = on
		case "migrations":
			c.migrations = on
		}
	}
	for _, key := range []string{"network", "filesystem", "exec", "migrations"} {
		if !seen[key] {
			t.Errorf("arandu.mod.toml does not declare %s under [permissions]", key)
		}
	}
	return c
}

// linkedFiles are the Go files an application that installs this package
// compiles in: the package at the root, without its tests. A command is run
// by hand and never linked.
func linkedFiles(t *testing.T) []*ast.File {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root(t), "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var out []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		if f.Name.Name == "main" {
			continue
		}
		f, err = parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		t.Fatal("no Go file was found at the package root, so every rule would pass by reading nothing")
	}
	return out
}

func root(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}
