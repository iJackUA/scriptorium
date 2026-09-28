package translation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ijackua/scriptorium/internal/format"
	"github.com/ijackua/scriptorium/internal/library"
)

// Translation prompt slots are documented here, beside the built-in template:
//
//   - {{SOURCE_LANGUAGE}}: human-readable name and canonical Language Tag.
//   - {{TARGET_LANGUAGE}}: human-readable name and canonical Language Tag.
//   - {{DICTIONARY}}: the complete merged Dictionary as TSV.
//   - {{CONTINUITY_WINDOW}}: the preceding source tail and accepted translation,
//     or an explicit empty marker at the start of a Chapter.
//   - {{NUMBERED_NODE_INSTRUCTIONS}}: the validator's response protocol.
//   - {{TEXT_NODES}}: the numbered source Text Nodes to translate.
const (
	sourceLanguageSlot           = "{{SOURCE_LANGUAGE}}"
	targetLanguageSlot           = "{{TARGET_LANGUAGE}}"
	dictionarySlot               = "{{DICTIONARY}}"
	continuityWindowSlot         = "{{CONTINUITY_WINDOW}}"
	numberedNodeInstructionsSlot = "{{NUMBERED_NODE_INSTRUCTIONS}}"
	textNodesSlot                = "{{TEXT_NODES}}"
)

// requiredSlots are the slots every prompt template must carry. An override
// without one would translate a whole book without, say, its Dictionary, and
// nobody would find out until reading it.
var requiredSlots = []string{
	sourceLanguageSlot,
	targetLanguageSlot,
	dictionarySlot,
	continuityWindowSlot,
	numberedNodeInstructionsSlot,
	textNodesSlot,
}

// DefaultPromptTemplate is the translation prompt shipped with Scriptorium.
// A Series may replace it with its own prompt.md; see LoadPromptTemplate.
const DefaultPromptTemplate = `Translate the supplied Text Nodes from {{SOURCE_LANGUAGE}} to {{TARGET_LANGUAGE}}.

DICTIONARY — apply every approved Term exactly:
{{DICTIONARY}}

CONTINUITY WINDOW — REFERENCE MATERIAL ONLY. DO NOT TRANSLATE THIS SECTION:
{{CONTINUITY_WINDOW}}
END CONTINUITY WINDOW

NUMBERED-NODE RESPONSE PROTOCOL:
{{NUMBERED_NODE_INSTRUCTIONS}}

TEXT NODES TO TRANSLATE:
{{TEXT_NODES}}`

const numberedNodeInstructions = `Return exactly one block for every supplied Text Node, retaining each global index exactly once.
Use this exact form and return no headings, Markdown, commentary, or other text:
[[NODE <index>]]
<translation>
[[/NODE]]`

type continuityWindow struct {
	source       []format.TextNode
	translations []format.TextNode
}

// LoadPromptTemplate returns the translation prompt template for a Series:
// its prompt.md when there is one, which replaces the built-in template
// outright, and DefaultPromptTemplate otherwise.
//
// An override is validated here, on load, so that one missing a required slot
// fails before any request is made rather than after a book is translated
// without it.
func LoadPromptTemplate(root, seriesCode string) (string, error) {
	rel := filepath.Join(seriesCode, library.PromptFile)
	body, err := os.ReadFile(filepath.Join(root, rel))
	if errors.Is(err, os.ErrNotExist) {
		return DefaultPromptTemplate, nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", rel, err)
	}
	template := string(body)
	var missing []string
	for _, slot := range requiredSlots {
		if !strings.Contains(template, slot) {
			missing = append(missing, slot)
		}
	}
	switch len(missing) {
	case 0:
		return template, nil
	case 1:
		return "", fmt.Errorf("%s is missing the required prompt slot %s", rel, missing[0])
	default:
		return "", fmt.Errorf("%s is missing the required prompt slots %s", rel, strings.Join(missing, ", "))
	}
}

func translationPrompt(template, sourceLanguage, targetLanguage string, dictionary []library.Term, continuity continuityWindow, nodes []format.TextNode) string {
	return strings.NewReplacer(
		sourceLanguageSlot, sourceLanguage,
		targetLanguageSlot, targetLanguage,
		dictionarySlot, formatDictionary(dictionary),
		continuityWindowSlot, formatContinuityWindow(continuity),
		numberedNodeInstructionsSlot, numberedNodeInstructions,
		textNodesSlot, SerializeNodes(nodes),
	).Replace(template)
}

func formatDictionary(terms []library.Term) string {
	if len(terms) == 0 {
		return "(empty)"
	}
	var dictionary strings.Builder
	dictionary.WriteString("original\ttranslation\tnote\n")
	for _, term := range terms {
		fmt.Fprintf(&dictionary, "%s\t%s\t%s\n", term.Original, term.Translation, term.Note)
	}
	return strings.TrimSuffix(dictionary.String(), "\n")
}

func formatContinuityWindow(window continuityWindow) string {
	if len(window.source) == 0 {
		return "(empty — start of Chapter)"
	}
	return fmt.Sprintf("Previous source Chunk:\n%s\n\nIts accepted translation:\n%s", SerializeNodes(window.source), SerializeNodes(window.translations))
}
