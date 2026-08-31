package ui

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/format"
	"github.com/ijackua/scriptorium/internal/format/fb2"
	"github.com/ijackua/scriptorium/internal/format/txt"
	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/translation"
	"github.com/ijackua/scriptorium/internal/workspace"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"languageName": func(tag string) string {
		if language, ok := workspace.LanguageFor(tag); ok {
			return language.Name
		}
		return tag
	},
	"hasTag": func(tags []string, tag string) bool {
		for _, candidate := range tags {
			if candidate == tag {
				return true
			}
		}
		return false
	},
}).ParseFS(templateFS, "templates/*.html"))

// routes wires the handler tree. Handlers are deliberately thin adapters over
// the domain: the spec does not test them, which makes keeping them thin a
// design obligation rather than a preference.
//
// They hang off a screens value rather than closing over the session, so that
// "which workspace is open" is answered in one place instead of threaded
// through every handler as an argument.
func routes(session *workspace.Session, agents func(string, agent.Logger) (agent.Agent, error)) http.Handler {
	s := screens{session: session, agents: agents, dictionaryRuns: newDictionaryRuns()}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", noDirectoryListing(http.FileServerFS(staticFS)))

	// The whole application is behind having a workspace, so what the root
	// serves is whichever of the two screens the session says applies.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		name, data := s.screen(form{})
		page(w, name, data)
	})

	// Choosing a folder is a POST because it is the moment the workspace is
	// created and remembered. htmx swaps the screen it returns into the page
	// the button was on, so the same two screens serve both entry points.
	mux.HandleFunc("POST /workspace", func(w http.ResponseWriter, r *http.Request) {
		// Whatever happened is already on the session, and the screen about to
		// be rendered is the one that reports it.
		s.session.Choose()
		s.reply(w, r, form{})
	})

	mux.HandleFunc("POST /series", s.createSeries)
	mux.HandleFunc("POST /books", s.addBook)
	mux.HandleFunc("POST /settings/target-languages", s.setTargetLanguages)
	mux.HandleFunc("GET /series/{series}/books/{book}", s.bookDetail)
	mux.HandleFunc("POST /series/{series}/books/{book}/source", s.uploadSourceFile)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets", s.createTarget)
	mux.HandleFunc("DELETE /series/{series}/books/{book}/targets/{target}", s.deleteTarget)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets/{target}/dictionary", s.startDictionaryBuilding)
	mux.HandleFunc("GET /series/{series}/books/{book}/targets/{target}/dictionary-progress", s.dictionaryProgress)
	return mux
}

// screens renders the interface over whichever workspace the session has open.
type screens struct {
	session        *workspace.Session
	agents         func(string, agent.Logger) (agent.Agent, error)
	dictionaryRuns *dictionaryRuns
}

type dictionaryRuns struct {
	mu       sync.Mutex
	progress map[string]translation.DictionaryProgress
}

func newDictionaryRuns() *dictionaryRuns {
	return &dictionaryRuns{progress: make(map[string]translation.DictionaryProgress)}
}

func (r *dictionaryRuns) set(key string, progress translation.DictionaryProgress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress[key] = progress
}

func (r *dictionaryRuns) get(key string) translation.DictionaryProgress {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.progress[key]
}

func (r *dictionaryRuns) clear(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.progress, key)
}

func (s screens) createSeries(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	typed := form{Panel: seriesPanel, Name: r.FormValue("name"), Language: r.FormValue("language")}
	if _, err := store.CreateSeries(typed.Name, typed.Language); err != nil {
		// The screen comes back with what the user typed still in it, so a
		// rejection is a correction rather than a retyping.
		typed.Problem = err.Error()
		s.reply(w, r, typed)
		return
	}
	s.reply(w, r, form{})
}

// addBook serves both kinds of Book, because on disk there is only one kind: a
// Book with no Series named gets a Series of one, and the user is never asked
// about it.
func (s screens) addBook(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	typed := form{
		Panel:    bookPanel,
		Series:   r.FormValue("series"),
		Code:     r.FormValue("code"),
		Title:    r.FormValue("title"),
		Author:   r.FormValue("author"),
		Language: r.FormValue("language"),
	}
	draft := library.BookDraft{Code: typed.Code, Title: typed.Title, Author: typed.Author}

	var err error
	if typed.Series == "" {
		_, _, err = store.AddStandaloneBook(draft, typed.Language)
	} else {
		_, err = store.AddBook(typed.Series, draft)
	}
	if err != nil {
		typed.Problem = err.Error()
		s.reply(w, r, typed)
		return
	}
	s.reply(w, r, form{})
}

func (s screens) bookDetail(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	lib, err := store.Library()
	if err != nil {
		s.reply(w, r, form{})
		return
	}
	series, book, ok := lib.Book(r.PathValue("series"), r.PathValue("book"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	fragment(w, "book-detail", s.bookDetailData(series, book, ""))
}

func (s screens) uploadSourceFile(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	file, header, err := r.FormFile("source")
	if err != nil {
		s.detail(w, r, "choose a .txt or .fb2 Source File to upload")
		return
	}
	defer file.Close()
	confirmed := r.FormValue("confirmed") == "true"
	if err := store.UploadSourceFile(r.PathValue("series"), r.PathValue("book"), header.Filename, file, confirmed); err != nil {
		s.detail(w, r, err.Error())
		return
	}
	s.bookDetail(w, r)
}

type targetOption struct {
	workspace.Language
	Status library.Status
}

func (s screens) setTargetLanguages(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.reply(w, r, form{Panel: "settings", Problem: err.Error()})
		return
	}
	if err := s.session.SetTargetLanguages(r.Form["languages"]); err != nil {
		s.reply(w, r, form{Panel: "settings", Problem: err.Error()})
		return
	}
	s.reply(w, r, form{})
}

func (s screens) createTarget(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	seriesCode, bookCode, target := r.PathValue("series"), r.PathValue("book"), r.FormValue("language")
	ws, _ := s.session.Current()
	allowed := false
	for _, tag := range ws.Config.Languages {
		if tag == target {
			allowed = true
			break
		}
	}
	if !allowed {
		s.detail(w, r, "that Target Language is not enabled in Workspace settings")
		return
	}
	if _, err := store.CreateTranslationTarget(seriesCode, bookCode, target, ws.Config.Languages); err != nil {
		s.detail(w, r, err.Error())
		return
	}
	s.bookDetail(w, r)
}

func (s screens) deleteTarget(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	if err := store.DeleteTranslationTarget(r.PathValue("series"), r.PathValue("book"), r.PathValue("target")); err != nil {
		s.detail(w, r, err.Error())
		return
	}
	s.bookDetail(w, r)
}

func (s screens) startDictionaryBuilding(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	seriesCode, bookCode, targetLanguage := r.PathValue("series"), r.PathValue("book"), r.PathValue("target")
	lib, err := store.Library()
	if err != nil {
		s.detail(w, r, err.Error())
		return
	}
	series, book, found := lib.Book(seriesCode, bookCode)
	if !found {
		http.NotFound(w, r)
		return
	}
	var target library.TranslationTarget
	for _, candidate := range book.Targets {
		if candidate.Language == targetLanguage {
			target = candidate
			break
		}
	}
	if target.Language == "" || target.Status != library.StatusNew {
		s.detail(w, r, "Dictionary Building can only start for a new Translation Target")
		return
	}
	source, filename, err := store.SourceFile(seriesCode, bookCode)
	if err != nil {
		s.detail(w, r, err.Error())
		return
	}
	document, err := parseSourceFile(filename, source)
	if err != nil {
		s.detail(w, r, err.Error())
		return
	}
	if err := store.SetTranslationTargetStatus(seriesCode, bookCode, targetLanguage, library.StatusAnalyzing); err != nil {
		s.detail(w, r, err.Error())
		return
	}
	key := dictionaryKey(seriesCode, bookCode, targetLanguage)
	s.dictionaryRuns.set(key, translation.DictionaryProgress{Total: len(translation.ChunkNodes(document.TextNodes(), 0))})
	ws, _ := s.session.Current()
	go s.buildDictionary(context.Background(), store, ws.Root, ws.Config, series, book, targetLanguage, key)
	s.bookDetail(w, r)
}

func (s screens) buildDictionary(ctx context.Context, store library.Store, workspaceRoot string, config workspace.Config, series library.Series, book library.Book, targetLanguage, key string) {
	defer s.dictionaryRuns.clear(key)
	fail := func(err error) {
		log.Printf("Dictionary Building for %s/%s/%s: %v", series.Code, book.Code, targetLanguage, err)
		if statusErr := store.SetTranslationTargetStatus(series.Code, book.Code, targetLanguage, library.StatusFailed); statusErr != nil {
			log.Printf("record Dictionary Building failure: %v", statusErr)
		}
	}
	source, filename, err := store.SourceFile(series.Code, book.Code)
	if err != nil {
		fail(err)
		return
	}
	document, err := parseSourceFile(filename, source)
	if err != nil {
		fail(err)
		return
	}
	transcript, err := agent.NewFileLogger(filepath.Join(workspaceRoot, "logs", "agent-transcript.jsonl"))
	if err != nil {
		fail(err)
		return
	}
	client, err := s.agents(config.Agent, transcript)
	if err != nil {
		fail(err)
		return
	}
	terms, err := (translation.DictionaryBuilder{Agent: client, MechanicalModel: config.Models.Mechanical, OccurrenceThreshold: config.DictionaryOccurrenceThreshold}).Build(ctx, document.TextNodes(), series.SourceLanguage, targetLanguage, func(progress translation.DictionaryProgress) {
		s.dictionaryRuns.set(key, progress)
	})
	if err != nil {
		fail(err)
		return
	}
	if err := store.WriteDictionary(series.Code, book.Code, targetLanguage, terms); err != nil {
		fail(err)
		return
	}
	if err := store.SetTranslationTargetStatus(series.Code, book.Code, targetLanguage, library.StatusDictionaryReady); err != nil {
		log.Printf("record Dictionary Building completion: %v", err)
	}
}

func parseSourceFile(filename string, source []byte) (format.Document, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".txt":
		return (txt.Handler{}).Parse(source)
	case ".fb2":
		return (fb2.Handler{}).Parse(source)
	default:
		return nil, fmt.Errorf("unsupported Source File %q", filename)
	}
}

func (s screens) dictionaryProgress(w http.ResponseWriter, r *http.Request) {
	s.bookDetail(w, r)
}

func (s screens) detail(w http.ResponseWriter, r *http.Request, problem string) {
	store, ok := s.current()
	if !ok {
		s.reply(w, r, form{})
		return
	}
	lib, err := store.Library()
	if err != nil {
		s.reply(w, r, form{})
		return
	}
	series, book, ok := lib.Book(r.PathValue("series"), r.PathValue("book"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	fragment(w, "book-detail", s.bookDetailData(series, book, problem))
}

type bookDetailData struct {
	Series   library.Series
	Book     library.Book
	Allowed  []targetOption
	Progress map[string]translation.DictionaryProgress
	Problem  string
}

func (s screens) bookDetailData(series library.Series, book library.Book, problem string) bookDetailData {
	ws, _ := s.session.Current()
	allowed := make([]targetOption, 0, len(ws.Config.Languages))
	existing := make(map[string]library.Status, len(book.Targets))
	progress := make(map[string]translation.DictionaryProgress, len(book.Targets))
	for _, target := range book.Targets {
		existing[target.Language] = target.Status
		if target.Status == library.StatusAnalyzing {
			progress[target.Language] = s.dictionaryRuns.get(dictionaryKey(series.Code, book.Code, target.Language))
		}
	}
	for _, tag := range ws.Config.Languages {
		if language, ok := workspace.LanguageFor(tag); ok && tag != series.SourceLanguage {
			allowed = append(allowed, targetOption{Language: language, Status: existing[tag]})
		}
	}
	return bookDetailData{Series: series, Book: book, Allowed: allowed, Progress: progress, Problem: problem}
}

func dictionaryKey(seriesCode, bookCode, targetLanguage string) string {
	return seriesCode + "/" + bookCode + "/" + targetLanguage
}

// welcome is what a user with no workspace sees, and carries whatever the
// session has to say about why they are seeing it.
type welcome struct{ Problem string }

// libraryScreen is everything the library page renders.
type libraryScreen struct {
	Library         library.Library
	Form            form
	Languages       []workspace.Language
	TargetLanguages []string
}

// The two panels the library page carries, named so that a rejection can
// reopen the one it came from.
const (
	seriesPanel = "series"
	bookPanel   = "book"
)

// form is what the user typed into one panel and what was wrong with it. It
// exists because a rejection re-renders the whole screen — there is no
// client-side state to preserve the fields, so the server hands them back.
type form struct {
	Panel    string
	Problem  string
	Series   string
	Name     string
	Code     string
	Title    string
	Author   string
	Language string
}

// For is the state to render the named panel with: what the user typed, if
// this is the panel they were typing into, and nothing if it is not.
//
// It exists so the template asks once per panel instead of guarding every
// field, and so a rejection in one panel cannot leak into the other.
func (f form) For(panel string) form {
	if f.Panel != panel {
		return form{}
	}
	return f
}

// screen picks the screen the session's state calls for.
//
// The library is read here rather than held, so a workspace that has gone away
// with its drive is reported the moment the user asks for a screen instead of
// being served from a stale copy.
func (s screens) screen(typed form) (string, any) {
	store, ok := s.current()
	if !ok {
		return "welcome", welcome{Problem: s.session.Problem()}
	}
	lib, err := store.Library()
	if err != nil {
		// There is a workspace as far as the session knows, and no library to
		// read out of it. The picker is the only thing the user can do about
		// that, so it is the screen they get.
		return "welcome", welcome{Problem: err.Error()}
	}
	ws, _ := s.session.Current()
	return "library", libraryScreen{Library: lib, Form: typed, Languages: workspace.Catalog(), TargetLanguages: ws.Config.Languages}
}

// current is the store for the open workspace, if one is open.
func (s screens) current() (library.Store, bool) {
	ws, ok := s.session.Current()
	if !ok {
		return library.Store{}, false
	}
	return library.NewStore(ws.Root), true
}

// store answers with the store for the open workspace, or renders the screen
// that says there is not one.
func (s screens) store(w http.ResponseWriter, r *http.Request) (library.Store, bool) {
	store, ok := s.current()
	if !ok {
		s.reply(w, r, form{})
	}
	return store, ok
}

// reply renders the screen the session calls for, the way the request asked
// for it.
func (s screens) reply(w http.ResponseWriter, r *http.Request, typed form) {
	name, data := s.screen(typed)
	reply(w, r, name, data)
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
