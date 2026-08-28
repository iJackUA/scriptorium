package ui

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/workspace"
)

// newTestServer serves a session that already has a workspace open, which is
// the state every screen but the welcome one is about.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	return newTestServerFor(t, openSession(t))
}

func newTestServerFor(t *testing.T, session *workspace.Session) *Server {
	t.Helper()
	s, err := NewServer(library.Fixture(), session)
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

func TestLibraryPageListsSeriesBooksAndStatuses(t *testing.T) {
	s := newTestServer(t)

	rec := serve(s, request(s, "/"))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()

	for _, want := range []string{
		"The Adventures of Sherlock Holmes",
		"The Memoirs of Sherlock Holmes",
		"Solaris",
		"New", "Analyzing", "Dictionary Ready", "Translating", "Translated", "Failed",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("library page is missing %q", want)
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

	rec := serve(s, request(s, "/series/holmes/books/memoirs"))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"The Memoirs of Sherlock Holmes", "Ukrainian", "Dictionary Ready"} {
		if !strings.Contains(body, want) {
			t.Errorf("book detail is missing %q", want)
		}
	}

	missing := serve(s, request(s, "/series/holmes/books/nope"))
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
