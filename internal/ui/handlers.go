package ui

import (
	"bytes"
	"embed"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/ijackua/scriptorium/internal/library"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// routes wires the handler tree. Handlers are deliberately thin adapters over
// the domain: the spec does not test them, which makes keeping them thin a
// design obligation rather than a preference.
func routes(lib library.Library) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", noDirectoryListing(http.FileServerFS(staticFS)))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		render(w, "library.html", lib)
	})
	mux.HandleFunc("GET /series/{series}/books/{book}", func(w http.ResponseWriter, r *http.Request) {
		series, book, ok := lib.Book(r.PathValue("series"), r.PathValue("book"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		render(w, "book-detail.html", struct {
			Series library.Series
			Book   library.Book
		}{series, book})
	})
	return mux
}

// render executes a template into memory before writing anything, so that a
// template that fails halfway produces a 500 rather than a 200 carrying half a
// page with a Go error spliced into it.
func render(w http.ResponseWriter, name string, data any) {
	var page bytes.Buffer
	if err := templates.ExecuteTemplate(&page, name, data); err != nil {
		log.Printf("ui: render %s: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page.Bytes())
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
