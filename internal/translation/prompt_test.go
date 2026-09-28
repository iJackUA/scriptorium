package translation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/agent"
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

func TestLoadPromptTemplateNamesEveryMissingSlot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "holmes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "holmes", library.PromptFile), []byte("Translate {{TEXT_NODES}} from {{SOURCE_LANGUAGE}}."), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPromptTemplate(root, "holmes")
	if err == nil {
		t.Fatal("LoadPromptTemplate accepted an incomplete override")
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
