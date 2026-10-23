package playwright

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	pw "github.com/mxschmitt/playwright-go"
)

// do runs call and returns its error, or the context's when the context ends
// first. playwright-go takes no context, so a call the context abandoned keeps
// running until Playwright answers it; its answer is dropped.
func do(ctx context.Context, call func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- call() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// timeout is the time left before ctx's deadline in milliseconds, for the
// Playwright calls that take a timeout of their own; zero, which Playwright
// reads as none, when ctx has no deadline.
func timeout(ctx context.Context) *float64 {
	deadline, ok := ctx.Deadline()
	if !ok {
		return pw.Float(0)
	}
	return pw.Float(math.Max(1, float64(time.Until(deadline).Milliseconds())))
}

// message is the text of an error from playwright-go without the prefix every
// one of them carries.
func message(err error) string {
	var perr *pw.Error
	if errors.As(err, &perr) && perr.Message != "" {
		return perr.Message
	}
	return strings.TrimPrefix(err.Error(), "playwright: ")
}

// decode reads what the evaluation wrapper answered into result.
func decode(answer any, result any) error {
	m, ok := answer.(map[string]any)
	if !ok {
		return fmt.Errorf("javascript: the page answered %T, not the evaluation's envelope", answer)
	}
	kind, _ := m["kind"].(string)
	if result == nil {
		return nil
	}
	if kind != "json" {
		return fmt.Errorf("javascript: the expression evaluated to %s, which has no JSON value", kind)
	}
	text, _ := m["json"].(string)
	if err := json.Unmarshal([]byte(text), result); err != nil {
		return fmt.Errorf("javascript: decode the value: %w", err)
	}
	return nil
}

// modifierNames are the modifiers a key may be prefixed with, as the contract
// spells them and as Playwright does.
var modifierNames = map[string]string{
	"Shift": "Shift", "Control": "Control", "Ctrl": "Control", "Alt": "Alt", "Meta": "Meta",
}

// keyName turns a key as the contract spells it into Playwright's spelling.
// "Shift++" is the plus key with a modifier.
func keyName(key string) (string, error) {
	mods, last := "", key
	if strings.HasSuffix(key, "++") {
		mods, last = strings.TrimSuffix(key, "++"), "+"
	} else if i := strings.LastIndex(key, "+"); i >= 0 && key != "+" {
		mods, last = key[:i], key[i+1:]
	}
	if last == "" {
		return "", fmt.Errorf("press %q: no key", key)
	}
	parts := []string{}
	if mods != "" {
		for _, part := range strings.Split(mods, "+") {
			name, ok := modifierNames[part]
			if !ok {
				return "", fmt.Errorf("press %q: unknown modifier %q", key, part)
			}
			parts = append(parts, name)
		}
	}
	return strings.Join(append(parts, last), "+"), nil
}

// toPlaywright turns a cookie the test holds into one Playwright stores. A
// cookie that names its domain is stored for that domain and its path;
// otherwise for url, as the page at url would have stored it.
func toPlaywright(url string, c *http.Cookie) pw.OptionalCookie {
	out := pw.OptionalCookie{
		Name:     c.Name,
		Value:    c.Value,
		HttpOnly: pw.Bool(c.HttpOnly),
		Secure:   pw.Bool(c.Secure),
	}
	if c.Domain != "" {
		out.Domain = pw.String(c.Domain)
		path := c.Path
		if path == "" {
			path = "/"
		}
		out.Path = pw.String(path)
	} else {
		out.URL = pw.String(url)
		if c.Path != "" {
			// Playwright takes either a url or a domain and path; a path of
			// its own is kept by joining it to the url's origin.
			out.URL = pw.String(origin(url) + c.Path)
		}
	}
	if !c.Expires.IsZero() {
		out.Expires = pw.Float(float64(c.Expires.Unix()))
	}
	switch c.SameSite {
	case http.SameSiteLaxMode:
		out.SameSite = pw.SameSiteAttributeLax
	case http.SameSiteStrictMode:
		out.SameSite = pw.SameSiteAttributeStrict
	case http.SameSiteNoneMode:
		out.SameSite = pw.SameSiteAttributeNone
	}
	return out
}

// origin is the scheme and host of url, without path, query or fragment.
func origin(url string) string {
	scheme, rest, ok := strings.Cut(url, "://")
	if !ok {
		return url
	}
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}
	return scheme + "://" + rest
}

// fromPlaywright turns a stored cookie into the standard library's.
func fromPlaywright(c pw.Cookie) *http.Cookie {
	out := &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, HttpOnly: c.HttpOnly, Secure: c.Secure}
	if c.Expires > 0 {
		out.Expires = time.Unix(int64(c.Expires), 0)
	}
	if c.SameSite != nil {
		switch *c.SameSite {
		case *pw.SameSiteAttributeLax:
			out.SameSite = http.SameSiteLaxMode
		case *pw.SameSiteAttributeStrict:
			out.SameSite = http.SameSiteStrictMode
		case *pw.SameSiteAttributeNone:
			out.SameSite = http.SameSiteNoneMode
		}
	}
	return out
}

// netErrors maps what Firefox and WebKit say about a request the network
// refused to Chromium's net::ERR_ names, which is the vocabulary the kit and
// the tests written for it read. The browser's own words are kept beside the
// name.
var netErrors = []struct{ match, name string }{
	{"NS_ERROR_CONNECTION_REFUSED", "net::ERR_CONNECTION_REFUSED"},
	{"Could not connect to server", "net::ERR_CONNECTION_REFUSED"},
	{"Could not connect to", "net::ERR_CONNECTION_REFUSED"},
	{"Connection refused", "net::ERR_CONNECTION_REFUSED"},
	{"Failed to connect", "net::ERR_CONNECTION_REFUSED"},
	{"NS_ERROR_UNKNOWN_HOST", "net::ERR_NAME_NOT_RESOLVED"},
	{"hostname could not be found", "net::ERR_NAME_NOT_RESOLVED"},
	{"Could not resolve host", "net::ERR_NAME_NOT_RESOLVED"},
	{"NS_ERROR_NET_TIMEOUT", "net::ERR_TIMED_OUT"},
	{"The request timed out", "net::ERR_TIMED_OUT"},
	{"NS_ERROR_NET_RESET", "net::ERR_CONNECTION_RESET"},
	{"The network connection was lost", "net::ERR_CONNECTION_RESET"},
	{"NS_ERROR_NET_INTERRUPT", "net::ERR_CONNECTION_CLOSED"},
	{"NS_ERROR_OFFLINE", "net::ERR_INTERNET_DISCONNECTED"},
	{"NS_BINDING_ABORTED", "net::ERR_ABORTED"},
	{"NS_ERROR_ABORT", "net::ERR_ABORTED"},
	{"cancelled", "net::ERR_ABORTED"},
	{"SEC_ERROR_UNKNOWN_ISSUER", "net::ERR_CERT_AUTHORITY_INVALID"},
	{"SSL_ERROR", "net::ERR_SSL_PROTOCOL_ERROR"},
}

// networkError names a network failure with its net::ERR_ name, keeping what
// the browser said. A text Chromium wrote, or one the table does not know, is
// returned as it is.
func networkError(text string) string {
	if strings.Contains(text, "net::ERR_") {
		return text
	}
	for _, e := range netErrors {
		if strings.Contains(text, e.match) {
			return e.name + " (" + text + ")"
		}
	}
	return text
}

// badPorts are the ports the Fetch standard forbids a browser to connect to
// over http, https and ws. Every engine refuses them, and not alike: Chromium
// fails the navigation with net::ERR_UNSAFE_PORT, Firefox reports it as an
// interrupted navigation, and WebKit never answers it.
var badPorts = map[string]bool{}

func init() {
	for _, port := range strings.Fields(`1 7 9 11 13 15 17 19 20 21 22 23 25 37 42 43 53 69 77 79 87 95
		101 102 103 104 109 110 111 113 115 117 119 123 135 137 139 143 161 179 389 427 465
		512 513 514 515 526 530 531 532 540 548 554 556 563 587 601 636 989 990 993 995
		1719 1720 1723 2049 3659 4045 4190 5060 5061 6000 6566 6665 6666 6667 6668 6669
		6679 6697 10080`) {
		badPorts[port] = true
	}
}

// blockedPort reports whether url names a port the Fetch standard blocks, and
// which.
func blockedPort(url string) (string, bool) {
	u, err := neturl.Parse(url)
	if err != nil {
		return "", false
	}
	switch u.Scheme {
	case "http", "https", "ws", "wss":
	default:
		return "", false
	}
	port := u.Port()
	return port, port != "" && badPorts[port]
}
