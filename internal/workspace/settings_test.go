package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestARememberedRootSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.toml")

	if err := NewSettings(path).Remember("/Volumes/Books/scriptorium"); err != nil {
		t.Fatalf("Remember: %v", err)
	}

	// A second Settings over the same file stands in for the next launch.
	root, err := NewSettings(path).Root()
	if err != nil || root != "/Volumes/Books/scriptorium" {
		t.Errorf("Root() = %q, %v; want the remembered path", root, err)
	}
}

func TestRememberCreatesTheDirectoryItNeeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Scriptorium", "settings.toml")

	if err := NewSettings(path).Remember("/tmp/books"); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("settings file was not written: %v", err)
	}
}

// A first launch has no settings file, and a launch after the user emptied it
// has one that says nothing. Both mean "ask the user", and neither is a fault
// worth refusing to start over.
func TestNothingRememberedIsNotAFailure(t *testing.T) {
	dir := t.TempDir()

	absent := filepath.Join(dir, "absent.toml")
	if root, err := NewSettings(absent).Root(); root != "" || err != nil {
		t.Errorf("absent file: got %q, %v; want nothing remembered", root, err)
	}

	blank := filepath.Join(dir, "blank.toml")
	if err := os.WriteFile(blank, []byte("workspace = \"\"\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if root, err := NewSettings(blank).Root(); root != "" || err != nil {
		t.Errorf("blank path: got %q, %v; want nothing remembered", root, err)
	}
}

// A settings file that exists but will not parse is a different thing from no
// settings file: the user did choose a folder, and is owed a reason for being
// asked again rather than a screen that pretends they never did.
func TestAnUnreadableSettingsFileIsAFailureWorthReporting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garbage.toml")
	if err := os.WriteFile(path, []byte("not = = toml"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	root, err := NewSettings(path).Root()
	if err == nil {
		t.Fatal("an unparseable settings file was reported as nothing remembered")
	}
	if root != "" {
		t.Errorf("Root() = %q, want nothing alongside the error", root)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error does not name the file at fault: %v", err)
	}
}

func TestUserSettingsLiveOutsideAnyWorkspace(t *testing.T) {
	path, err := UserSettingsPath()
	if err != nil {
		t.Fatalf("UserSettingsPath: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("UserSettingsPath() = %q, want an absolute path", path)
	}
}

// Folder names are the user's, not ours, so the settings file has to survive
// whatever punctuation is in one.
func TestAnAwkwardFolderNameRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.toml")
	awkward := `/Users/me/He said "books"\and things`

	if err := NewSettings(path).Remember(awkward); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if root, err := NewSettings(path).Root(); err != nil || root != awkward {
		t.Errorf("Root() = %q, %v; want %q", root, err, awkward)
	}
}
