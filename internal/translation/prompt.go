package translation

import (
	"fmt"
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

// DefaultPromptTemplate is the translation prompt shipped with Scriptorium.
// Per-Series replacement of this template is intentionally a later feature.
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
	source      format.TextNode
	translation format.TextNode
	present     bool
}

func translationPrompt(sourceLanguage, targetLanguage string, dictionary []library.Term, continuity continuityWindow, nodes []format.TextNode) string {
	return strings.NewReplacer(
		sourceLanguageSlot, sourceLanguage,
		targetLanguageSlot, targetLanguage,
		dictionarySlot, formatDictionary(dictionary),
		continuityWindowSlot, formatContinuityWindow(continuity),
		numberedNodeInstructionsSlot, numberedNodeInstructions,
		textNodesSlot, SerializeNodes(nodes),
	).Replace(DefaultPromptTemplate)
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
	if !window.present {
		return "(empty — start of Chapter)"
	}
	return fmt.Sprintf("Previous source tail:\n%s\n\nIts accepted translation:\n%s", SerializeNodes([]format.TextNode{window.source}), SerializeNodes([]format.TextNode{window.translation}))
}
