package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCreatesAConfigWithDefaults(t *testing.T) {
	root := t.TempDir()

	ws, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if ws.Root != root {
		t.Errorf("Root = %q, want %q", ws.Root, root)
	}
	if ws.Config.Agent == "" {
		t.Error("no default Agent")
	}
	if ws.Config.Models.Mechanical == "" || ws.Config.Models.Translation == "" {
		t.Errorf("both Models want a default, got %+v", ws.Config.Models)
	}
	if ws.Config.DictionaryOccurrenceThreshold != 2 {
		t.Errorf("DictionaryOccurrenceThreshold = %d, want 2", ws.Config.DictionaryOccurrenceThreshold)
	}
	if len(ws.Config.Languages) != 0 {
		t.Errorf("Languages = %v, want an empty allowlist", ws.Config.Languages)
	}

	if _, err := os.Stat(filepath.Join(root, ConfigFile)); err != nil {
		t.Errorf("%s was not written: %v", ConfigFile, err)
	}
}

// The written file is the one the user hand-edits, so it has to be readable
// prose rather than a minimal encoding of the defaults.
func TestTheCreatedConfigCarriesItsOwnComments(t *testing.T) {
	root := t.TempDir()
	if _, err := Open(root); err != nil {
		t.Fatalf("Open: %v", err)
	}

	written := read(t, filepath.Join(root, ConfigFile))
	if !strings.Contains(written, "#") {
		t.Error("the created workspace.toml has no comments explaining it")
	}
}

// TOML was chosen over JSON precisely so hand-written comments survive, which
// they only do if opening a workspace never rewrites its config.
func TestOpeningAnExistingWorkspacePreservesItByteForByte(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ConfigFile)

	edited := "# my own note, please keep\nagent = \"claude\"\nlanguages = [\"de\"]\n\n[models]\nmechanical = \"cheap\"\ntranslation = \"good\"\n"
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	ws, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if got := read(t, path); got != edited {
		t.Errorf("workspace.toml was rewritten:\n%s", got)
	}
	if got := ws.Config.Languages; len(got) != 1 || got[0] != "de" {
		t.Errorf("Languages = %v, want [de]", got)
	}
	if ws.Config.Models.Translation != "good" {
		t.Errorf("Models.Translation = %q, want %q", ws.Config.Models.Translation, "good")
	}
}

func TestDictionaryOccurrenceThresholdCanBeConfigured(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ConfigFile)
	config := "agent = \"claude\"\nlanguages = []\ndictionary_occurrence_threshold = 5\n[models]\nmechanical = \"cheap\"\ntranslation = \"good\"\n"
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	ws, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := ws.Config.DictionaryOccurrenceThreshold; got != 5 {
		t.Errorf("DictionaryOccurrenceThreshold = %d, want 5", got)
	}
}

func TestTargetLanguagesCanBeUpdatedWithoutLosingOtherConfiguration(t *testing.T) {
	root := t.TempDir()
	ws, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	updated, err := ws.SetTargetLanguages([]string{"uk", "de"})
	if err != nil {
		t.Fatalf("SetTargetLanguages: %v", err)
	}
	if got := updated.Config.Languages; strings.Join(got, ",") != "uk,de" {
		t.Errorf("Languages = %v, want [uk de]", got)
	}
	written := read(t, filepath.Join(root, ConfigFile))
	for _, want := range []string{"languages = [\"uk\", \"de\"]", "agent = \"claude\"", "[models]"} {
		if !strings.Contains(written, want) {
			t.Errorf("workspace.toml does not preserve %q:\n%s", want, written)
		}
	}
}

func TestTargetLanguagesRejectUnknownTagsAndDuplicates(t *testing.T) {
	root := t.TempDir()
	ws, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	before := read(t, filepath.Join(root, ConfigFile))
	for _, languages := range [][]string{{"German"}, {"de", "de"}} {
		if _, err := ws.SetTargetLanguages(languages); err == nil {
			t.Errorf("SetTargetLanguages(%v) succeeded", languages)
		}
	}
	if got := read(t, filepath.Join(root, ConfigFile)); got != before {
		t.Error("a rejected target-language update rewrote workspace.toml")
	}
}

func TestOpenReportsAMissingRootDistinctly(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "moved-to-another-drive")

	_, err := Open(gone)
	if !errors.Is(err, ErrRootMissing) {
		t.Fatalf("Open: got %v, want ErrRootMissing", err)
	}
	if !strings.Contains(err.Error(), gone) {
		t.Errorf("error does not name the folder: %v", err)
	}
}

func TestOpenRejectsARootThatIsNotADirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := Open(file); err == nil {
		t.Fatal("Open accepted a file as a workspace root")
	}
}

func TestOpenReportsAnUnwritableRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "read-only")
	if err := os.Mkdir(root, 0o500); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })

	if _, err := Open(root); err == nil {
		t.Fatal("Open accepted a folder it cannot write to")
	}
}

func TestOpenReportsAnUnreadableConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ConfigFile), []byte("this is not = = toml"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	_, err := Open(root)
	if err == nil {
		t.Fatal("Open accepted a config it cannot parse")
	}
	if !strings.Contains(err.Error(), ConfigFile) {
		t.Errorf("error does not name the file at fault: %v", err)
	}
}

func TestOpenRejectsAnUnknownAgent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ConfigFile), []byte("agent = \"codex\"\n"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	_, err := Open(root)
	if err == nil {
		t.Fatal("Open accepted an unknown Agent")
	}
	if !strings.Contains(err.Error(), "codex") {
		t.Errorf("error = %v, want the configured Agent named", err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
