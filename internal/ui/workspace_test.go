package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/desktop"
	"github.com/ijackua/scriptorium/internal/desktop/desktoptest"
	"github.com/ijackua/scriptorium/internal/workspace"
)

func newSettings(t *testing.T) *workspace.Settings {
	t.Helper()
	return workspace.NewSettings(filepath.Join(t.TempDir(), "settings.toml"))
}

// openSession is a session with a workspace already open, as a second launch
// would find it.
func openSession(t *testing.T) *workspace.Session {
	t.Helper()
	settings := newSettings(t)
	if err := settings.Remember(t.TempDir()); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	return workspace.NewSession(&desktoptest.Picker{}, settings)
}

// post makes the request the way htmx makes it, since that is the only way the
// interface reaches this route.
func post(s *Server, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, s.URL()+path, nil)
	r.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

func TestWithNoWorkspaceTheRootAsksForOne(t *testing.T) {
	s := newTestServerFor(t, workspace.NewSession(&desktoptest.Picker{}, newSettings(t)))

	rec := serve(s, request(s, "/"))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "Choose a workspace folder") {
		t.Error("the root does not offer the folder picker")
	}
	if strings.Contains(body, "Sherlock") {
		t.Error("the library is on screen before a workspace has been chosen")
	}
	// It is a whole page rather than a fragment, because nothing is on screen
	// for a fragment to be swapped into.
	if !strings.Contains(body, "<!doctype html>") {
		t.Error("the welcome screen was served without its layout")
	}
}

func TestChoosingAFolderSwapsInTheLibrary(t *testing.T) {
	root := t.TempDir()
	settings := newSettings(t)
	p := &desktoptest.Picker{Answer: root}
	s := newTestServerFor(t, workspace.NewSession(p, settings))

	rec := post(s, "/workspace")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}
	if p.Calls != 1 {
		t.Errorf("the picker was shown %d times, want once", p.Calls)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Sherlock") {
		t.Error("the library did not replace the welcome screen")
	}
	// The response is swapped into a page that already has a layout, so it
	// must not carry one of its own.
	if strings.Contains(body, "<!doctype html>") {
		t.Error("the swap fragment carries a whole page")
	}

	if remembered, err := settings.Root(); err != nil || remembered != root {
		t.Errorf("remembered %q, %v; want %q", remembered, err, root)
	}
	if _, err := os.Stat(filepath.Join(root, workspace.ConfigFile)); err != nil {
		t.Errorf("%s was not created: %v", workspace.ConfigFile, err)
	}

	// And a relaunch against the same settings goes straight to the library.
	next := newTestServerFor(t, workspace.NewSession(&desktoptest.Picker{}, settings))
	if !strings.Contains(serve(next, request(next, "/")).Body.String(), "Sherlock") {
		t.Error("a second launch did not open the remembered workspace")
	}
}

func TestDismissingThePickerLeavesTheWelcomeScreenAlone(t *testing.T) {
	s := newTestServerFor(t, workspace.NewSession(&desktoptest.Picker{Err: desktop.ErrCancelled}, newSettings(t)))

	body := post(s, "/workspace").Body.String()

	if !strings.Contains(body, "Choose a workspace folder") {
		t.Error("cancelling the picker left the welcome screen behind")
	}
	if strings.Contains(body, "workspace-problem") {
		t.Error("cancelling the picker was reported as a problem")
	}
}

func TestAFolderThatCannotBeOpenedIsReportedOnScreen(t *testing.T) {
	unwritable := filepath.Join(t.TempDir(), "read-only")
	if err := os.Mkdir(unwritable, 0o500); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(unwritable, 0o700) })
	s := newTestServerFor(t, workspace.NewSession(&desktoptest.Picker{Answer: unwritable}, newSettings(t)))

	body := post(s, "/workspace").Body.String()

	if !strings.Contains(body, "workspace-problem") {
		t.Error("the failure was not reported on screen")
	}
	if !strings.Contains(body, "Choose folder") {
		t.Error("the picker was not offered again")
	}
}

func TestAVanishedWorkspaceIsReportedAndThePickerOfferedAgain(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "on-a-drive-that-is-gone")
	settings := newSettings(t)
	if err := os.Mkdir(gone, 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := settings.Remember(gone); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	s := newTestServerFor(t, workspace.NewSession(&desktoptest.Picker{}, settings))

	body := serve(s, request(s, "/")).Body.String()

	if !strings.Contains(body, gone) {
		t.Errorf("the missing folder is not named on screen:\n%s", body)
	}
	if !strings.Contains(body, "Choose folder") {
		t.Error("the picker was not offered again")
	}
}

// The picker is a hole in an HTTP server on the user's machine, so it is
// behind the same checks as everything else.
func TestThePickerCannotBeTriggeredFromAnotherOrigin(t *testing.T) {
	p := &desktoptest.Picker{Answer: t.TempDir()}
	s := newTestServerFor(t, workspace.NewSession(p, newSettings(t)))

	r := httptest.NewRequest(http.MethodPost, s.URL()+"/workspace", nil)
	r.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)

	if rec.Code != http.StatusForbidden {
		t.Errorf("got %d, want %d", rec.Code, http.StatusForbidden)
	}
	if p.Calls != 0 {
		t.Error("a foreign page opened a folder chooser on the user's desktop")
	}
}

// The button posts through htmx, but the route is a plain URL and nothing
// stops something else reaching it. What comes back should be readable either
// way rather than a fragment with no page around it.
func TestAPlainPostGetsAWholePageBack(t *testing.T) {
	s := newTestServerFor(t, workspace.NewSession(&desktoptest.Picker{Answer: t.TempDir()}, newSettings(t)))

	r := httptest.NewRequest(http.MethodPost, s.URL()+"/workspace", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Error("a request that cannot swap a fragment was sent one anyway")
	}
}
