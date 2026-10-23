package playwright

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arandu-io/hesape/arandutest/browser"
	pw "github.com/mxschmitt/playwright-go"
)

// events translates what Playwright reports about a page into browser.Event.
//
// Its handlers run on playwright-go's dispatching goroutine, which is the one
// that reads Playwright's answers, so they read only what the event already
// carries and never call back into Playwright: a call from there would wait
// for an answer the goroutine itself has to read.
type events struct {
	mu       sync.Mutex
	listener func(browser.Event)
	ids      map[pw.Request]string
	next     int
	// empty holds the main-frame navigation requests answered with no
	// document to load: a 204, a 205, an attachment.
	empty map[pw.Request]bool
	// loading is true from the main frame's navigation request until its
	// document loaded or the navigation ended without one.
	loading bool
}

func newEvents() *events {
	return &events{ids: map[pw.Request]string{}, empty: map[pw.Request]bool{}}
}

func (e *events) listen(fn func(browser.Event)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listener = fn
}

func (e *events) emit(ev browser.Event) {
	e.mu.Lock()
	fn := e.listener
	e.mu.Unlock()
	if fn != nil {
		ev.Time = time.Now()
		fn(ev)
	}
}

// id names a request for the life of the page. Playwright gives every hop of
// a redirect a request of its own, and so does this.
func (e *events) id(r pw.Request) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if id, ok := e.ids[r]; ok {
		return id
	}
	e.next++
	id := strconv.Itoa(e.next)
	e.ids[r] = id
	return id
}

func (e *events) forget(r pw.Request) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.ids, r)
	delete(e.empty, r)
}

func (e *events) attach(page pw.Page) {
	page.OnConsole(func(m pw.ConsoleMessage) { e.emit(consoleEvent(m)) })
	page.OnPageError(func(err error) {
		e.emit(browser.Event{Kind: browser.EventException, Text: message(err)})
	})
	page.OnRequest(func(r pw.Request) {
		if mainNavigation(page, r) {
			e.navigation(true)
		}
		e.emit(browser.Event{Kind: browser.EventRequest, RequestID: e.id(r), Method: r.Method(), URL: r.URL(), ResourceType: resourceType(r.ResourceType())})
	})
	page.OnResponse(func(resp pw.Response) {
		r := resp.Request()
		if mainNavigation(page, r) && !loadsDocument(resp) {
			e.mu.Lock()
			e.empty[r] = true
			e.mu.Unlock()
		}
		e.emit(browser.Event{Kind: browser.EventResponse, RequestID: e.id(r), Method: r.Method(), URL: resp.URL(),
			ResourceType: resourceType(r.ResourceType()), Status: resp.Status(), Header: header(resp.Headers())})
	})
	page.OnRequestFinished(func(r pw.Request) {
		e.emit(browser.Event{Kind: browser.EventRequestFinished, RequestID: e.id(r)})
		e.mu.Lock()
		empty := e.empty[r]
		e.mu.Unlock()
		e.forget(r)
		// A navigation answered with no document to load ends here, since no
		// load event follows it.
		if empty {
			e.navigation(false)
		}
	})
	page.OnRequestFailed(func(r pw.Request) {
		text := "failed"
		if err := r.Failure(); err != nil {
			text = networkError(err.Error())
		}
		e.emit(browser.Event{Kind: browser.EventRequestFailed, RequestID: e.id(r), Text: text, ResourceType: resourceType(r.ResourceType())})
		e.forget(r)
		if mainNavigation(page, r) {
			e.navigation(false)
		}
	})
	page.OnLoad(func(pw.Page) { e.navigation(false) })
}

// navigation reports the main frame starting or ending a load, once each.
func (e *events) navigation(started bool) {
	e.mu.Lock()
	changed := e.loading != started
	e.loading = started
	e.mu.Unlock()
	if !changed {
		return
	}
	kind := browser.EventNavigationFinished
	if started {
		kind = browser.EventNavigationStarted
	}
	e.emit(browser.Event{Kind: kind})
}

// mainNavigation reports whether r loads a document into the page's main
// frame.
func mainNavigation(page pw.Page, r pw.Request) bool {
	if !r.IsNavigationRequest() {
		return false
	}
	frame := r.Frame()
	return frame != nil && frame == page.MainFrame()
}

// loadsDocument reports whether the answer to a navigation request replaces
// the document, which is what makes a load event follow it.
func loadsDocument(resp pw.Response) bool {
	switch resp.Status() {
	case http.StatusNoContent, http.StatusResetContent:
		return false
	}
	return !strings.HasPrefix(strings.ToLower(resp.Headers()["content-disposition"]), "attachment")
}

// consoleEvent translates a console message. A message the page wrote with
// console.* carries its arguments; one the browser wrote about the page -- a
// resource that failed to load, a security notice -- carries none, and is
// told apart that way.
func consoleEvent(m pw.ConsoleMessage) browser.Event {
	ev := browser.Event{Kind: browser.EventConsole, Level: consoleLevel(m.Type()), Text: m.Text()}
	if loc := m.Location(); loc != nil {
		ev.URL = loc.URL
	}
	if len(m.Args()) > 0 {
		ev.Source = "console-api"
	} else {
		ev.Source = browserSource(ev.Text)
	}
	return ev
}

// browserSource names the part of the browser that wrote a message of its
// own, from its text: Playwright does not pass the source on.
func browserSource(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "content security policy") || strings.Contains(lower, "content-security-policy"):
		return "security"
	case strings.Contains(lower, "failed to load resource") || strings.Contains(lower, "net::err_"):
		return "network"
	}
	return "other"
}

// consoleLevel maps a console message type to the contract's levels.
func consoleLevel(kind string) string {
	switch kind {
	case "error", "assert":
		return "error"
	case "warning", "warn":
		return "warning"
	case "info":
		return "info"
	case "debug", "verbose":
		return "debug"
	}
	return "log"
}

// resourceTypes are Playwright's resource types in the contract's spelling.
var resourceTypes = map[string]string{
	"document":    "Document",
	"stylesheet":  "Stylesheet",
	"image":       "Image",
	"media":       "Media",
	"font":        "Font",
	"script":      "Script",
	"texttrack":   "TextTrack",
	"xhr":         "XHR",
	"fetch":       "Fetch",
	"eventsource": "EventSource",
	"websocket":   "WebSocket",
	"manifest":    "Manifest",
	"ping":        "Ping",
	"other":       "Other",
}

func resourceType(kind string) string {
	if name, ok := resourceTypes[kind]; ok {
		return name
	}
	if kind == "" {
		return "Other"
	}
	return strings.ToUpper(kind[:1]) + kind[1:]
}

func header(raw map[string]string) http.Header {
	h := http.Header{}
	for name, value := range raw {
		h.Set(name, value)
	}
	return h
}
