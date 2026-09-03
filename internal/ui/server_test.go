package ui

import (
	"html/template"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/workspace"
)

// newTestServer serves a session that already has a workspace open with a
// library in it, which is the state every screen but the welcome one is about.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	seedLibrary(t, root)
	return newTestServerFor(t, sessionFor(t, root))
}

// seedLibrary writes a library to root through the same service layer the
// handlers use, so the screens are driven by real files rather than by a
// fixture that could drift from what the store actually writes.
func seedLibrary(t *testing.T, root string) library.Store {
	t.Helper()
	store := library.NewStore(root)

	holmes, err := store.CreateSeries("The Adventures of Sherlock Holmes", "en")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	for _, book := range []library.BookDraft{
		{Code: "adventures", Title: "The Adventures of Sherlock Holmes", Author: "Arthur Conan Doyle"},
		{Code: "memoirs", Title: "The Memoirs of Sherlock Holmes", Author: "Arthur Conan Doyle"},
		{Code: "return", Title: "The Return of Sherlock Holmes", Author: "Arthur Conan Doyle"},
	} {
		if _, err := store.AddBook(holmes.Code, book); err != nil {
			t.Fatalf("AddBook %s: %v", book.Code, err)
		}
	}
	if _, _, err := store.AddStandaloneBook(library.BookDraft{Code: "solaris", Title: "Solaris", Author: "Stanisław Lem"}, "pl"); err != nil {
		t.Fatalf("AddStandaloneBook: %v", err)
	}
	return store
}

func newTestServerFor(t *testing.T, session *workspace.Session) *Server {
	t.Helper()
	s, err := NewServer(session)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// request builds a request addressed to the server the way the window
// addresses it, so that only the header under test varies.
func request(s *Server, path string) *http.Request {
	return httptest.NewRequest(http.MethodGet, s.URL()+path, nil)
}

func serve(s *Server, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

func TestServerBindsLoopbackOnAnEphemeralPort(t *testing.T) {
	first, second := newTestServer(t), newTestServer(t)

	for _, s := range []*Server{first, second} {
		addr := s.listener.Addr().(*net.TCPAddr)
		if !addr.IP.IsLoopback() {
			t.Errorf("bound to %s, want loopback", addr.IP)
		}
		if addr.Port == 0 {
			t.Error("no port assigned")
		}
	}

	if first.listener.Addr().String() == second.listener.Addr().String() {
		t.Error("two servers took the same address; the port is not ephemeral")
	}
}

func TestForeignOriginIsRejected(t *testing.T) {
	s := newTestServer(t)

	for _, origin := range []string{
		"http://evil.example",
		"https://evil.example",
		"http://127.0.0.1:9999",
		"null",
	} {
		req := request(s, "/")
		req.Header.Set("Origin", origin)

		if rec := serve(s, req); rec.Code != http.StatusForbidden {
			t.Errorf("origin %q: got %d, want %d", origin, rec.Code, http.StatusForbidden)
		}
	}
}

// A page whose DNS has been rebound to 127.0.0.1 is same-origin as far as the
// browser is concerned, so it sends no Origin header at all. The Host it was
// addressed to is what gives it away.
func TestForeignHostIsRejectedEvenWithNoOrigin(t *testing.T) {
	s := newTestServer(t)
	port := s.listener.Addr().(*net.TCPAddr).Port

	for _, host := range []string{
		"evil.example",
		"evil.example:" + strconv.Itoa(port),
		"127.0.0.1:9999",
	} {
		req := request(s, "/")
		req.Host = host

		if rec := serve(s, req); rec.Code != http.StatusForbidden {
			t.Errorf("host %q: got %d, want %d", host, rec.Code, http.StatusForbidden)
		}
	}
}

func TestOwnOriginAndOriginlessRequestsAreServed(t *testing.T) {
	s := newTestServer(t)

	for name, origin := range map[string]string{
		"own origin": s.URL(),
		"no origin":  "",
	} {
		req := request(s, "/")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}

		if rec := serve(s, req); rec.Code != http.StatusOK {
			t.Errorf("%s: got %d, want %d", name, rec.Code, http.StatusOK)
		}
	}
}

// The library on screen is the library on disk — every Series and Book here
// was written by the store, not by a fixture.
func TestLibraryPageListsWhatIsInTheWorkspace(t *testing.T) {
	s := newTestServer(t)

	rec := serve(s, request(s, "/"))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()

	for _, want := range []string{
		"The Adventures of Sherlock Holmes",
		"The Memoirs of Sherlock Holmes",
		"The Return of Sherlock Holmes",
		"Solaris",
		"Arthur Conan Doyle",
		"adventures", "memoirs", "return",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("library page is missing %q", want)
		}
	}
	if strings.Contains(body, "empty-library") {
		t.Error("a workspace with Books in it renders as empty")
	}
}

func TestBooksListRefreshRereadsTheWorkspace(t *testing.T) {
	root := t.TempDir()
	store := seedLibrary(t, root)
	if _, err := store.CreateTranslationTarget("the-adventures-of-sherlock-holmes", "memoirs", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	if err := store.SetTranslationTargetStatus("the-adventures-of-sherlock-holmes", "memoirs", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}
	s := newTestServerFor(t, sessionFor(t, root))

	req := request(s, "/books")
	req.Header.Set("HX-Request", "true")
	body := serve(s, req).Body.String()
	for _, want := range []string{`id="books-list"`, "Books:", "Refresh books list", "Ukrainian (uk): Dictionary Ready"} {
		if !strings.Contains(body, want) {
			t.Errorf("refreshed Books list is missing %q:\n%s", want, body)
		}
	}
}

func TestSelectingABookRereadsItsWorkspaceStateAndRefreshesTheBooksList(t *testing.T) {
	root := t.TempDir()
	store := seedLibrary(t, root)
	if _, err := store.CreateTranslationTarget("the-adventures-of-sherlock-holmes", "memoirs", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	if err := store.SetTranslationTargetStatus("the-adventures-of-sherlock-holmes", "memoirs", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}
	s := newTestServerFor(t, sessionFor(t, root))

	req := request(s, "/series/the-adventures-of-sherlock-holmes/books/memoirs")
	req.Header.Set("HX-Request", "true")
	body := serve(s, req).Body.String()
	for _, want := range []string{"Dictionary Ready", `id="books-list" hx-swap-oob="outerHTML"`} {
		if !strings.Contains(body, want) {
			t.Errorf("selecting a Book did not use workspace-backed state or refresh the list: missing %q:\n%s", want, body)
		}
	}
}

func TestSingleBookSeriesRendersWithoutAGroupHeader(t *testing.T) {
	s := newTestServer(t)

	body := serve(s, request(s, "/")).Body.String()

	// The fixture holds one multi-Book Series and one Series of a single Book;
	// only the former earns a group header.
	if got := strings.Count(body, "card-title"); got != 1 {
		t.Errorf("got %d group headers, want 1 (the one-Book Series should have none)", got)
	}
}

func TestBookDetailIsServedForHtmx(t *testing.T) {
	s := newTestServer(t)

	rec := serve(s, request(s, "/series/the-adventures-of-sherlock-holmes/books/memoirs"))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"The Memoirs of Sherlock Holmes", "Arthur Conan Doyle", "memoirs", "English", "en"} {
		if !strings.Contains(body, want) {
			t.Errorf("book detail is missing %q", want)
		}
	}

	missing := serve(s, request(s, "/series/the-adventures-of-sherlock-holmes/books/nope"))
	if missing.Code != http.StatusNotFound {
		t.Errorf("unknown book: got %d, want %d", missing.Code, http.StatusNotFound)
	}
}

func TestStaticAssetsAreServedFromTheBinary(t *testing.T) {
	s := newTestServer(t)

	for path, marker := range map[string]string{
		"/static/htmx.min.js": "htmx",
		"/static/app.css":     "badge-success",
	} {
		rec := serve(s, request(s, path))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: got %d, want %d", path, rec.Code, http.StatusOK)
		}
		if !strings.Contains(rec.Body.String(), marker) {
			t.Errorf("%s does not look like the real asset", path)
		}
	}
}

func TestLanguageControlsPositionModalDropdownsInTheDialogTopLayer(t *testing.T) {
	body, err := execute("layout", template.HTML(""))
	if err != nil {
		t.Fatalf("render layout: %v", err)
	}
	for _, want := range []string{"dropdownParent: dialog", "dropdown.style.position = 'fixed'", "dropdown.style.top = rect.bottom + 'px'"} {
		if !strings.Contains(body, want) {
			t.Errorf("language controls do not position a modal dropdown above the modal content: missing %q", want)
		}
	}
}

func TestStaticDirectoryIsNotListed(t *testing.T) {
	s := newTestServer(t)

	rec := serve(s, request(s, "/static/"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want %d", rec.Code, http.StatusNotFound)
	}
	if strings.Contains(rec.Body.String(), "htmx.min.js") {
		t.Error("the embedded assets are enumerable")
	}
}
