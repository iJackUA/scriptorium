package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// newEmptyLibraryServer serves an opened workspace with nothing in it, which is
// what a user sees the moment after choosing a folder.
func newEmptyLibraryServer(t *testing.T) (*Server, string) {
	t.Helper()
	root := t.TempDir()
	return newTestServerFor(t, sessionFor(t, root)), root
}

func TestCreatingASeriesShowsItInTheLibrary(t *testing.T) {
	s, root := newEmptyLibraryServer(t)

	rec := postForm(s, "/series", url.Values{"name": {"Solaris"}, "language": {"Polish"}})
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
	if rec := postForm(s, "/series", url.Values{"name": {"Solaris"}, "language": {"Polish"}}); rec.Code != http.StatusOK {
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
		"series": {""}, "code": {"solaris"}, "title": {"Solaris"}, "author": {"Stanisław Lem"}, "language": {"Polish"},
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
		// Retyping the whole form to fix one field is what the round trip
		// through the server threatens, so the values come back with it.
		for _, want := range []string{values.Get("code"), "Something Else", "Someone"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: the form came back without %q", name, want)
			}
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

	rec := postForm(s, "/books", url.Values{"series": {""}, "code": {"solaris"}, "language": {"Polish"}})
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
		"/series": {"name": {"Solaris"}, "language": {"Polish"}},
		"/books":  {"series": {""}, "code": {"solaris"}, "language": {"Polish"}},
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
