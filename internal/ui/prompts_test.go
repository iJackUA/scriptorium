package ui

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/translation"
)

func deleteRequest(s *Server, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodDelete, s.URL()+path, nil)
	r.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

func TestPromptEditorsShowTheBuiltInPromptsUntilAFileIsSaved(t *testing.T) {
	root, _ := holmesWithModels(t, "")
	s := newTestServerFor(t, sessionFor(t, root))

	settings := html.UnescapeString(serve(s, request(s, "/")).Body.String())
	book := html.UnescapeString(serve(s, request(s, "/series/holmes/books/adventures")).Body.String())
	for _, p := range translation.Prompts() {
		if !strings.Contains(settings, p.BuiltIn()) {
			t.Errorf("Workspace settings do not show the built-in %s prompt", p)
		}
		if !strings.Contains(book, p.BuiltIn()) {
			t.Errorf("the Book does not show the built-in %s prompt", p)
		}
	}
	if !strings.Contains(settings, "Saving writes <code>"+translation.WorkspacePromptFile(translation.PromptTranslation)+"</code>") {
		t.Errorf("Workspace settings do not say where saving writes:\n%s", settings)
	}
	if !strings.Contains(settings, "Built-in</span>") {
		t.Errorf("Workspace settings do not badge the prompts as built-in")
	}
	if _, err := os.Stat(filepath.Join(root, library.PromptsDir)); !os.IsNotExist(err) {
		t.Errorf("rendering the editors created the prompts folder: %v", err)
	}
}

func TestSavingAWorkspacePromptWritesTheFileAndTheBookInheritsIt(t *testing.T) {
	root, _ := holmesWithModels(t, "")
	s := newTestServerFor(t, sessionFor(t, root))
	edited := "Only the names, please.\r\n{{TEXT_NODES}}"

	rec := postForm(s, "/settings/prompts/dictionary-extraction", url.Values{"template": {edited}})
	if body := rec.Body.String(); !strings.Contains(body, "Saved.") || !strings.Contains(body, "Customized") {
		t.Fatalf("save did not report success:\n%s", body)
	}
	file := filepath.Join(root, translation.WorkspacePromptFile(translation.PromptTermExtraction))
	if got := read(t, file); got != "Only the names, please.\n{{TEXT_NODES}}" {
		t.Errorf("saved prompt = %q", got)
	}

	book := html.UnescapeString(serve(s, request(s, "/series/holmes/books/adventures")).Body.String())
	if !strings.Contains(book, "Only the names, please.") || !strings.Contains(book, "the prompt from <code>"+translation.WorkspacePromptFile(translation.PromptTermExtraction)+"</code>") {
		t.Errorf("the Book does not show the inherited workspace prompt:\n%s", book)
	}
}

func TestABookPromptIsSavedInTheBookAndCanBeReverted(t *testing.T) {
	root, _ := holmesWithModels(t, "")
	s := newTestServerFor(t, sessionFor(t, root))
	edited := "Terms from {{SOURCE_LANGUAGE}} to {{TARGET_LANGUAGE}}:\n{{TERMS}}"

	postForm(s, "/series/holmes/books/adventures/prompts/dictionary-terms", url.Values{"template": {edited}})
	file := filepath.Join(root, translation.BookPromptFile("holmes", "adventures", translation.PromptTermTranslation))
	if got := read(t, file); got != edited {
		t.Fatalf("saved prompt = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, translation.WorkspacePromptFile(translation.PromptTermTranslation))); !os.IsNotExist(err) {
		t.Errorf("a Book prompt was also written to the workspace: %v", err)
	}

	rec := deleteRequest(s, "/series/holmes/books/adventures/prompts/dictionary-terms")
	if body := html.UnescapeString(rec.Body.String()); !strings.Contains(body, "Reverted.") || !strings.Contains(body, translation.PromptTermTranslation.BuiltIn()) {
		t.Fatalf("revert did not show the built-in prompt again:\n%s", body)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("revert left the Book prompt behind: %v", err)
	}
}

func TestAnUnchangedInheritedPromptIsNotWritten(t *testing.T) {
	root, _ := holmesWithModels(t, "")
	s := newTestServerFor(t, sessionFor(t, root))

	rec := postForm(s, "/settings/prompts/translation", url.Values{"template": {translation.DefaultPromptTemplate}})
	if body := rec.Body.String(); !strings.Contains(body, "nothing was saved") {
		t.Errorf("an unchanged save was not reported:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(root, translation.WorkspacePromptFile(translation.PromptTranslation))); !os.IsNotExist(err) {
		t.Errorf("an unchanged prompt was written: %v", err)
	}
}

func TestAPromptMissingASlotIsRejectedWithTheEditStillThere(t *testing.T) {
	root, _ := holmesWithModels(t, "")
	s := newTestServerFor(t, sessionFor(t, root))

	rec := postForm(s, "/series/holmes/books/adventures/prompts/translation", url.Values{"template": {"Just translate it."}})
	body := html.UnescapeString(rec.Body.String())
	if !strings.Contains(body, "{{DICTIONARY}}") || !strings.Contains(body, "Just translate it.") {
		t.Fatalf("the rejection did not name the slot and keep the edit:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(root, translation.BookPromptFile("holmes", "adventures", translation.PromptTranslation))); !os.IsNotExist(err) {
		t.Errorf("an invalid prompt was written: %v", err)
	}
}

func TestDictionaryBuildingSendsTheBooksPrompts(t *testing.T) {
	root, store := holmesWithModels(t, "")
	if err := translation.SavePrompt(root, translation.BookPromptFile("holmes", "adventures", translation.PromptTermExtraction), translation.PromptTermExtraction, "Book extraction:\n{{TEXT_NODES}}"); err != nil {
		t.Fatalf("SavePrompt: %v", err)
	}
	agents := &recordingAgents{fake: agent.NewFake(agent.Response{Result: "Holmes"}, agent.Response{Result: "Holmes\tГолмс\t"})}
	s, err := newServer(sessionFor(t, root), agents.build)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	postForm(s, "/series/holmes/books/adventures/targets/uk/dictionary", url.Values{})
	waitForStatus(t, store, library.StatusDictionaryReady)
	requests := agents.fake.RecordedRequests()
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	if !strings.HasPrefix(requests[0].Prompt, "Book extraction:\n[0]\nHolmes waited.") {
		t.Errorf("extraction prompt is not the Book's:\n%s", requests[0].Prompt)
	}
	if !strings.HasPrefix(requests[1].Prompt, "Render this complete Dictionary Term set") {
		t.Errorf("term translation prompt is not the built-in one:\n%s", requests[1].Prompt)
	}
}

func TestABrokenDictionaryPromptStopsDictionaryBuildingBeforeAnyRequest(t *testing.T) {
	root, store := holmesWithModels(t, "")
	file := translation.WorkspacePromptFile(translation.PromptTermTranslation)
	if err := os.MkdirAll(filepath.Join(root, library.PromptsDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, file), []byte("Translate {{TERMS}}."), 0o644); err != nil {
		t.Fatal(err)
	}
	agents := &recordingAgents{fake: agent.NewFake()}
	s, err := newServer(sessionFor(t, root), agents.build)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	body := html.UnescapeString(postForm(s, "/series/holmes/books/adventures/targets/uk/dictionary", url.Values{}).Body.String())
	if !strings.Contains(body, file+" is missing the required prompt slots {{SOURCE_LANGUAGE}}, {{TARGET_LANGUAGE}}") {
		t.Fatalf("Start Dictionary Building did not report the missing slots:\n%s", body)
	}
	lib, err := store.Library()
	if err != nil {
		t.Fatal(err)
	}
	if _, book, _ := lib.Book("holmes", "adventures"); book.Targets[0].Status != library.StatusNew {
		t.Errorf("Status = %s, want New", book.Targets[0].Status)
	}
	if got := len(agents.fake.RecordedRequests()); got != 0 {
		t.Errorf("Agent requests = %d, want none", got)
	}
}
