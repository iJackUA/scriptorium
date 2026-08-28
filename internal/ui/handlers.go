package ui

import (
	"bytes"
	"embed"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/workspace"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// routes wires the handler tree. Handlers are deliberately thin adapters over
// the domain: the spec does not test them, which makes keeping them thin a
// design obligation rather than a preference.
func routes(lib library.Library, session *workspace.Session) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", noDirectoryListing(http.FileServerFS(staticFS)))

	// The whole application is behind having a workspace, so what the root
	// serves is whichever of the two screens the session says applies.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		name, data := screen(lib, session)
		page(w, name, data)
	})

	// Choosing a folder is a POST because it is the moment the workspace is
	// created and remembered. htmx swaps the screen it returns into the page
	// the button was on, so the same two screens serve both entry points.
	mux.HandleFunc("POST /workspace", func(w http.ResponseWriter, r *http.Request) {
		// Whatever happened is already on the session, and the screen about to
		// be rendered is the one that reports it.
		session.Choose()
		name, data := screen(lib, session)
		reply(w, r, name, data)
	})

	mux.HandleFunc("GET /series/{series}/books/{book}", func(w http.ResponseWriter, r *http.Request) {
		series, book, ok := lib.Book(r.PathValue("series"), r.PathValue("book"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		fragment(w, "book-detail", struct {
			Series library.Series
			Book   library.Book
		}{series, book})
	})
	return mux
}

// welcome is what a user with no workspace sees, and carries whatever the
// session has to say about why they are seeing it.
type welcome struct{ Problem string }

// screen picks the screen the session's state calls for.
func screen(lib library.Library, session *workspace.Session) (string, any) {
	if _, ok := session.Current(); ok {
		return "library", lib
	}
	return "welcome", welcome{Problem: session.Problem()}
}

// Every screen is written once, as a fragment. Whether it arrives wrapped in
// the layout is the caller's question, and there are only two answers.

// fragment writes a screen on its own, for htmx to swap into a page that is
// already on the user's screen.
func fragment(w http.ResponseWriter, name string, data any) {
	body, err := execute(name, data)
	if err != nil {
		fail(w, name, err)
		return
	}
	write(w, body)
}

// page writes a screen wrapped in the layout, for a navigation that has
// nothing on screen to swap into.
func page(w http.ResponseWriter, name string, data any) {
	body, err := execute(name, data)
	if err != nil {
		fail(w, name, err)
		return
	}
	wrapped, err := execute("layout", template.HTML(body))
	if err != nil {
		fail(w, "layout", err)
		return
	}
	write(w, wrapped)
}

// reply writes a screen the way the request asked for it. htmx wants the
// fragment; anything else — a form posted with scripting off, or curl — wants
// a page it can read.
func reply(w http.ResponseWriter, r *http.Request, name string, data any) {
	if r.Header.Get("HX-Request") == "true" {
		fragment(w, name, data)
		return
	}
	page(w, name, data)
}

// execute renders a template into memory rather than onto the wire, so that a
// template failing halfway produces a 500 rather than a 200 carrying half a
// page with a Go error spliced into it.
func execute(name string, data any) (string, error) {
	var out bytes.Buffer
	if err := templates.ExecuteTemplate(&out, name, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

func write(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, body)
}

func fail(w http.ResponseWriter, name string, err error) {
	log.Printf("ui: render %s: %v", name, err)
	http.Error(w, "template error", http.StatusInternalServerError)
}

// noDirectoryListing hides the index http.FileServer generates for a directory
// with no index.html, so the embedded assets are not enumerable.
func noDirectoryListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
