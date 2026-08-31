package translation

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/format"
)

func TestDictionaryBuildingExtractsRecurringTermsThenTranslatesThemTogether(t *testing.T) {
	fake := agent.NewFake(
		agent.Response{Result: "Holmes\nWatson\nLondon"},
		agent.Response{Result: "Holmes\nLondon\nBaker Street"},
		agent.Response{Result: "Holmes\t\u0413\u043e\u043b\u043c\u0441\t\nLondon\t\u041b\u043e\u043d\u0434\u043e\u043d\t"},
	)
	progress := []DictionaryProgress{}

	terms, err := DictionaryBuilder{
		Agent:               fake,
		MechanicalModel:     "cheap",
		OccurrenceThreshold: 2,
		ChunkWordBudget:     4,
	}.Build(context.Background(), []format.TextNode{
		{Index: 0, Chapter: 0, Text: "Holmes met Watson in London."},
		{Index: 1, Chapter: 0, Text: "Holmes returned to London."},
	}, "en", "uk", func(update DictionaryProgress) { progress = append(progress, update) })
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got, want := terms, []Term{{Original: "Holmes", Translation: "\u0413\u043e\u043b\u043c\u0441"}, {Original: "London", Translation: "\u041b\u043e\u043d\u0434\u043e\u043d"}}; !slices.Equal(got, want) {
		t.Errorf("terms = %#v, want %#v", got, want)
	}
	if got, want := progress, []DictionaryProgress{{Completed: 1, Total: 2}, {Completed: 2, Total: 2}}; !slices.Equal(got, want) {
		t.Errorf("progress = %#v, want %#v", got, want)
	}

	requests := fake.RecordedRequests()
	if len(requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(requests))
	}
	for _, request := range requests[:2] {
		if request.Model != "cheap" {
			t.Errorf("extraction model = %q, want cheap", request.Model)
		}
		if strings.Contains(strings.ToLower(request.Prompt), "translat") {
			t.Errorf("extraction prompt requests translation:\n%s", request.Prompt)
		}
	}
	if requests[2].Model != "cheap" {
		t.Errorf("term translation model = %q, want cheap", requests[2].Model)
	}
	for _, term := range []string{"Holmes", "London"} {
		if !strings.Contains(requests[2].Prompt, term) {
			t.Errorf("term translation prompt lacks %q:\n%s", term, requests[2].Prompt)
		}
	}
}
