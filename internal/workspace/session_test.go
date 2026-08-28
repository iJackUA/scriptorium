package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/desktop"
	"github.com/ijackua/scriptorium/internal/desktop/desktoptest"
)

func newSession(t *testing.T, p desktop.FolderPicker) (*Session, *Settings) {
	t.Helper()
	settings := NewSettings(filepath.Join(t.TempDir(), "settings.toml"))
	return NewSession(p, settings), settings
}

func TestAFirstLaunchHasNoWorkspaceAndNothingToComplainAbout(t *testing.T) {
	session, _ := newSession(t, &desktoptest.Picker{})

	if _, ok := session.Current(); ok {
		t.Error("a fresh session claims to have a workspace open")
	}
	if problem := session.Problem(); problem != "" {
		t.Errorf("Problem() = %q, want nothing to report", problem)
	}
}

func TestASecondLaunchOpensTheRememberedWorkspace(t *testing.T) {
	root := t.TempDir()
	p := &desktoptest.Picker{}
	settings := NewSettings(filepath.Join(t.TempDir(), "settings.toml"))
	if err := settings.Remember(root); err != nil {
		t.Fatalf("Remember: %v", err)
	}

	session := NewSession(p, settings)

	ws, ok := session.Current()
	if !ok {
		t.Fatalf("no workspace opened; problem was %q", session.Problem())
	}
	if ws.Root != root {
		t.Errorf("Root = %q, want %q", ws.Root, root)
	}
	if p.Calls != 0 {
		t.Error("the picker was shown even though a workspace was remembered")
	}
}

func TestAVanishedWorkspaceIsReportedAndThePickerOfferedAgain(t *testing.T) {
	root := filepath.Join(t.TempDir(), "on-a-drive-that-is-gone")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	settings := NewSettings(filepath.Join(t.TempDir(), "settings.toml"))
	if err := settings.Remember(root); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove: %v", err)
	}

	session := NewSession(&desktoptest.Picker{}, settings)

	if _, ok := session.Current(); ok {
		t.Fatal("a workspace was opened from a folder that is not there")
	}
	problem := session.Problem()
	if !strings.Contains(problem, root) {
		t.Errorf("Problem() = %q; it should name the folder that is missing", problem)
	}
}

// The user did choose a folder once; they are owed a reason for being asked
// again, not a screen that looks like a first launch.
func TestSettingsThatCannotBeReadAreReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.toml")
	if err := os.WriteFile(path, []byte("not = = toml"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	session := NewSession(&desktoptest.Picker{}, NewSettings(path))

	if _, ok := session.Current(); ok {
		t.Fatal("a workspace was opened from settings that will not parse")
	}
	if session.Problem() == "" {
		t.Error("an unreadable settings file was passed off as a first launch")
	}
}

func TestChoosingAWorkspaceOpensAndRemembersIt(t *testing.T) {
	root := t.TempDir()
	p := &desktoptest.Picker{Answer: root}
	session, settings := newSession(t, p)

	session.Choose()

	ws, ok := session.Current()
	if !ok || ws.Root != root {
		t.Fatalf("Current() = %+v, %v; want the chosen folder", ws, ok)
	}
	if remembered, err := settings.Root(); err != nil || remembered != root {
		t.Errorf("remembered %q, %v; want %q", remembered, err, root)
	}
	if _, err := os.Stat(filepath.Join(root, ConfigFile)); err != nil {
		t.Errorf("%s was not created in the chosen folder: %v", ConfigFile, err)
	}
	if session.Problem() != "" {
		t.Errorf("Problem() = %q, want nothing to report", session.Problem())
	}
}

// Dismissing the picker is an answer, not a fault. The user knows what they
// did, so nothing changes and nothing is said back to them.
func TestCancellingThePickerChangesNothing(t *testing.T) {
	root := t.TempDir()
	p := &desktoptest.Picker{Answer: root}
	session, _ := newSession(t, p)
	session.Choose()

	p.Answer, p.Err = "", desktop.ErrCancelled
	session.Choose()

	ws, ok := session.Current()
	if !ok || ws.Root != root {
		t.Errorf("Current() = %+v, %v; the earlier workspace should still be open", ws, ok)
	}
	if session.Problem() != "" {
		t.Errorf("Problem() = %q; cancelling is not a problem", session.Problem())
	}
}

func TestAFolderThatCannotBeOpenedIsReportedAndLeavesTheSessionAlone(t *testing.T) {
	unwritable := filepath.Join(t.TempDir(), "read-only")
	if err := os.Mkdir(unwritable, 0o500); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(unwritable, 0o700) })
	session, settings := newSession(t, &desktoptest.Picker{Answer: unwritable})

	session.Choose()

	if _, ok := session.Current(); ok {
		t.Error("a workspace was opened despite the failure")
	}
	if session.Problem() == "" {
		t.Error("the failure was not reported to the user")
	}
	if remembered, err := settings.Root(); err != nil || remembered != "" {
		t.Errorf("remembered %q, %v; a folder that could not be opened should not be", remembered, err)
	}
}

func TestThePickerFailingIsReportedNotSwallowed(t *testing.T) {
	session, _ := newSession(t, &desktoptest.Picker{Err: errors.New("no window to attach to")})

	session.Choose()

	if !strings.Contains(session.Problem(), "no window to attach to") {
		t.Errorf("Problem() = %q, want the picker's own words", session.Problem())
	}
}
