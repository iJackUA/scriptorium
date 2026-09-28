package ui

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/translation"
	"github.com/ijackua/scriptorium/internal/workspace"
)

// recordingAgents hands out one fake and remembers which Agent each piece of
// work asked for.
type recordingAgents struct {
	fake  *agent.Fake
	mu    sync.Mutex
	names []string
}

func (a *recordingAgents) build(name string, _ agent.Logger) (agent.Agent, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.names = append(a.names, name)
	return a.fake, nil
}

func holmesWithModels(t *testing.T, bookOverride string) (string, library.Store) {
	t.Helper()
	root := t.TempDir()
	config := "agent = \"claude\"\nlanguages = [\"uk\"]\ndictionary_occurrence_threshold = 1\n\n[agents.claude]\nmechanical = \"cheap-model\"\ntranslation = \"strong-model\"\n"
	if err := os.WriteFile(filepath.Join(root, workspace.ConfigFile), []byte(config), 0o644); err != nil {
		t.Fatalf("write workspace.toml: %v", err)
	}
	store := library.NewStore(root)
	series, err := store.CreateSeries("Holmes", "en")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if _, err := store.AddBook(series.Code, library.BookDraft{Code: "adventures"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	bookPath := filepath.Join(root, series.Code, library.BooksDir, "adventures", library.BookFile)
	if err := os.WriteFile(bookPath, []byte(read(t, bookPath)+bookOverride), 0o644); err != nil {
		t.Fatalf("edit book.toml: %v", err)
	}
	if err := store.UploadSourceFile(series.Code, "adventures", "adventures.txt", strings.NewReader("Holmes waited."), false); err != nil {
		t.Fatalf("UploadSourceFile: %v", err)
	}
	if _, err := store.CreateTranslationTarget(series.Code, "adventures", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	return root, store
}

func waitForStatus(t *testing.T, store library.Store, want library.Status) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		lib, err := store.Library()
		if err != nil {
			t.Fatalf("Library: %v", err)
		}
		if lib.Series[0].Books[0].Targets[0].Status == want {
			return
		}
	}
	t.Fatalf("Translation Target never reached %s", want)
}

func TestMechanicalWorkUsesTheCheapModelAndTranslationTheBooksStrongOne(t *testing.T) {
	root, store := holmesWithModels(t, "\n[models]\ntranslation = \"book-model\"\n")
	agents := &recordingAgents{fake: agent.NewFake(
		agent.Response{Result: "Holmes"},
		agent.Response{Result: "original\ttranslation\tnote\nHolmes\tГолмс\t"},
		agent.Response{Result: "[[NODE 0]]\nГолмс чекав.\n[[/NODE]]"},
	)}
	s, err := newServer(sessionFor(t, root), agents.build)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	postForm(s, "/series/holmes/books/adventures/targets/uk/dictionary", url.Values{})
	waitForStatus(t, store, library.StatusDictionaryReady)
	mechanical := agents.fake.RecordedRequests()
	if len(mechanical) == 0 {
		t.Fatal("Dictionary Building made no Agent requests")
	}
	for i, request := range mechanical {
		if request.Model != "cheap-model" {
			t.Errorf("Dictionary Building request %d Model = %q, want the workspace's mechanical Model", i, request.Model)
		}
	}

	if _, err := (translation.Translator{Root: root}).PrepareTextChunks("holmes", "adventures"); err != nil {
		t.Fatalf("PrepareTextChunks: %v", err)
	}
	if rec := postForm(s, "/series/holmes/books/adventures/targets/uk/translate", url.Values{}); rec.Code != http.StatusOK {
		t.Fatalf("Start Translation response = %d", rec.Code)
	}
	waitForStatus(t, store, library.StatusTranslated)
	requests := agents.fake.RecordedRequests()
	if len(requests) != len(mechanical)+1 {
		t.Fatalf("Agent requests = %d, want one translation request after Dictionary Building", len(requests))
	}
	if got := requests[len(requests)-1].Model; got != "book-model" {
		t.Errorf("translation Model = %q, want the Book's override", got)
	}
	for _, name := range agents.names {
		if name != agent.ClaudeName {
			t.Errorf("Agent asked for = %q, want claude", name)
		}
	}
}

func TestABrokenPromptOverrideStopsTranslationBeforeAnyRequest(t *testing.T) {
	root, store := holmesWithModels(t, "")
	if err := store.SetTranslationTargetStatus("holmes", "adventures", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}
	if _, err := (translation.Translator{Root: root}).PrepareTextChunks("holmes", "adventures"); err != nil {
		t.Fatalf("PrepareTextChunks: %v", err)
	}
	prompt := "Translate {{TEXT_NODES}} from {{SOURCE_LANGUAGE}} to {{TARGET_LANGUAGE}}.\n{{CONTINUITY_WINDOW}}\n{{NUMBERED_NODE_INSTRUCTIONS}}\n"
	if err := os.WriteFile(filepath.Join(root, "holmes", library.PromptFile), []byte(prompt), 0o644); err != nil {
		t.Fatalf("write prompt.md: %v", err)
	}
	agents := &recordingAgents{fake: agent.NewFake()}
	s, err := newServer(sessionFor(t, root), agents.build)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	rec := postForm(s, "/series/holmes/books/adventures/targets/uk/translate", url.Values{})
	if body := rec.Body.String(); !strings.Contains(body, "{{DICTIONARY}}") || !strings.Contains(body, "prompt.md") {
		t.Fatalf("Start Translation did not report the missing slot:\n%s", body)
	}
	if got := len(agents.fake.RecordedRequests()); got != 0 {
		t.Errorf("Agent requests = %d, want none", got)
	}
}
