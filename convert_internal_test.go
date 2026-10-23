package playwright

import (
	"net/http"
	"strings"
	"testing"
	"time"

	pw "github.com/mxschmitt/playwright-go"
)

func TestKeysAreSpelledAsPlaywrightSpellsThem(t *testing.T) {
	for in, want := range map[string]string{
		"Enter":         "Enter",
		"a":             "a",
		"Shift+Tab":     "Shift+Tab",
		"Ctrl+a":        "Control+a",
		"Control+Alt+x": "Control+Alt+x",
		"Shift++":       "Shift++",
		"ArrowDown":     "ArrowDown",
	} {
		got, err := keyName(in)
		if err != nil || got != want {
			t.Errorf("keyName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"Hyper+a", "Shift+"} {
		if _, err := keyName(bad); err == nil {
			t.Errorf("keyName(%q) accepted a key it cannot press", bad)
		}
	}
}

func TestNetworkFailuresCarryChromiumsNameAndTheBrowsersWords(t *testing.T) {
	for in, want := range map[string]string{
		"net::ERR_CONNECTION_REFUSED at http://127.0.0.1:9/": "net::ERR_CONNECTION_REFUSED at http://127.0.0.1:9/",
		"NS_ERROR_CONNECTION_REFUSED":                        "net::ERR_CONNECTION_REFUSED (NS_ERROR_CONNECTION_REFUSED)",
		"Could not connect to the server.":                   "net::ERR_CONNECTION_REFUSED (Could not connect to the server.)",
		"NS_ERROR_UNKNOWN_HOST":                              "net::ERR_NAME_NOT_RESOLVED (NS_ERROR_UNKNOWN_HOST)",
		"something else entirely":                            "something else entirely",
	} {
		if got := networkError(in); got != want {
			t.Errorf("networkError(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPortsTheFetchStandardBlocksAreRefusedBeforeNavigating(t *testing.T) {
	for url, want := range map[string]bool{
		"http://127.0.0.1:1/":     true,
		"https://example.test:25": true,
		"http://127.0.0.1:8080/":  false,
		"http://127.0.0.1/":       false,
		"about:blank":             false,
		"ftp://example.test:21/":  false,
	} {
		if _, got := blockedPort(url); got != want {
			t.Errorf("blockedPort(%q) = %t, want %t", url, got, want)
		}
	}
}

func TestEventsUseTheContractsVocabulary(t *testing.T) {
	for in, want := range map[string]string{"document": "Document", "xhr": "XHR", "fetch": "Fetch", "eventsource": "EventSource", "websocket": "WebSocket", "beacon": "Beacon", "": "Other"} {
		if got := resourceType(in); got != want {
			t.Errorf("resourceType(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"error": "error", "assert": "error", "warning": "warning", "info": "info", "verbose": "debug", "debug": "debug", "log": "log", "table": "log"} {
		if got := consoleLevel(in); got != want {
			t.Errorf("consoleLevel(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"Failed to load resource: the server responded with a status of 503 ()":                                "network",
		"Refused to execute inline script because it violates the following Content Security Policy directive": "security",
		"anything": "other",
	} {
		if got := browserSource(in); got != want {
			t.Errorf("browserSource(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCookiesRoundTripThroughPlaywrightsShape(t *testing.T) {
	expires := time.Unix(1893456000, 0)
	in := &http.Cookie{Name: "s", Value: "v", HttpOnly: true, Secure: true, Expires: expires, SameSite: http.SameSiteStrictMode}
	out := toPlaywright("http://127.0.0.1:8080/app?x=1", in)
	if out.URL == nil || *out.URL != "http://127.0.0.1:8080/app?x=1" || out.Domain != nil || out.Path != nil {
		t.Fatalf("a cookie without a domain is stored for the url: %+v", out)
	}
	if !*out.HttpOnly || !*out.Secure || *out.Expires != float64(expires.Unix()) || *out.SameSite != *pw.SameSiteAttributeStrict {
		t.Fatalf("attributes were lost: %+v", out)
	}

	scoped := toPlaywright("http://127.0.0.1:8080/app", &http.Cookie{Name: "s", Value: "v", Path: "/admin"})
	if scoped.URL == nil || *scoped.URL != "http://127.0.0.1:8080/admin" {
		t.Fatalf("a path of its own is joined to the url's origin: %+v", scoped)
	}
	domained := toPlaywright("http://127.0.0.1:8080/", &http.Cookie{Name: "s", Value: "v", Domain: "example.test"})
	if domained.URL != nil || *domained.Domain != "example.test" || *domained.Path != "/" {
		t.Fatalf("a cookie with a domain is stored for the domain and path: %+v", domained)
	}

	back := fromPlaywright(pw.Cookie{Name: "s", Value: "v", Domain: "127.0.0.1", Path: "/", Expires: -1, HttpOnly: true, SameSite: pw.SameSiteAttributeLax})
	if back.Name != "s" || !back.HttpOnly || !back.Expires.IsZero() || back.SameSite != http.SameSiteLaxMode {
		t.Fatalf("fromPlaywright = %+v", back)
	}
}

func TestTheEvaluationEnvelopeDecodesLikeAValueByJSON(t *testing.T) {
	var n int
	if err := decode(map[string]any{"kind": "json", "json": "42"}, &n); err != nil || n != 42 {
		t.Fatalf("decode = %d, %v", n, err)
	}
	var s *string
	if err := decode(map[string]any{"kind": "json", "json": "null"}, &s); err != nil || s != nil {
		t.Fatalf("null must decode into a nil pointer: %v, %v", s, err)
	}
	if err := decode(map[string]any{"kind": "undefined"}, &n); err == nil || !strings.Contains(err.Error(), "no JSON value") {
		t.Fatalf("undefined has no JSON value, got %v", err)
	}
	if err := decode(map[string]any{"kind": "undefined"}, nil); err != nil {
		t.Fatalf("a nil result accepts anything: %v", err)
	}
}

func TestOnlyTheThreeEnginesAreDriven(t *testing.T) {
	for in, want := range map[string]string{"": "chromium", "Chrome": "chromium", "firefox": "firefox", "safari": "webkit", " WebKit ": "webkit"} {
		if got, err := engineName(in); err != nil || got != want {
			t.Errorf("engineName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := engineName("opera"); err == nil {
		t.Error("an engine Playwright does not drive was accepted")
	}
}
