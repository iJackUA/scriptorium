package ui

import (
	"errors"
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
	config := "languages = [\"uk\"]\ndictionary_occurrence_threshold = 1\n\n[mechanical]\nmodel = \"cheap-model\"\neffort = \"medium\"\n\n[translation]\nmodel = \"strong-model\"\n"
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
	root, store := holmesWithModels(t, "")
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

	// The Book overrides only its translation Model and Effort, through the
	// pickers on its page; everything else is inherited.
	picked := url.Values{"translation_model": {"claude-fable-5-1"}, "translation_effort": {"xhigh"}}
	if body := postForm(s, "/series/holmes/books/adventures/choices", picked).Body.String(); !strings.Contains(body, "Saved.") {
		t.Fatalf("saving the Book's pickers did not confirm:\n%s", body)
	}

	postForm(s, "/series/holmes/books/adventures/targets/uk/dictionary", url.Values{})
	waitForStatus(t, store, library.StatusDictionaryReady)
	mechanical := agents.fake.RecordedRequests()
	if len(mechanical) == 0 {
		t.Fatal("Dictionary Building made no Agent requests")
	}
	for i, request := range mechanical {
		if request.Model != "cheap-model" || request.Effort != agent.EffortMedium {
			t.Errorf("Dictionary Building request %d = %s at %s, want the workspace's mechanical Model and Effort", i, request.Model, request.Effort)
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
	if got := requests[len(requests)-1]; got.Model != "claude-fable-5-1" || got.Effort != agent.EffortXHigh {
		t.Errorf("translation request = %s at %s, want the Book's override", got.Model, got.Effort)
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

func TestTheWorkspacePickersSaveOnChange(t *testing.T) {
	root, _ := holmesWithModels(t, "")
	s := newTestServerFor(t, sessionFor(t, root))

	values := url.Values{
		"mechanical_agent": {"claude"}, "mechanical_model": {"claude-sonnet-5"}, "mechanical_effort": {"high"},
		"translation_agent": {"claude"}, "translation_model": {"strong-model"}, "translation_effort": {"medium"},
	}
	body := postForm(s, "/settings/choices", values).Body.String()
	if !strings.Contains(body, "Saved.") || !strings.Contains(body, `<option value="claude-sonnet-5" selected>`) {
		t.Fatalf("the saved pickers were not rendered back:\n%s", body)
	}
	ws, err := workspace.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if want := (workspace.Choice{Agent: "claude", Model: "claude-sonnet-5", Effort: agent.EffortHigh}); ws.Config.Mechanical != want {
		t.Errorf("mechanical on disk = %+v, want %+v", ws.Config.Mechanical, want)
	}
	if ws.Config.Translation.Model != "strong-model" {
		t.Errorf("translation Model = %q, want the untouched hand-written one", ws.Config.Translation.Model)
	}
	if page := serve(s, request(s, "/")).Body.String(); !strings.Contains(page, `<option value="claude-sonnet-5" selected>`) {
		t.Error("the settings screen does not show the saved Model")
	}
}

func TestARejectedPickerIsReportedAndNotSaved(t *testing.T) {
	root, _ := holmesWithModels(t, "")
	s := newTestServerFor(t, sessionFor(t, root))
	before := read(t, filepath.Join(root, workspace.ConfigFile))

	values := url.Values{
		"mechanical_agent": {"claude"}, "mechanical_model": {"cheap-model"}, "mechanical_effort": {"extreme"},
		"translation_agent": {"claude"}, "translation_model": {"strong-model"}, "translation_effort": {"medium"},
	}
	if body := postForm(s, "/settings/choices", values).Body.String(); !strings.Contains(body, "form-problem") || !strings.Contains(body, "extreme") {
		t.Fatalf("an unknown Effort was not reported:\n%s", body)
	}
	if read(t, filepath.Join(root, workspace.ConfigFile)) != before {
		t.Error("a rejected picker rewrote workspace.toml")
	}
}

func TestTheBookPickersOfferInheritingAndCanReturnToIt(t *testing.T) {
	root, _ := holmesWithModels(t, "\n[translation]\nmodel = \"claude-fable-5-1\"\n")
	s := newTestServerFor(t, sessionFor(t, root))

	page := serve(s, request(s, "/series/holmes/books/adventures")).Body.String()
	for _, want := range []string{`<option value="" selected>Inherit (cheap-model)</option>`, `<option value="claude-fable-5-1" selected>`, `Inherit (medium)`} {
		if !strings.Contains(page, want) {
			t.Errorf("Book page does not offer %q", want)
		}
	}

	body := postForm(s, "/series/holmes/books/adventures/choices", url.Values{"translation_model": {""}}).Body.String()
	if !strings.Contains(body, "Saved.") {
		t.Fatalf("returning to inherit did not save:\n%s", body)
	}
	lib, err := library.NewStore(root).Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if got := lib.Series[0].Books[0].Override; got != (workspace.Choices{}) {
		t.Errorf("Override = %+v, want the Book inheriting everything again", got)
	}
}

func TestThePickersOfferTheModelsTheAgentLists(t *testing.T) {
	root, _ := holmesWithModels(t, "")
	fake := agent.NewFake()
	fake.Models = []agent.ModelInfo{{Name: "claude-opus-5-5", Label: "Opus 5.5"}, {Name: "claude-haiku-4-5-20251001", Label: "Haiku 4.5"}}
	s, err := newServer(sessionFor(t, root), func(string, agent.Logger) (agent.Agent, error) { return fake, nil })
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	page := serve(s, request(s, "/series/holmes/books/adventures")).Body.String()
	for _, want := range []string{`<option value="claude-opus-5-5" >Opus 5.5 (claude-opus-5-5)</option>`, `>Haiku 4.5 (claude-haiku-4-5-20251001)</option>`} {
		if !strings.Contains(page, want) {
			t.Errorf("Book page does not offer %s:\n%s", want, page)
		}
	}
	if strings.Contains(page, `data-testid="models-unlisted"`) {
		t.Error("a listed catalog was reported as unlisted")
	}
}

// Listing starts the Agent's CLI, and the pickers render on every screen.
func TestTheModelListIsAskedForOnceAndReused(t *testing.T) {
	var asked int
	catalogs := newModelCatalogs(func(string, agent.Logger) (agent.Agent, error) {
		asked++
		fake := agent.NewFake()
		fake.Models = []agent.ModelInfo{{Name: "m", Label: "M"}}
		return fake, nil
	})
	now := time.Now()
	catalogs.now = func() time.Time { return now }

	for range 3 {
		if models, err := catalogs.models("claude"); err != nil || len(models) != 1 {
			t.Fatalf("models = %+v, %v", models, err)
		}
	}
	if asked != 1 {
		t.Errorf("the Agent was asked %d times, want once", asked)
	}
	now = now.Add(catalogTTL + time.Second)
	if _, _ = catalogs.models("claude"); asked != 2 {
		t.Errorf("an expired list was not asked for again (asked %d times)", asked)
	}
}

func TestAnAgentThatCannotListItsModelsIsSaidSo(t *testing.T) {
	root, _ := holmesWithModels(t, "")
	fake := agent.NewFake()
	fake.ModelsErr = errors.New("claude: not logged in")
	s, err := newServer(sessionFor(t, root), func(string, agent.Logger) (agent.Agent, error) { return fake, nil })
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	page := serve(s, request(s, "/")).Body.String()
	if !strings.Contains(page, "not logged in") || !strings.Contains(page, `<option value="strong-model" selected>strong-model</option>`) {
		t.Errorf("the settings pickers do not explain the missing list or keep the configured Model:\n%s", page)
	}
}
