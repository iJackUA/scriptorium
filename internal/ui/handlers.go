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
	"github.com/ijackua/scriptorium/internal/metadata"
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
	mux.HandleFunc("GET /books", s.booksList)
	mux.HandleFunc("GET /series/{series}/books/{book}", s.bookDetail)
	mux.HandleFunc("POST /series/{series}/books/{book}/source", s.uploadSourceFile)
	mux.HandleFunc("POST /series/{series}/books/{book}/metadata", s.updateBookMetadata)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets", s.createTarget)
	mux.HandleFunc("DELETE /series/{series}/books/{book}/targets/{target}", s.deleteTarget)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets/{target}/dictionary", s.startDictionaryBuilding)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets/{target}/dictionary/stop", s.stopDictionaryBuilding)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets/{target}/chunks", s.prepareTextChunks)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets/{target}/translate", s.startTranslation)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets/{target}/validate-repair", s.validateAndRepair)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets/{target}/retry-failed", s.retryFailedChunks)
	mux.HandleFunc("GET /series/{series}/books/{book}/targets/{target}/dictionary.tsv", s.bookDictionaryTSV)
	mux.HandleFunc("GET /series/{series}/dictionaries/{target}", s.seriesDictionaryTSV)
	mux.HandleFunc("GET /series/{series}/books/{book}/targets/{target}/dictionary/review", s.dictionaryReview)
	mux.HandleFunc("GET /series/{series}/books/{book}/targets/{target}/dictionary/edit", s.editDictionary)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets/{target}/dictionary/edit", s.saveDictionary)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets/{target}/dictionary/promote", s.promoteDictionaryTerm)
	mux.HandleFunc("POST /series/{series}/books/{book}/targets/{target}/dictionary/unpromote", s.unpromoteDictionaryTerm)
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
	mu     sync.Mutex
	nextID uint64
	runs   map[string]dictionaryRun
}

type dictionaryRun struct {
	id       uint64
	cancel   context.CancelFunc
	progress translation.DictionaryProgress
}

func newDictionaryRuns() *dictionaryRuns {
	return &dictionaryRuns{runs: make(map[string]dictionaryRun)}
}

func (r *dictionaryRuns) start(key string, progress translation.DictionaryProgress) (context.Context, uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	ctx, cancel := context.WithCancel(context.Background())
	r.runs[key] = dictionaryRun{id: r.nextID, cancel: cancel, progress: progress}
	return ctx, r.nextID
}

func (r *dictionaryRuns) set(key string, id uint64, progress translation.DictionaryProgress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[key]
	if ok && run.id == id {
		run.progress = progress
		r.runs[key] = run
	}
}

func (r *dictionaryRuns) get(key string) translation.DictionaryProgress {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runs[key].progress
}

func (r *dictionaryRuns) stop(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if run, ok := r.runs[key]; ok {
		run.cancel()
		delete(r.runs, key)
	}
}

func (r *dictionaryRuns) clear(key string, id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if run, ok := r.runs[key]; ok && run.id == id {
		delete(r.runs, key)
	}
}

// finish serializes terminal Status writes with Stop. If Stop wins, it removes
// this run and writes New; a late completion must not overwrite that choice.
func (r *dictionaryRuns) finish(key string, id uint64, update func()) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[key]
	if !ok || run.id != id {
		return false
	}
	update()
	delete(r.runs, key)
	return true
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
	s.renderBookDetail(w, r, lib, series, book, "", "")
}

// booksList rereads the workspace before rendering the sidebar, so a user can
// explicitly pick up Books or status changes made outside Scriptorium.
func (s screens) booksList(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	lib, err := store.Library()
	if err != nil {
		s.reply(w, r, form{})
		return
	}
	fragment(w, "books-list", libraryScreen{Library: lib})
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
	source, err := io.ReadAll(file)
	if err != nil {
		s.detail(w, r, "read Source File: "+err.Error())
		return
	}
	seriesCode, bookCode := r.PathValue("series"), r.PathValue("book")
	if err := store.UploadSourceFile(seriesCode, bookCode, header.Filename, bytes.NewReader(source), confirmed); err != nil {
		s.detail(w, r, err.Error())
		return
	}
	s.fillMetadata(store, seriesCode, bookCode, header.Filename, source)
	s.bookDetail(w, r)
}

func (s screens) fillMetadata(store library.Store, seriesCode, bookCode, filename string, source []byte) {
	var fields metadata.Fields
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".fb2":
		fields = metadata.FB2(source)
	case ".txt":
		ws, ok := s.session.Current()
		if !ok {
			return
		}
		client, err := s.agents(ws.Config.Agent, nil)
		if err != nil {
			return
		}
		inferred, err := metadata.InferText(context.Background(), client, ws.Config.Models.Mechanical, source)
		if err != nil {
			return
		}
		fields = inferred
	}
	if fields.SourceFileLanguage != "" {
		if _, ok := workspace.LanguageFor(fields.SourceFileLanguage); !ok {
			fields.SourceFileLanguage = ""
		}
	}
	_ = store.FillBookMetadata(seriesCode, bookCode, library.BookMetadata(fields))
}

func (s screens) updateBookMetadata(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	fields := library.BookMetadata{Title: r.FormValue("title"), Author: r.FormValue("author"), SourceFileLanguage: r.FormValue("source_file_language")}
	if fields.SourceFileLanguage != "" {
		if _, ok := workspace.LanguageFor(fields.SourceFileLanguage); !ok {
			s.detail(w, r, "choose a language from the catalog")
			return
		}
	}
	if err := store.UpdateBookMetadata(r.PathValue("series"), r.PathValue("book"), fields); err != nil {
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

func (s screens) prepareTextChunks(w http.ResponseWriter, r *http.Request) {
	_, ok := s.store(w, r)
	if !ok {
		return
	}
	ws, ok := s.session.Current()
	if !ok {
		s.reply(w, r, form{})
		return
	}
	translator := translation.Translator{Root: ws.Root}
	count, err := translator.PrepareTextChunks(r.PathValue("series"), r.PathValue("book"))
	if err != nil {
		s.detail(w, r, err.Error())
		return
	}
	s.detailNotice(w, r, fmt.Sprintf("Prepared %d Text Chunks.", count))
}

func (s screens) startTranslation(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	ws, ok := s.session.Current()
	if !ok {
		s.reply(w, r, form{})
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
	if target.Language == "" || target.Status != library.StatusDictionaryReady {
		s.detail(w, r, "translation can only start when the Dictionary is ready")
		return
	}
	translator := translation.Translator{Root: ws.Root}
	prepared, err := translator.PreparedTextChunksPresent(seriesCode, bookCode)
	if err != nil {
		s.detail(w, r, err.Error())
		return
	}
	if !prepared {
		s.detail(w, r, "prepare Text Chunks before starting translation")
		return
	}
	go s.translateBook(ws, series, book, targetLanguage)
	s.bookDetail(w, r)
}

func (s screens) validateAndRepair(w http.ResponseWriter, r *http.Request) {
	s.startRepair(w, r, false)
}

func (s screens) retryFailedChunks(w http.ResponseWriter, r *http.Request) {
	s.startRepair(w, r, true)
}

func (s screens) startRepair(w http.ResponseWriter, r *http.Request, onlyFailed bool) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	ws, ok := s.session.Current()
	if !ok {
		s.reply(w, r, form{})
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
	for _, target := range book.Targets {
		if target.Language != targetLanguage {
			continue
		}
		if target.Status == library.StatusNew || target.Status == library.StatusAnalyzing {
			s.detail(w, r, "Dictionary Building must finish before repairing translation Chunks")
			return
		}
		go s.repairBook(ws, series, book, targetLanguage, onlyFailed)
		s.bookDetail(w, r)
		return
	}
	http.NotFound(w, r)
}

func (s screens) translateBook(ws workspace.Workspace, series library.Series, book library.Book, targetLanguage string) {
	transcript, err := agent.NewFileLogger(filepath.Join(ws.Root, "logs", "agent-transcript.jsonl"))
	if err != nil {
		log.Printf("translation for %s/%s/%s: %v", series.Code, book.Code, targetLanguage, err)
		s.recordTranslationFailure(ws.Root, series, book, targetLanguage, err)
		return
	}
	client, err := s.agents(ws.Config.Agent, transcript)
	if err != nil {
		log.Printf("translation for %s/%s/%s: %v", series.Code, book.Code, targetLanguage, err)
		s.recordTranslationFailure(ws.Root, series, book, targetLanguage, err)
		return
	}
	_, err = (translation.Translator{
		Root:             ws.Root,
		Agent:            client,
		TranslationModel: ws.Config.Models.Translation,
	}).Translate(context.Background(), series.Code, book.Code, targetLanguage)
	if err != nil {
		log.Printf("translation for %s/%s/%s: %v", series.Code, book.Code, targetLanguage, err)
		s.recordTranslationFailure(ws.Root, series, book, targetLanguage, err)
	}
}

func (s screens) repairBook(ws workspace.Workspace, series library.Series, book library.Book, targetLanguage string, onlyFailed bool) {
	transcript, err := agent.NewFileLogger(filepath.Join(ws.Root, "logs", "agent-transcript.jsonl"))
	if err != nil {
		log.Printf("translation repair for %s/%s/%s: %v", series.Code, book.Code, targetLanguage, err)
		s.recordTranslationFailure(ws.Root, series, book, targetLanguage, err)
		return
	}
	client, err := s.agents(ws.Config.Agent, transcript)
	if err != nil {
		log.Printf("translation repair for %s/%s/%s: %v", series.Code, book.Code, targetLanguage, err)
		s.recordTranslationFailure(ws.Root, series, book, targetLanguage, err)
		return
	}
	translator := translation.Translator{Root: ws.Root, Agent: client, TranslationModel: ws.Config.Models.Translation}
	if onlyFailed {
		_, err = translator.RetryFailedChunks(context.Background(), series.Code, book.Code, targetLanguage)
	} else {
		_, err = translator.ValidateAndRepair(context.Background(), series.Code, book.Code, targetLanguage)
	}
	if err != nil {
		log.Printf("translation repair for %s/%s/%s: %v", series.Code, book.Code, targetLanguage, err)
		s.recordTranslationFailure(ws.Root, series, book, targetLanguage, err)
	}
}

func (s screens) recordTranslationFailure(root string, series library.Series, book library.Book, targetLanguage string, cause error) {
	if err := library.NewStore(root).SetTranslationTargetStatus(series.Code, book.Code, targetLanguage, library.StatusFailed); err != nil {
		log.Printf("record translation failure after %v: %v", cause, err)
	}
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
	ctx, runID := s.dictionaryRuns.start(key, translation.DictionaryProgress{Active: 1, Total: len(translation.ChunkNodes(document.TextNodes(), 0))})
	ws, _ := s.session.Current()
	go s.buildDictionary(ctx, store, ws.Root, ws.Config, series, book, targetLanguage, key, runID)
	s.bookDetail(w, r)
}

func (s screens) stopDictionaryBuilding(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	seriesCode, bookCode, targetLanguage := r.PathValue("series"), r.PathValue("book"), r.PathValue("target")
	key := dictionaryKey(seriesCode, bookCode, targetLanguage)
	s.dictionaryRuns.stop(key)
	if err := store.SetTranslationTargetStatus(seriesCode, bookCode, targetLanguage, library.StatusNew); err != nil {
		s.detail(w, r, err.Error())
		return
	}
	s.bookDetail(w, r)
}

func (s screens) bookDictionaryTSV(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	body, err := store.BookDictionaryTSV(r.PathValue("series"), r.PathValue("book"), r.PathValue("target"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/tab-separated-values; charset=utf-8")
	_, _ = w.Write(body)
}

func (s screens) seriesDictionaryTSV(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	targetLanguage := strings.TrimSuffix(r.PathValue("target"), ".tsv")
	if targetLanguage == r.PathValue("target") {
		http.NotFound(w, r)
		return
	}
	body, err := store.SeriesDictionaryTSV(r.PathValue("series"), targetLanguage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/tab-separated-values; charset=utf-8")
	_, _ = w.Write(body)
}

func (s screens) promoteDictionaryTerm(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	if err := store.PromoteDictionaryTerm(r.PathValue("series"), r.PathValue("book"), r.PathValue("target"), r.FormValue("term")); err != nil {
		s.detail(w, r, err.Error())
		return
	}
	s.dictionaryReview(w, r)
}

func (s screens) unpromoteDictionaryTerm(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	if err := store.UnpromoteDictionaryTerm(r.PathValue("series"), r.PathValue("target"), r.FormValue("term")); err != nil {
		s.detail(w, r, err.Error())
		return
	}
	s.dictionaryReview(w, r)
}

func (s screens) dictionaryReview(w http.ResponseWriter, r *http.Request) {
	review, ok := s.loadDictionaryReview(w, r)
	if !ok {
		return
	}
	fragment(w, "dictionary-review-content", review)
}

func (s screens) editDictionary(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	seriesCode, bookCode, targetLanguage := r.PathValue("series"), r.PathValue("book"), r.PathValue("target")
	body, err := store.BookDictionaryTSV(seriesCode, bookCode, targetLanguage)
	if err != nil {
		s.detail(w, r, err.Error())
		return
	}
	fragment(w, "dictionary-editor", dictionaryEditorData{
		SeriesCode: seriesCode,
		BookCode:   bookCode,
		Language:   targetLanguage,
		TSV:        string(body),
	})
}

func (s screens) saveDictionary(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	seriesCode, bookCode, targetLanguage := r.PathValue("series"), r.PathValue("book"), r.PathValue("target")
	tsv := r.FormValue("tsv")
	if err := store.UpdateBookDictionaryTSV(seriesCode, bookCode, targetLanguage, []byte(tsv)); err != nil {
		fragment(w, "dictionary-editor", dictionaryEditorData{
			SeriesCode: seriesCode,
			BookCode:   bookCode,
			Language:   targetLanguage,
			TSV:        tsv,
			Problem:    err.Error(),
		})
		return
	}
	s.dictionaryReview(w, r)
}

func (s screens) loadDictionaryReview(w http.ResponseWriter, r *http.Request) (dictionaryReview, bool) {
	store, ok := s.store(w, r)
	if !ok {
		return dictionaryReview{}, false
	}
	lib, err := store.Library()
	if err != nil {
		s.detail(w, r, err.Error())
		return dictionaryReview{}, false
	}
	series, book, found := lib.Book(r.PathValue("series"), r.PathValue("book"))
	if !found {
		http.NotFound(w, r)
		return dictionaryReview{}, false
	}
	targetLanguage := r.PathValue("target")
	review := s.reviewDictionary(store, series, book, targetLanguage)
	return review, true
}

func (s screens) buildDictionary(ctx context.Context, store library.Store, workspaceRoot string, config workspace.Config, series library.Series, book library.Book, targetLanguage, key string, runID uint64) {
	defer s.dictionaryRuns.clear(key, runID)
	fail := func(err error) {
		if ctx.Err() != nil {
			return
		}
		log.Printf("Dictionary Building for %s/%s/%s: %v", series.Code, book.Code, targetLanguage, err)
		s.dictionaryRuns.finish(key, runID, func() {
			if statusErr := store.SetTranslationTargetStatus(series.Code, book.Code, targetLanguage, library.StatusFailed); statusErr != nil {
				log.Printf("record Dictionary Building failure: %v", statusErr)
			}
		})
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
		s.dictionaryRuns.set(key, runID, progress)
	})
	if err != nil {
		fail(err)
		return
	}
	if err := store.WriteDictionary(series.Code, book.Code, targetLanguage, terms); err != nil {
		fail(err)
		return
	}
	s.dictionaryRuns.finish(key, runID, func() {
		if err := store.SetTranslationTargetStatus(series.Code, book.Code, targetLanguage, library.StatusDictionaryReady); err != nil {
			log.Printf("record Dictionary Building completion: %v", err)
		}
	})
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
	store, ok := s.current()
	if !ok {
		s.reply(w, r, form{})
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
	for _, target := range book.Targets {
		if target.Language != targetLanguage {
			continue
		}
		if target.Status != library.StatusAnalyzing {
			w.Header().Set("HX-Retarget", "#book-detail")
			w.Header().Set("HX-Reswap", "innerHTML")
			s.bookDetail(w, r)
			return
		}
		fragment(w, "dictionary-progress", dictionaryProgressData{
			SeriesCode: series.Code,
			BookCode:   book.Code,
			Language:   target.Language,
			Progress:   s.dictionaryRuns.get(dictionaryKey(series.Code, book.Code, target.Language)),
		})
		return
	}
	http.NotFound(w, r)
}

func (s screens) detail(w http.ResponseWriter, r *http.Request, problem string) {
	s.renderDetail(w, r, problem, "")
}

func (s screens) detailNotice(w http.ResponseWriter, r *http.Request, notice string) {
	s.renderDetail(w, r, "", notice)
}

func (s screens) renderDetail(w http.ResponseWriter, r *http.Request, problem, notice string) {
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
	s.renderBookDetail(w, r, lib, series, book, problem, notice)
}

// renderBookDetail keeps the selected Book and the Books sidebar in lockstep.
// Every action rereads the Library from disk, then htmx swaps the detail and
// receives the newly rendered sidebar as an out-of-band update.
func (s screens) renderBookDetail(w http.ResponseWriter, r *http.Request, lib library.Library, series library.Series, book library.Book, problem, notice string) {
	data := s.bookDetailData(series, book, problem)
	data.Notice = notice
	body, err := execute("book-detail", data)
	if err != nil {
		fail(w, "book-detail", err)
		return
	}
	if r.Header.Get("HX-Request") == "true" {
		books, err := execute("books-list-oob", libraryScreen{Library: lib})
		if err != nil {
			fail(w, "books-list-oob", err)
			return
		}
		body += books
	}
	write(w, body)
}

type bookDetailData struct {
	Series       library.Series
	Book         library.Book
	Allowed      []targetOption
	Languages    []workspace.Language
	Progress     map[string]translation.DictionaryProgress
	Dictionaries map[string]dictionaryReview
	Prepared     map[string]bool
	Notice       string
	Problem      string
}

type dictionaryReview struct {
	SeriesCode string
	BookCode   string
	Language   string
	Terms      []dictionaryReviewTerm
	Warning    bool
	Problem    string
}

type dictionaryReviewTerm struct {
	library.Term
	Promoted bool
}

type dictionaryEditorData struct {
	SeriesCode string
	BookCode   string
	Language   string
	TSV        string
	Problem    string
}

type dictionaryProgressData struct {
	SeriesCode string
	BookCode   string
	Language   string
	Progress   translation.DictionaryProgress
}

func (s screens) bookDetailData(series library.Series, book library.Book, problem string) bookDetailData {
	ws, _ := s.session.Current()
	allowed := make([]targetOption, 0, len(ws.Config.Languages))
	existing := make(map[string]library.Status, len(book.Targets))
	progress := make(map[string]translation.DictionaryProgress, len(book.Targets))
	dictionaries := make(map[string]dictionaryReview, len(book.Targets))
	prepared := make(map[string]bool, len(book.Targets))
	translator := translation.Translator{Root: ws.Root}
	for _, target := range book.Targets {
		if ready, err := translator.PreparedTextChunksPresent(series.Code, book.Code); err == nil {
			prepared[target.Language] = ready
		}
		existing[target.Language] = target.Status
		if target.Status != library.StatusNew && target.Status != library.StatusAnalyzing {
			store, ok := s.current()
			if ok {
				dictionaries[target.Language] = s.reviewDictionary(store, series, book, target.Language)
			}
		}
		if target.Status == library.StatusAnalyzing {
			progress[target.Language] = s.dictionaryRuns.get(dictionaryKey(series.Code, book.Code, target.Language))
		}
	}
	for _, tag := range ws.Config.Languages {
		if language, ok := workspace.LanguageFor(tag); ok && tag != series.SourceLanguage {
			allowed = append(allowed, targetOption{Language: language, Status: existing[tag]})
		}
	}
	return bookDetailData{Series: series, Book: book, Allowed: allowed, Languages: workspace.Catalog(), Progress: progress, Dictionaries: dictionaries, Prepared: prepared, Problem: problem}
}

func (s screens) reviewDictionary(store library.Store, series library.Series, book library.Book, targetLanguage string) dictionaryReview {
	review := dictionaryReview{
		SeriesCode: series.Code,
		BookCode:   book.Code,
		Language:   targetLanguage,
	}
	bookTerms, err := store.BookDictionary(series.Code, book.Code, targetLanguage)
	if err != nil {
		review.Problem = err.Error()
		return review
	}
	seriesTerms, err := store.SeriesDictionary(series.Code, targetLanguage)
	if err != nil {
		review.Problem = err.Error()
		return review
	}
	promoted := make(map[string]bool, len(seriesTerms))
	for _, term := range seriesTerms {
		promoted[term.Original] = true
	}
	for _, term := range bookTerms {
		review.Terms = append(review.Terms, dictionaryReviewTerm{Term: term, Promoted: promoted[term.Original]})
	}
	mergedTerms, err := store.Dictionary(series.Code, book.Code, targetLanguage)
	if err != nil {
		review.Problem = err.Error()
		return review
	}
	review.Warning = len(mergedTerms) > 100
	return review
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

// The library's forms, named so that a rejection can reopen the modal it came
// from.
const (
	seriesPanel = "series"
	bookPanel   = "book"
)

// form is what the user typed into one modal and what was wrong with it. It
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

// For is the state to render the named modal with: what the user typed, if
// this is the modal they were typing into, and nothing if it is not.
//
// It exists so the template asks once per modal instead of guarding every
// field, and so a rejection in one modal cannot leak into the other.
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
