package translation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/format"
	"github.com/ijackua/scriptorium/internal/library"
)

const archaicPrompt = `Render this author formally and archaically, from {{SOURCE_LANGUAGE}} into {{TARGET_LANGUAGE}}.
Terms: {{DICTIONARY}}
Reference: {{CONTINUITY_WINDOW}}
Protocol: {{NUMBERED_NODE_INSTRUCTIONS}}
Nodes: {{TEXT_NODES}}`

func promptTarget(t *testing.T, override string) (readyTargetFixture, *agent.Fake) {
	t.Helper()
	fixture := newReadyTarget(t, "Holmes", "en", "holmes", "uk", "holmes.txt", []byte("Watson arrived."))
	if err := fixture.store.WriteDictionary(fixture.series.Code, "holmes", "uk", []library.Term{{Original: "Watson", Translation: "Ватсон"}}); err != nil {
		t.Fatalf("WriteDictionary: %v", err)
	}
	if override != "" {
		if err := os.WriteFile(filepath.Join(fixture.root, fixture.series.Code, library.PromptFile), []byte(override), 0o644); err != nil {
			t.Fatalf("write prompt override: %v", err)
		}
	}
	if _, err := (Translator{Root: fixture.root}).PrepareTextChunks(fixture.series.Code, "holmes"); err != nil {
		t.Fatalf("PrepareTextChunks: %v", err)
	}
	return fixture, agent.NewFake(agent.Response{Result: "[[NODE 0]]\nВатсон прибув.\n[[/NODE]]"})
}

func TestASeriesPromptOverrideReplacesTheBuiltInPrompt(t *testing.T) {
	fixture, fake := promptTarget(t, archaicPrompt)

	if _, err := (Translator{Root: fixture.root, Agent: fake, TranslationModel: "strong"}).Translate(context.Background(), fixture.series.Code, "holmes", "uk"); err != nil {
		t.Fatalf("Translate: %v", err)
	}
	requests := fake.RecordedRequests()
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want one", len(requests))
	}
	prompt := requests[0].Prompt
	if !strings.HasPrefix(prompt, "Render this author formally and archaically") {
		t.Errorf("prompt does not start with the override:\n%s", prompt)
	}
	if strings.Contains(prompt, "NUMBERED-NODE RESPONSE PROTOCOL") {
		t.Errorf("the built-in prompt was appended to rather than replaced:\n%s", prompt)
	}
	for _, want := range []string{"English (en)", "Ukrainian (uk)", "Watson\tВатсон", "[[NODE 0]]\nWatson arrived.\n[[/NODE]]"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt does not carry %q:\n%s", want, prompt)
		}
	}
}

func TestWithoutAnOverrideTheBuiltInPromptIsUsed(t *testing.T) {
	fixture, fake := promptTarget(t, "")

	if _, err := (Translator{Root: fixture.root, Agent: fake, TranslationModel: "strong"}).Translate(context.Background(), fixture.series.Code, "holmes", "uk"); err != nil {
		t.Fatalf("Translate: %v", err)
	}
	prompt := fake.RecordedRequests()[0].Prompt
	if !strings.HasPrefix(prompt, "Translate the supplied Text Nodes from English (en) to Ukrainian (uk).") {
		t.Errorf("prompt is not the built-in one:\n%s", prompt)
	}
}

func TestAnOverrideMissingASlotFailsBeforeAnyRequest(t *testing.T) {
	fixture, fake := promptTarget(t, strings.Replace(archaicPrompt, "Terms: {{DICTIONARY}}\n", "", 1))

	_, err := (Translator{Root: fixture.root, Agent: fake, TranslationModel: "strong"}).Translate(context.Background(), fixture.series.Code, "holmes", "uk")
	if err == nil {
		t.Fatal("Translate accepted an override without the Dictionary")
	}
	if !strings.Contains(err.Error(), "{{DICTIONARY}}") || !strings.Contains(err.Error(), library.PromptFile) {
		t.Errorf("error = %v, want the missing slot and prompt.md named", err)
	}
	if got := len(fake.RecordedRequests()); got != 0 {
		t.Errorf("requests = %d, want none before the override is valid", got)
	}
}

func TestLoadPromptNamesEveryMissingSlot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "holmes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "holmes", library.PromptFile), []byte("Translate {{TEXT_NODES}} from {{SOURCE_LANGUAGE}}."), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPrompt(root, "holmes", "adventures", PromptTranslation)
	if err == nil {
		t.Fatal("LoadPrompt accepted an incomplete override")
	}
	for _, want := range []string{"{{TARGET_LANGUAGE}}", "{{DICTIONARY}}", "{{CONTINUITY_WINDOW}}", "{{NUMBERED_NODE_INSTRUCTIONS}}"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want %s named", err, want)
		}
	}
	for _, present := range []string{"{{TEXT_NODES}}", "{{SOURCE_LANGUAGE}}"} {
		if strings.Contains(err.Error(), present) {
			t.Errorf("error = %v, names %s, which is present", err, present)
		}
	}
}

func writePrompt(t *testing.T, root, rel, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTheMostSpecificPromptOverrideWins(t *testing.T) {
	root := t.TempDir()
	load := func() string {
		t.Helper()
		template, err := LoadPrompt(root, "holmes", "adventures", PromptTranslation)
		if err != nil {
			t.Fatalf("LoadPrompt: %v", err)
		}
		return template
	}
	if got := load(); got != DefaultPromptTemplate {
		t.Fatalf("with no override, got:\n%s", got)
	}
	workspacePrompt := "Workspace. " + archaicPrompt
	writePrompt(t, root, WorkspacePromptFile(PromptTranslation), workspacePrompt)
	if got := load(); got != workspacePrompt {
		t.Errorf("the workspace override was not used:\n%s", got)
	}
	seriesPrompt := "Series. " + archaicPrompt
	writePrompt(t, root, filepath.Join("holmes", library.PromptFile), seriesPrompt)
	if got := load(); got != seriesPrompt {
		t.Errorf("the Series override did not beat the workspace:\n%s", got)
	}
	bookPrompt := "Book. " + archaicPrompt
	writePrompt(t, root, BookPromptFile("holmes", "adventures", PromptTranslation), bookPrompt)
	if got := load(); got != bookPrompt {
		t.Errorf("the Book override did not beat the Series:\n%s", got)
	}
	other, err := LoadPrompt(root, "holmes", "memoirs", PromptTranslation)
	if err != nil {
		t.Fatalf("LoadPrompt: %v", err)
	}
	if other != seriesPrompt {
		t.Errorf("another Book of the Series got:\n%s", other)
	}
}

func TestDictionaryPromptsAreOverriddenByTheWorkspaceAndTheBook(t *testing.T) {
	root := t.TempDir()
	writePrompt(t, root, WorkspacePromptFile(PromptTermExtraction), "Workspace extraction {{TEXT_NODES}}")
	writePrompt(t, root, BookPromptFile("holmes", "adventures", PromptTermTranslation), "Book terms {{SOURCE_LANGUAGE}} {{TARGET_LANGUAGE}} {{TERMS}}")
	writePrompt(t, root, filepath.Join("holmes", library.PromptFile), "The Series prompt.md is only for translation.")

	for prompt, want := range map[Prompt]string{
		PromptTermExtraction:  "Workspace extraction {{TEXT_NODES}}",
		PromptTermTranslation: "Book terms {{SOURCE_LANGUAGE}} {{TARGET_LANGUAGE}} {{TERMS}}",
	} {
		got, err := LoadPrompt(root, "holmes", "adventures", prompt)
		if err != nil {
			t.Fatalf("LoadPrompt %s: %v", prompt, err)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", prompt, got, want)
		}
	}
	if got, _ := LoadPrompt(root, "holmes", "memoirs", PromptTermTranslation); got != PromptTermTranslation.BuiltIn() {
		t.Errorf("another Book got the Book override:\n%s", got)
	}
}

func TestSavingAPromptWithoutARequiredSlotWritesNothing(t *testing.T) {
	root := t.TempDir()
	file := BookPromptFile("holmes", "adventures", PromptTermTranslation)

	err := SavePrompt(root, file, PromptTermTranslation, "Translate {{TERMS}}.")
	if err == nil || !strings.Contains(err.Error(), "{{SOURCE_LANGUAGE}}") || !strings.Contains(err.Error(), file) {
		t.Fatalf("SavePrompt error = %v, want the missing slot and the file named", err)
	}
	if _, err := os.Stat(filepath.Join(root, file)); !os.IsNotExist(err) {
		t.Errorf("an invalid prompt was written: %v", err)
	}
}

func TestDictionaryBuildingSendsTheSuppliedTemplates(t *testing.T) {
	fake := agent.NewFake(agent.Response{Result: "Holmes"}, agent.Response{Result: "Holmes"}, agent.Response{Result: "Holmes\tГолмс\t"})

	_, err := DictionaryBuilder{
		Agent:                   fake,
		MechanicalModel:         "cheap",
		ChunkWordBudget:         2,
		ExtractionTemplate:      "Names only, please:\n{{TEXT_NODES}}",
		TermTranslationTemplate: "Terms {{SOURCE_LANGUAGE}}>{{TARGET_LANGUAGE}}:\n{{TERMS}}",
	}.Build(context.Background(), []format.TextNode{{Index: 0, Text: "Holmes spoke."}, {Index: 1, Text: "Holmes left."}}, "en", "uk", nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	requests := fake.RecordedRequests()
	if len(requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(requests))
	}
	if got, want := requests[0].Prompt, "Names only, please:\n[0]\nHolmes spoke.\n\n"; got != want {
		t.Errorf("extraction prompt = %q, want %q", got, want)
	}
	if got, want := requests[2].Prompt, "Terms en>uk:\nHolmes"; got != want {
		t.Errorf("term translation prompt = %q, want %q", got, want)
	}
}
