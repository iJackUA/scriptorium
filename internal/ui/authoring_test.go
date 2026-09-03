package ui

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/translation"
	"github.com/ijackua/scriptorium/internal/workspace"
)

// postForm submits a form the way htmx submits it, since that is the only way
// these routes are reached from the interface.
func postForm(s *Server, path string, values url.Values) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, s.URL()+path, strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

func postSourceFile(s *Server, path, name, contents string, confirmed bool) *httptest.ResponseRecorder {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("source", name)
	if err != nil {
		panic(err)
	}
	if _, err := file.Write([]byte(contents)); err != nil {
		panic(err)
	}
	if confirmed {
		if err := form.WriteField("confirmed", "true"); err != nil {
			panic(err)
		}
	}
	if err := form.Close(); err != nil {
		panic(err)
	}
	r := httptest.NewRequest(http.MethodPost, s.URL()+path, &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	r.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

// newEmptyLibraryServer serves an opened workspace with nothing in it, which is
// what a user sees the moment after choosing a folder.
func newEmptyLibraryServer(t *testing.T) (*Server, string) {
	t.Helper()
	root := t.TempDir()
	return newTestServerFor(t, sessionFor(t, root)), root
}

func TestCreatingASeriesShowsItInTheLibrary(t *testing.T) {
	s, root := newEmptyLibraryServer(t)

	rec := postForm(s, "/series", url.Values{"name": {"Solaris"}, "language": {"pl"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); strings.Contains(body, "form-problem") {
		t.Errorf("creating a Series was rejected:\n%s", body)
	}

	if _, err := os.Stat(filepath.Join(root, "solaris", "series.toml")); err != nil {
		t.Errorf("series.toml was not written: %v", err)
	}
	// A Series with no Books yet has no Book to render, so what proves it is
	// there is the next screen offering it to add one to.
	body := serve(s, request(s, "/")).Body.String()
	if !strings.Contains(body, `value="solaris"`) {
		t.Errorf("the new Series is not offered to add a Book to:\n%s", body)
	}
}

func TestAddingABookToASeriesShowsItUnderThatSeries(t *testing.T) {
	s, root := newEmptyLibraryServer(t)
	if rec := postForm(s, "/series", url.Values{"name": {"Solaris"}, "language": {"pl"}}); rec.Code != http.StatusOK {
		t.Fatalf("CreateSeries: got %d", rec.Code)
	}

	rec := postForm(s, "/books", url.Values{
		"series": {"solaris"}, "code": {"solaris"}, "title": {"Solaris"}, "author": {"Stanisław Lem"},
	})
	body := rec.Body.String()
	if strings.Contains(body, "form-problem") {
		t.Fatalf("adding a Book was rejected:\n%s", body)
	}
	for _, want := range []string{"Solaris", "Stanisław Lem"} {
		if !strings.Contains(body, want) {
			t.Errorf("the library does not show %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "solaris", "books", "solaris", "book.toml")); err != nil {
		t.Errorf("book.toml was not written: %v", err)
	}
}

// The user picks "On its own" and is asked nothing else about Series; behind
// the screen a real Series of one is created, and it renders flat.
func TestABookAddedOnItsOwnRendersFlat(t *testing.T) {
	s, root := newEmptyLibraryServer(t)

	rec := postForm(s, "/books", url.Values{
		"series": {""}, "code": {"solaris"}, "title": {"Solaris"}, "author": {"Stanisław Lem"}, "language": {"pl"},
	})
	body := rec.Body.String()
	if strings.Contains(body, "form-problem") {
		t.Fatalf("adding a standalone Book was rejected:\n%s", body)
	}
	if !strings.Contains(body, "Solaris") {
		t.Error("the standalone Book is not in the library")
	}
	if strings.Contains(body, "card-title") {
		t.Error("a Series of one was given a group header")
	}

	// It is a real Series on disk all the same, so a sequel needs no migration.
	if _, err := os.Stat(filepath.Join(root, "solaris", "series.toml")); err != nil {
		t.Errorf("the Series of one is not on disk: %v", err)
	}
	if rec := postForm(s, "/books", url.Values{
		"series": {"solaris"}, "code": {"eden"}, "title": {"Eden"},
	}); strings.Contains(rec.Body.String(), "form-problem") {
		t.Errorf("the sequel was rejected:\n%s", rec.Body.String())
	}
	if body := serve(s, request(s, "/")).Body.String(); !strings.Contains(body, "card-title") {
		t.Error("a Series of two still renders flat")
	}
}

func TestARejectedBookCodeIsReportedWithTheFormStillFilledIn(t *testing.T) {
	s := newTestServer(t)
	series := "the-adventures-of-sherlock-holmes"

	for name, values := range map[string]url.Values{
		"already used": {"series": {series}, "code": {"memoirs"}, "title": {"Something Else"}, "author": {"Someone"}},
		"not a folder": {"series": {series}, "code": {"has spaces"}, "title": {"Something Else"}, "author": {"Someone"}},
	} {
		rec := postForm(s, "/books", values)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: got %d, want %d", name, rec.Code, http.StatusOK)
		}
		body := rec.Body.String()

		if !strings.Contains(body, "form-problem") {
			t.Errorf("%s: the rejection was not reported on screen:\n%s", name, body)
		}
		if !strings.Contains(body, `id="add-book-modal"`) || !strings.Contains(body, "data-open-on-render") {
			t.Errorf("%s: the rejected Book form did not return in an openable modal:\n%s", name, body)
		}
		// Retyping the whole form to fix one field is what the round trip
		// through the server threatens, so the values come back with it.
		for _, want := range []string{values.Get("code"), "Something Else", "Someone"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: the form came back without %q", name, want)
			}
		}
	}
}

func TestLibraryActionsOpenTheAuthoringAndSettingsModals(t *testing.T) {
	s, _ := newEmptyLibraryServer(t)
	body := serve(s, request(s, "/")).Body.String()

	for _, want := range []string{
		">Add Book</button>", ">Add Series</button>", `aria-label="Workspace settings"`,
		`id="add-book-modal"`, `id="add-series-modal"`, `id="workspace-settings-modal"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("library navigation is missing %q:\n%s", want, body)
		}
	}
}

// Nothing is written until every reason to refuse has been found, so a
// rejection cannot leave a half-made Book behind.
func TestARejectedBookLeavesTheWorkspaceAlone(t *testing.T) {
	root := t.TempDir()
	seedLibrary(t, root)
	s := newTestServerFor(t, sessionFor(t, root))
	series := filepath.Join(root, "the-adventures-of-sherlock-holmes")

	before := entries(t, filepath.Join(series, "books"))
	memoirs := read(t, filepath.Join(series, "books", "memoirs", "book.toml"))

	postForm(s, "/books", url.Values{"series": {"the-adventures-of-sherlock-holmes"}, "code": {"memoirs"}, "title": {"Something Else"}})
	postForm(s, "/books", url.Values{"series": {"the-adventures-of-sherlock-holmes"}, "code": {"../escape"}, "title": {"Something Else"}})

	if after := entries(t, filepath.Join(series, "books")); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Errorf("the Series holds %v, wanted it unchanged at %v", after, before)
	}
	if got := read(t, filepath.Join(series, "books", "memoirs", "book.toml")); got != memoirs {
		t.Errorf("the Book that was already there was rewritten:\n%s", got)
	}
}

// A Book is added before its Source File exists, so the title comes out of
// that file rather than out of the user. All they are asked for is the code.
func TestABookCanBeAddedWithNothingButItsCode(t *testing.T) {
	s, root := newEmptyLibraryServer(t)

	rec := postForm(s, "/books", url.Values{"series": {""}, "code": {"solaris"}, "language": {"pl"}})
	body := rec.Body.String()
	if strings.Contains(body, "form-problem") {
		t.Fatalf("a Book with no title was rejected:\n%s", body)
	}
	// With no title yet, the Book Code is what it is called on screen.
	if !strings.Contains(body, "solaris") {
		t.Error("the untitled Book is not in the library")
	}
	if _, err := os.Stat(filepath.Join(root, "solaris", "books", "solaris", "book.toml")); err != nil {
		t.Errorf("book.toml was not written: %v", err)
	}
}

func TestWorkspaceTargetLanguagesCreateAndRenderTranslationTargets(t *testing.T) {
	s, root := newEmptyLibraryServer(t)
	if body := serve(s, request(s, "/")).Body.String(); !strings.Contains(body, "Workspace settings") {
		t.Fatal("Workspace settings are not reachable from the library")
	}
	if body := postForm(s, "/settings/target-languages", url.Values{"languages": {"uk", "de"}}).Body.String(); !strings.Contains(body, "uk") {
		t.Fatalf("updated Target Languages are not shown: %s", body)
	}
	if rec := postForm(s, "/series", url.Values{"name": {"Solaris"}, "language": {"pl"}}); strings.Contains(rec.Body.String(), "form-problem") {
		t.Fatalf("CreateSeries: %s", rec.Body.String())
	}
	if rec := postForm(s, "/books", url.Values{"series": {"solaris"}, "code": {"solaris"}}); strings.Contains(rec.Body.String(), "form-problem") {
		t.Fatalf("AddBook: %s", rec.Body.String())
	}
	if body := serve(s, request(s, "/series/solaris/books/solaris")).Body.String(); !strings.Contains(body, "Ukrainian (uk)") {
		t.Fatalf("Book details do not offer allowlisted languages: %s", body)
	}
	rec := postForm(s, "/series/solaris/books/solaris/targets", url.Values{"language": {"uk"}})
	if body := rec.Body.String(); !strings.Contains(body, "Ukrainian (uk)") || !strings.Contains(body, "New") {
		t.Fatalf("Translation Target is not rendered: %s", body)
	}
	if _, err := os.Stat(filepath.Join(root, "solaris", "books", "solaris", "translations", "pl-to-uk", "state.json")); err != nil {
		t.Fatalf("target state was not written: %v", err)
	}
}

func TestBookDetailsUploadAndReplaceASourceFile(t *testing.T) {
	s, root := newEmptyLibraryServer(t)
	if rec := postForm(s, "/series", url.Values{"name": {"Solaris"}, "language": {"pl"}}); strings.Contains(rec.Body.String(), "form-problem") {
		t.Fatalf("CreateSeries: %s", rec.Body.String())
	}
	if rec := postForm(s, "/books", url.Values{"series": {"solaris"}, "code": {"solaris"}}); strings.Contains(rec.Body.String(), "form-problem") {
		t.Fatalf("AddBook: %s", rec.Body.String())
	}
	path := "/series/solaris/books/solaris/source"
	if body := serve(s, request(s, "/series/solaris/books/solaris")).Body.String(); !strings.Contains(body, "No Source File uploaded") || !strings.Contains(body, "Upload Source File") {
		t.Fatalf("a Book without a Source File is not visibly actionable: %s", body)
	}

	if rec := postSourceFile(s, path, "solaris.txt", "original", false); !strings.Contains(rec.Body.String(), "source.txt") {
		t.Fatalf("uploaded Source File is not shown: %s", rec.Body.String())
	}
	stored := filepath.Join(root, "solaris", "books", "solaris", "source.txt")
	if got := read(t, stored); got != "original" {
		t.Errorf("stored Source File = %q", got)
	}

	if rec := postSourceFile(s, path, "solaris.fb2", "replacement", false); !strings.Contains(rec.Body.String(), "discards all existing translation work") {
		t.Fatalf("replacement warning is not shown: %s", rec.Body.String())
	}
	if got := read(t, stored); got != "original" {
		t.Errorf("warning request changed the Source File to %q", got)
	}
	if rec := postSourceFile(s, path, "solaris.fb2", "replacement", true); !strings.Contains(rec.Body.String(), "source.fb2") {
		t.Fatalf("confirmed replacement is not shown: %s", rec.Body.String())
	}
}

func TestNewTranslationTargetWithASourceFileOffersDictionaryBuilding(t *testing.T) {
	s, _ := newEmptyLibraryServer(t)
	if rec := postForm(s, "/settings/target-languages", url.Values{"languages": {"uk"}}); strings.Contains(rec.Body.String(), "form-problem") {
		t.Fatalf("Set Target Languages: %s", rec.Body.String())
	}
	if rec := postForm(s, "/series", url.Values{"name": {"Solaris"}, "language": {"pl"}}); strings.Contains(rec.Body.String(), "form-problem") {
		t.Fatalf("CreateSeries: %s", rec.Body.String())
	}
	if rec := postForm(s, "/books", url.Values{"series": {"solaris"}, "code": {"solaris"}}); strings.Contains(rec.Body.String(), "form-problem") {
		t.Fatalf("AddBook: %s", rec.Body.String())
	}
	if rec := postSourceFile(s, "/series/solaris/books/solaris/source", "solaris.txt", "Solaris", false); strings.Contains(rec.Body.String(), "form-problem") {
		t.Fatalf("UploadSourceFile: %s", rec.Body.String())
	}
	if rec := postForm(s, "/series/solaris/books/solaris/targets", url.Values{"language": {"uk"}}); strings.Contains(rec.Body.String(), "form-problem") {
		t.Fatalf("CreateTranslationTarget: %s", rec.Body.String())
	}

	body := serve(s, request(s, "/series/solaris/books/solaris")).Body.String()
	if !strings.Contains(body, "Start Dictionary Building") {
		t.Errorf("new Target with a Source File does not offer Dictionary Building:\n%s", body)
	}
}

func TestPrepareTextChunksActionPersistsBookChunks(t *testing.T) {
	root := t.TempDir()
	store := library.NewStore(root)
	series, _ := store.CreateSeries("Solaris", "en")
	_, _ = store.AddBook(series.Code, library.BookDraft{Code: "solaris"})
	if err := store.UploadSourceFile(series.Code, "solaris", "solaris.txt", strings.NewReader("First paragraph.\n\nSecond paragraph."), false); err != nil {
		t.Fatalf("UploadSourceFile: %v", err)
	}
	if _, err := store.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	s := newTestServerFor(t, sessionFor(t, root))

	rec := postForm(s, "/series/solaris/books/solaris/targets/uk/chunks", url.Values{})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Prepared 1 Text Chunks.") {
		t.Fatalf("Prepare Text Chunks response = %d\n%s", rec.Code, rec.Body.String())
	}
	for _, path := range []string{
		filepath.Join(root, "solaris", "books", "solaris", "chunks", "manifest.json"),
		filepath.Join(root, "solaris", "books", "solaris", "chunks", "original", "0.txt"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("prepared artifact %s is missing: %v", path, err)
		}
	}
	body := serve(s, request(s, "/series/solaris/books/solaris")).Body.String()
	for _, want := range []string{"Prepare Text Chunks", "Text Chunks prepared", "Start Dictionary Building"} {
		if !strings.Contains(body, want) {
			t.Errorf("Book panel is missing %q:\n%s", want, body)
		}
	}
	if !strings.Contains(body, "flex flex-col items-start gap-2") {
		t.Error("Translation Target actions are not laid out as a vertical block")
	}
}

func TestCompletedAndFailedTargetsOfferExplicitRepairActions(t *testing.T) {
	body, err := execute("book-detail", bookDetailData{
		Series: library.Series{Code: "series", Name: "Series", SourceLanguage: "en"},
		Book: library.Book{Code: "book", SourceFile: "source.txt", Targets: []library.TranslationTarget{
			{Language: "uk", Status: library.StatusTranslated},
			{Language: "de", Status: library.StatusFailed},
		}},
	})
	if err != nil {
		t.Fatalf("render repair actions: %v", err)
	}
	if strings.Count(body, "Validate and Repair") != 2 {
		t.Errorf("Validate and Repair buttons = %d, want two", strings.Count(body, "Validate and Repair"))
	}
	if strings.Count(body, "Retry failed Chunks") != 1 {
		t.Errorf("Retry failed Chunks buttons = %d, want one", strings.Count(body, "Retry failed Chunks"))
	}
	for _, want := range []string{"/targets/uk/validate-repair", "/targets/de/validate-repair", "/targets/de/retry-failed"} {
		if !strings.Contains(body, want) {
			t.Errorf("repair action markup is missing %q", want)
		}
	}
}

func TestStartTranslationRefusesToRunWithoutPreparedChunks(t *testing.T) {
	root := t.TempDir()
	store := library.NewStore(root)
	series, _ := store.CreateSeries("Solaris", "en")
	_, _ = store.AddBook(series.Code, library.BookDraft{Code: "solaris"})
	if err := store.UploadSourceFile(series.Code, "solaris", "solaris.txt", strings.NewReader("Source."), false); err != nil {
		t.Fatalf("UploadSourceFile: %v", err)
	}
	if _, err := store.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	if err := store.SetTranslationTargetStatus(series.Code, "solaris", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}
	fake := agent.NewFake(agent.Response{Result: "[[NODE 0]]\nПереклад.\n[[/NODE]]"})
	s, err := newServer(sessionFor(t, root), func(string, agent.Logger) (agent.Agent, error) { return fake, nil })
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	rec := postForm(s, "/series/solaris/books/solaris/targets/uk/translate", url.Values{})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "prepare Text Chunks before starting translation") {
		t.Fatalf("Start Translation response = %d\n%s", rec.Code, rec.Body.String())
	}
	if got := len(fake.RecordedRequests()); got != 0 {
		t.Fatalf("Agent requests = %d, want no request without prepared Chunks", got)
	}
}

func TestStartTranslationActionRunsPreparedBook(t *testing.T) {
	root := t.TempDir()
	store := library.NewStore(root)
	series, _ := store.CreateSeries("Solaris", "en")
	_, _ = store.AddBook(series.Code, library.BookDraft{Code: "solaris"})
	if err := store.UploadSourceFile(series.Code, "solaris", "solaris.txt", strings.NewReader("Source."), false); err != nil {
		t.Fatalf("UploadSourceFile: %v", err)
	}
	if _, err := store.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	if err := store.SetTranslationTargetStatus(series.Code, "solaris", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}
	if _, err := (translation.Translator{Root: root, ChunkWordBudget: 10}).PrepareTextChunks(series.Code, "solaris"); err != nil {
		t.Fatalf("PrepareTextChunks: %v", err)
	}
	fake := agent.NewFake(agent.Response{Result: "[[NODE 0]]\nПереклад.\n[[/NODE]]"})
	s, err := newServer(sessionFor(t, root), func(string, agent.Logger) (agent.Agent, error) { return fake, nil })
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	path := "/series/solaris/books/solaris/targets/uk/translate"
	if rec := postForm(s, path, url.Values{}); rec.Code != http.StatusOK {
		t.Fatalf("Start Translation response = %d", rec.Code)
	}
	outputPath := filepath.Join(root, "solaris", "books", "solaris", "translations", "en-to-uk", "out", "solaris.uk.txt")
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(outputPath); err == nil {
			break
		}
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("Start Translation did not produce output: %v", err)
	}
	if got := len(fake.RecordedRequests()); got != 1 {
		t.Errorf("Agent requests = %d, want one prepared Chunk", got)
	}
}

func TestDictionaryBuildingShowsTheActiveChunkAndRefreshesIt(t *testing.T) {
	body, err := execute("book-detail", bookDetailData{
		Series: library.Series{Code: "holmes", Name: "Holmes", SourceLanguage: "en"},
		Book:   library.Book{Code: "adventures", Targets: []library.TranslationTarget{{Language: "uk", Status: library.StatusAnalyzing}}},
		Progress: map[string]translation.DictionaryProgress{
			"uk": {Completed: 3, Active: 4, Total: 20},
		},
	})
	if err != nil {
		t.Fatalf("render Dictionary progress: %v", err)
	}
	for _, want := range []string{"Analyzing Chunk 4/20 (3 complete)", `hx-trigger="every 1s"`, `hx-swap="outerHTML"`} {
		if !strings.Contains(body, want) {
			t.Errorf("Dictionary progress does not render %q:\n%s", want, body)
		}
	}
}

func TestDictionaryProgressRefreshesOnlyItsOwnRegionUntilTheRunFinishes(t *testing.T) {
	root := t.TempDir()
	store := library.NewStore(root)
	series, _ := store.CreateSeries("Holmes", "en")
	_, _ = store.AddBook(series.Code, library.BookDraft{Code: "adventures"})
	if _, err := store.CreateTranslationTarget(series.Code, "adventures", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	if err := store.SetTranslationTargetStatus(series.Code, "adventures", "uk", library.StatusAnalyzing); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}
	runs := newDictionaryRuns()
	_, runID := runs.start(dictionaryKey(series.Code, "adventures", "uk"), translation.DictionaryProgress{Active: 1, Total: 3})
	runs.set(dictionaryKey(series.Code, "adventures", "uk"), runID, translation.DictionaryProgress{Completed: 1, Active: 2, Total: 3})
	s := screens{session: sessionFor(t, root), dictionaryRuns: runs}
	req := httptest.NewRequest(http.MethodGet, "/series/holmes/books/adventures/targets/uk/dictionary-progress", nil)
	req.SetPathValue("series", "holmes")
	req.SetPathValue("book", "adventures")
	req.SetPathValue("target", "uk")
	rec := httptest.NewRecorder()
	s.dictionaryProgress(rec, req)

	body := rec.Body.String()
	if !strings.HasPrefix(body, "<span") || !strings.Contains(body, "Analyzing Chunk 2/3 (1 complete)") {
		t.Fatalf("progress refresh = %s, want only the updated progress region", body)
	}

	if err := store.SetTranslationTargetStatus(series.Code, "adventures", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus(Dictionary Ready): %v", err)
	}
	terminal := httptest.NewRecorder()
	s.dictionaryProgress(terminal, req)
	if got := terminal.Header().Get("HX-Retarget"); got != "#book-detail" {
		t.Errorf("HX-Retarget = %q, want #book-detail", got)
	}
	if !strings.Contains(terminal.Body.String(), "Dictionary Ready") {
		t.Errorf("terminal refresh does not render Dictionary Ready: %s", terminal.Body.String())
	}
}

func TestDictionaryBuildingWritesAFencedAgentReplyAndReachesDictionaryReady(t *testing.T) {
	root := t.TempDir()
	store := library.NewStore(root)
	series, _ := store.CreateSeries("Holmes", "en")
	_, _ = store.AddBook(series.Code, library.BookDraft{Code: "adventures"})
	source := strings.Repeat("word ", translation.DefaultWordBudget) + "Holmes\n\n" + strings.Repeat("word ", translation.DefaultWordBudget) + "Holmes"
	if err := store.UploadSourceFile(series.Code, "adventures", "adventures.txt", strings.NewReader(source), false); err != nil {
		t.Fatalf("UploadSourceFile: %v", err)
	}
	if _, err := store.CreateTranslationTarget(series.Code, "adventures", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	fake := agent.NewFake(
		agent.Response{Result: "Holmes"},
		agent.Response{Result: "Holmes"},
		agent.Response{Result: "# English to Ukrainian Dictionary\n\noriginal\ttranslation\tnote\nHolmes\t\u0413\u043e\u043b\u043c\u0441\t"},
	)
	s, err := newServer(sessionFor(t, root), func(string, agent.Logger) (agent.Agent, error) { return fake, nil })
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	path := "/series/holmes/books/adventures/targets/uk/dictionary"
	postForm(s, path, url.Values{})
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		lib, err := store.Library()
		if err != nil {
			t.Fatalf("Library: %v", err)
		}
		if lib.Series[0].Books[0].Targets[0].Status == library.StatusDictionaryReady {
			break
		}
	}

	body := serve(s, request(s, "/series/holmes/books/adventures")).Body.String()
	if !strings.Contains(body, "Dictionary Ready") {
		t.Fatalf("Dictionary Building did not reach its terminal UI state: %s", body)
	}
	pathOnDisk := filepath.Join(root, "holmes", "books", "adventures", "translations", "en-to-uk", library.DictionaryFile)
	if got := read(t, pathOnDisk); !strings.Contains(got, "Holmes\t\u0413\u043e\u043b\u043c\u0441") {
		t.Errorf("dictionary.tsv = %q, want fenced agent reply to be persisted", got)
	}
}

func TestDictionaryReviewOffersPlainTSVAndWarnsWhenTheMergedDictionaryIsLarge(t *testing.T) {
	s, root := newEmptyLibraryServer(t)
	store := library.NewStore(root)
	series, _ := store.CreateSeries("Solaris", "pl")
	_, _ = store.AddBook(series.Code, library.BookDraft{Code: "solaris"})
	_, _ = store.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"})
	terms := make([]library.Term, 101)
	for index := range terms {
		terms[index] = library.Term{Original: fmt.Sprintf("term-%d", index), Translation: fmt.Sprintf("переклад-%d", index)}
	}
	if err := store.WriteDictionary(series.Code, "solaris", "uk", terms); err != nil {
		t.Fatalf("WriteDictionary: %v", err)
	}
	if err := store.SetTranslationTargetStatus(series.Code, "solaris", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}

	body := serve(s, request(s, "/series/solaris/books/solaris")).Body.String()
	for _, want := range []string{
		"Manage Dictionary", "Book Dictionary", "Series Dictionary", "101 Terms will bloat every translation request",
		`id="dictionary-modal-uk"`, `document.getElementById('dictionary-modal-uk').showModal()`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Dictionary review does not show %q:\n%s", want, body)
		}
	}
	book := serve(s, request(s, "/series/solaris/books/solaris/targets/uk/dictionary.tsv"))
	if got := book.Header().Get("Content-Type"); !strings.Contains(got, "text/tab-separated-values") {
		t.Errorf("book Dictionary Content-Type = %q", got)
	}
	if !strings.Contains(book.Body.String(), "term-100\tпереклад-100") {
		t.Errorf("book Dictionary TSV is missing a reviewed Term: %s", book.Body.String())
	}
	seriesTSV := serve(s, request(s, "/series/solaris/dictionaries/uk.tsv"))
	if got := seriesTSV.Code; got != http.StatusOK {
		t.Errorf("Series Dictionary status = %d, want %d", got, http.StatusOK)
	}
}

func TestDictionaryReviewCanEditSaveAndCancelTSVInsideTheModal(t *testing.T) {
	s, root := newEmptyLibraryServer(t)
	store := library.NewStore(root)
	series, _ := store.CreateSeries("Solaris", "pl")
	_, _ = store.AddBook(series.Code, library.BookDraft{Code: "solaris"})
	_, _ = store.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"})
	if err := store.WriteDictionary(series.Code, "solaris", "uk", []library.Term{{Original: "Ocean", Translation: "Океан"}}); err != nil {
		t.Fatalf("WriteDictionary: %v", err)
	}
	if err := store.SetTranslationTargetStatus(series.Code, "solaris", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}

	body := serve(s, request(s, "/series/solaris/books/solaris")).Body.String()
	for _, want := range []string{
		"Edit Dictionary",
		`id="dictionary-modal-content-uk"`,
		`hx-target="#dictionary-modal-content-uk"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Dictionary review is missing %q:\n%s", want, body)
		}
	}

	edited := serve(s, request(s, "/series/solaris/books/solaris/targets/uk/dictionary/edit"))
	if !strings.Contains(edited.Body.String(), `<textarea`) || !strings.Contains(edited.Body.String(), "Ocean\tОкеан") {
		t.Fatalf("edit mode did not show the current raw TSV: %s", edited.Body.String())
	}

	invalid := postForm(s, "/series/solaris/books/solaris/targets/uk/dictionary/edit", url.Values{
		"tsv": {"original\ttranslation\tnote\nOcean\n"},
	})
	if !strings.Contains(invalid.Body.String(), `role="alert"`) || !strings.Contains(invalid.Body.String(), "line 2") || !strings.Contains(invalid.Body.String(), `<textarea`) {
		t.Fatalf("invalid TSV did not stay in edit mode with an error: %s", invalid.Body.String())
	}
	terms, err := store.BookDictionary(series.Code, "solaris", "uk")
	if err != nil {
		t.Fatalf("BookDictionary after invalid edit: %v", err)
	}
	if len(terms) != 1 || terms[0].Translation != "Океан" {
		t.Fatalf("invalid edit changed the stored Dictionary: %#v", terms)
	}

	valid := postForm(s, "/series/solaris/books/solaris/targets/uk/dictionary/edit", url.Values{
		"tsv": {"original\ttranslation\tnote\nOcean\tморе\tmanual\nRiver\tрічка\t"},
	})
	if strings.Contains(valid.Body.String(), `<textarea`) || !strings.Contains(valid.Body.String(), "River") || !strings.Contains(valid.Body.String(), "Manage Dictionary") {
		t.Fatalf("valid TSV did not return to review mode inside the modal: %s", valid.Body.String())
	}
	terms, err = store.BookDictionary(series.Code, "solaris", "uk")
	if err != nil {
		t.Fatalf("BookDictionary after valid edit: %v", err)
	}
	if len(terms) != 2 || terms[0].Translation != "море" || terms[1].Original != "River" {
		t.Fatalf("valid edit stored %#v, want the edited Terms", terms)
	}

	editedAgain := serve(s, request(s, "/series/solaris/books/solaris/targets/uk/dictionary/edit"))
	if !strings.Contains(editedAgain.Body.String(), "River\tрічка") {
		t.Fatalf("edit mode did not reload the saved TSV: %s", editedAgain.Body.String())
	}
	cancelled := serve(s, request(s, "/series/solaris/books/solaris/targets/uk/dictionary/review"))
	if strings.Contains(cancelled.Body.String(), `<textarea`) || !strings.Contains(cancelled.Body.String(), "Manage Dictionary") {
		t.Fatalf("cancel/review did not return read-only content: %s", cancelled.Body.String())
	}
}

func TestPromotingATermFromDictionaryReviewWritesTheSeriesDictionary(t *testing.T) {
	s, root := newEmptyLibraryServer(t)
	store := library.NewStore(root)
	series, _ := store.CreateSeries("Solaris", "pl")
	_, _ = store.AddBook(series.Code, library.BookDraft{Code: "solaris"})
	_, _ = store.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"})
	if err := store.WriteDictionary(series.Code, "solaris", "uk", []library.Term{{Original: "Ocean", Translation: "Океан"}}); err != nil {
		t.Fatalf("WriteDictionary: %v", err)
	}
	if err := store.SetTranslationTargetStatus(series.Code, "solaris", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}
	beforePromotion := serve(s, request(s, "/series/solaris/books/solaris")).Body.String()
	if !strings.Contains(beforePromotion, "Promote to Series dict") {
		t.Errorf("unpromoted Term is not offered promotion:\n%s", beforePromotion)
	}

	rec := postForm(s, "/series/solaris/books/solaris/targets/uk/dictionary/promote", url.Values{"term": {"Ocean"}})
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "form-problem") {
		t.Fatalf("promoting Dictionary Term failed: %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Manage Dictionary") || strings.Contains(rec.Body.String(), `id="book-detail"`) {
		t.Fatalf("promoting a Term replaced or closed the Dictionary modal content: %s", rec.Body.String())
	}
	terms, err := store.SeriesDictionary(series.Code, "uk")
	if err != nil {
		t.Fatalf("SeriesDictionary: %v", err)
	}
	if len(terms) != 1 || terms[0].Original != "Ocean" {
		t.Errorf("SeriesDictionary = %#v, want promoted Ocean", terms)
	}
	body := serve(s, request(s, "/series/solaris/books/solaris")).Body.String()
	if !strings.Contains(body, "Unpromote from Series Dict") || !strings.Contains(body, "btn-warning") {
		t.Errorf("promoted Term is not highlighted for unpromotion:\n%s", body)
	}

	rec = postForm(s, "/series/solaris/books/solaris/targets/uk/dictionary/unpromote", url.Values{"term": {"Ocean"}})
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "form-problem") {
		t.Fatalf("unpromoting Dictionary Term failed: %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Manage Dictionary") || strings.Contains(rec.Body.String(), `id="book-detail"`) {
		t.Fatalf("unpromoting a Term replaced or closed the Dictionary modal content: %s", rec.Body.String())
	}
	terms, err = store.SeriesDictionary(series.Code, "uk")
	if err != nil {
		t.Fatalf("SeriesDictionary: %v", err)
	}
	if len(terms) != 0 {
		t.Errorf("SeriesDictionary = %#v, want no Ocean", terms)
	}
	bookTerms, err := store.BookDictionary(series.Code, "solaris", "uk")
	if err != nil || len(bookTerms) != 1 || bookTerms[0].Original != "Ocean" {
		t.Errorf("BookDictionary = %#v, %v; want retained Ocean", bookTerms, err)
	}
}

func TestStoppingDictionaryBuildingCancelsTheAgentAndRestoresNew(t *testing.T) {
	root := t.TempDir()
	store := library.NewStore(root)
	series, _ := store.CreateSeries("Solaris", "pl")
	_, _ = store.AddBook(series.Code, library.BookDraft{Code: "solaris"})
	if err := store.UploadSourceFile(series.Code, "solaris", "solaris.txt", strings.NewReader("Holmes"), false); err != nil {
		t.Fatalf("UploadSourceFile: %v", err)
	}
	if _, err := store.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	blocked := &cancellableAgent{started: make(chan struct{}, 1), cancelled: make(chan struct{}, 1)}
	s, err := newServer(sessionFor(t, root), func(string, agent.Logger) (agent.Agent, error) { return blocked, nil })
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	path := "/series/solaris/books/solaris/targets/uk/dictionary"
	postForm(s, path, url.Values{})
	select {
	case <-blocked.started:
	case <-time.After(time.Second):
		t.Fatal("Dictionary Building did not call its Agent")
	}

	stopped := postForm(s, path+"/stop", url.Values{})
	if !strings.Contains(stopped.Body.String(), "Start Dictionary Building") {
		t.Fatalf("stopping did not restore the start action: %s", stopped.Body.String())
	}
	select {
	case <-blocked.cancelled:
	case <-time.After(time.Second):
		t.Fatal("stopping did not cancel the Agent context")
	}
	lib, err := store.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if got := lib.Series[0].Books[0].Targets[0].Status; got != library.StatusNew {
		t.Errorf("Status = %q, want New", got)
	}
}

type cancellableAgent struct {
	started   chan struct{}
	cancelled chan struct{}
}

func (a *cancellableAgent) Call(ctx context.Context, _ agent.Request) (agent.Response, error) {
	a.started <- struct{}{}
	<-ctx.Done()
	a.cancelled <- struct{}{}
	return agent.Response{}, ctx.Err()
}

// The gap ticket 02 left for this one: a workspace that already has a
// workspace.toml opens cleanly however locked down it is, and only the first
// write finds out. This is where the first write lives.
func TestAWorkspaceThatCannotBeWrittenToIsReportedOnScreen(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, workspace.ConfigFile), []byte("agent = \"claude\"\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })

	s := newTestServerFor(t, sessionFor(t, root))
	if body := serve(s, request(s, "/")).Body.String(); !strings.Contains(body, "Add a Book") {
		t.Fatal("a read-only workspace did not open; the gap is elsewhere")
	}

	for path, values := range map[string]url.Values{
		"/series": {"name": {"Solaris"}, "language": {"pl"}},
		"/books":  {"series": {""}, "code": {"solaris"}, "language": {"pl"}},
	} {
		body := postForm(s, path, values).Body.String()
		if !strings.Contains(body, "form-problem") {
			t.Errorf("%s: a write that cannot happen was not reported:\n%s", path, body)
		}
	}
}

// A workspace can go away while the application is running — an unmounted
// drive, a folder renamed in Finder. Reading the library at every render is
// what turns that into something the user is told about.
func TestAWorkspaceThatDisappearsWhileRunningIsReported(t *testing.T) {
	root := filepath.Join(t.TempDir(), "library")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	s := newTestServerFor(t, sessionFor(t, root))
	if body := serve(s, request(s, "/")).Body.String(); !strings.Contains(body, "Add a Book") {
		t.Fatal("the library was not open to begin with")
	}

	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove: %v", err)
	}

	body := serve(s, request(s, "/")).Body.String()
	if !strings.Contains(body, "workspace-problem") {
		t.Errorf("a workspace that vanished mid-session is not reported:\n%s", body)
	}
	if !strings.Contains(body, "Choose folder") {
		t.Error("the picker was not offered again")
	}
}

// These routes write to the user's disk, so they are behind the same check as
// the folder picker.
func TestAuthoringCannotBeDrivenFromAnotherOrigin(t *testing.T) {
	s, root := newEmptyLibraryServer(t)

	for _, path := range []string{"/series", "/books"} {
		r := httptest.NewRequest(http.MethodPost, s.URL()+path, strings.NewReader("name=Solaris&language=Polish&code=solaris&title=Solaris"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "http://evil.example")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, r)

		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: got %d, want %d", path, rec.Code, http.StatusForbidden)
		}
	}
	// Only what opening the workspace put there; no Series, no Book.
	if left := entries(t, root); len(left) != 1 || left[0] != workspace.ConfigFile {
		t.Errorf("a foreign page wrote %v into the workspace", left)
	}
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	found, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var names []string
	for _, e := range found {
		names = append(names, e.Name())
	}
	return names
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
