package translation

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/format"
)

func TestValidateTranslationParsesNodesByTheirMarkers(t *testing.T) {
	source := []format.TextNode{
		{Index: 4, Chapter: 2, Text: "First source node."},
		{Index: 9, Chapter: 2, Text: "Second source node."},
	}
	response := "[[NODE 9]]\nSecond translation.\n[[/NODE]]\n[[NODE 4]]\nFirst translation.\n[[/NODE]]"

	got, err := ValidateTranslation(source, response)
	if err != nil {
		t.Fatalf("ValidateTranslation() error = %v", err)
	}
	want := []format.TextNode{
		{Index: 4, Chapter: 2, Text: "First translation."},
		{Index: 9, Chapter: 2, Text: "Second translation."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ValidateTranslation() = %#v, want %#v", got, want)
	}
}

func TestValidateTranslationPreservesTextNodeWhitespace(t *testing.T) {
	source := []format.TextNode{{Index: 4, Text: "Original."}}
	response := "[[NODE 4]]\n  Translated with whitespace.  \n\n[[/NODE]]"

	got, err := ValidateTranslation(source, response)
	if err != nil {
		t.Fatalf("ValidateTranslation() error = %v", err)
	}
	if want := "  Translated with whitespace.  \n"; got[0].Text != want {
		t.Errorf("translation = %q, want %q", got[0].Text, want)
	}
}

func TestSerializeNodesUsesExplicitIndexMarkers(t *testing.T) {
	nodes := []format.TextNode{{Index: 4, Text: "First."}, {Index: 9, Text: "Second."}}

	got := SerializeNodes(nodes)
	want := "[[NODE 4]]\nFirst.\n[[/NODE]]\n[[NODE 9]]\nSecond.\n[[/NODE]]"
	if got != want {
		t.Errorf("SerializeNodes() = %q, want %q", got, want)
	}
}

func TestValidateTranslationStripsKnownConversationalPrefix(t *testing.T) {
	source := []format.TextNode{{Index: 0, Text: "Original."}}

	got, err := ValidateTranslation(source, "Here is the translation:\n[[NODE 0]]\nTranslated.\n[[/NODE]]")
	if err != nil {
		t.Fatalf("ValidateTranslation() error = %v", err)
	}
	if got[0].Text != "Translated." {
		t.Errorf("translation = %q, want translated text", got[0].Text)
	}
}

func TestValidateTranslationRejectsUnsafeResponses(t *testing.T) {
	source := []format.TextNode{{Index: 4, Text: "First source."}, {Index: 9, Text: "Second source."}}
	cases := map[string]string{
		"fewer nodes":       "[[NODE 4]]\nTranslated first.\n[[/NODE]]",
		"invented index":    "[[NODE 4]]\nTranslated first.\n[[/NODE]]\n[[NODE 12]]\nTranslated second.\n[[/NODE]]",
		"unchanged content": SerializeNodes(source),
		"truncated node":    "[[NODE 4]]\nTranslated first.\n[[/NODE]]\n[[NODE 9]]\nTranslated second.",
	}

	for name, response := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ValidateTranslation(source, response)
			if err == nil {
				t.Fatal("ValidateTranslation() error = nil, want rejection")
			}
		})
	}
}

func TestValidateTranslationRejectsUnexpectedContentAfterTheProtocol(t *testing.T) {
	source := []format.TextNode{{Index: 0, Text: "Original."}}
	response := "[[NODE 0]]\nTranslation.\n[[/NODE]]\nI hope this helps."

	_, err := ValidateTranslation(source, response)
	if err == nil || !strings.Contains(err.Error(), "Text Node marker") {
		t.Errorf("ValidateTranslation() error = %v, want protocol rejection", err)
	}
}
