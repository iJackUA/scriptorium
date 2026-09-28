package translation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ijackua/scriptorium/internal/format"
	"github.com/ijackua/scriptorium/internal/library"
)

// Prompt slots are documented here, beside the built-in templates:
//
//   - {{SOURCE_LANGUAGE}}: the Source Language. The translation prompt gets
//     its human-readable name and canonical Language Tag.
//   - {{TARGET_LANGUAGE}}: the Target Language, likewise.
//   - {{DICTIONARY}}: the complete merged Dictionary as TSV.
//   - {{CONTINUITY_WINDOW}}: the preceding source tail and accepted translation,
//     or an explicit empty marker at the start of a Chapter.
//   - {{NUMBERED_NODE_INSTRUCTIONS}}: the validator's response protocol.
//   - {{TEXT_NODES}}: the numbered source Text Nodes to translate, or to read
//     Terms out of.
//   - {{TERMS}}: the recurring candidate Terms, one per line, to translate.
const (
	sourceLanguageSlot           = "{{SOURCE_LANGUAGE}}"
	targetLanguageSlot           = "{{TARGET_LANGUAGE}}"
	dictionarySlot               = "{{DICTIONARY}}"
	continuityWindowSlot         = "{{CONTINUITY_WINDOW}}"
	numberedNodeInstructionsSlot = "{{NUMBERED_NODE_INSTRUCTIONS}}"
	textNodesSlot                = "{{TEXT_NODES}}"
	termsSlot                    = "{{TERMS}}"
)

// Prompt names one of the prompt templates sent to an Agent. Each has a
// built-in default, and each may be replaced — never appended to — by a file
// in the workspace; see LoadPrompt for which file wins.
type Prompt string

const (
	// PromptTranslation translates one Chunk.
	PromptTranslation Prompt = "translation"
	// PromptTermExtraction reads candidate Terms out of one Chunk during
	// Dictionary Building.
	PromptTermExtraction Prompt = "dictionary-extraction"
	// PromptTermTranslation renders the surviving Terms into the Target
	// Language, at the end of Dictionary Building.
	PromptTermTranslation Prompt = "dictionary-terms"
)

type promptSpec struct {
	label   string
	hint    string
	builtIn string
	// slots are the ones a template must carry. An override without one
	// would, say, translate a whole book without its Dictionary, and nobody
	// would find out until reading it.
	slots []string
}

var prompts = map[Prompt]promptSpec{
	PromptTranslation: {
		label:   "Translation",
		hint:    "Sent for every Chunk translated.",
		builtIn: DefaultPromptTemplate,
		slots:   []string{sourceLanguageSlot, targetLanguageSlot, dictionarySlot, continuityWindowSlot, numberedNodeInstructionsSlot, textNodesSlot},
	},
	PromptTermExtraction: {
		label:   "Dictionary Building: Term extraction",
		hint:    "Sent for every Chunk read while proposing Terms.",
		builtIn: defaultTermExtractionTemplate,
		slots:   []string{textNodesSlot},
	},
	PromptTermTranslation: {
		label:   "Dictionary Building: Term translation",
		hint:    "Sent once, with every recurring Term, to translate them together.",
		builtIn: defaultTermTranslationTemplate,
		slots:   []string{sourceLanguageSlot, targetLanguageSlot, termsSlot},
	},
}

// Prompts lists every Prompt in the order the interface shows them.
func Prompts() []Prompt {
	return []Prompt{PromptTermExtraction, PromptTermTranslation, PromptTranslation}
}

// ParsePrompt reads a Prompt name as it appears in a file name or a URL.
func ParsePrompt(name string) (Prompt, bool) {
	_, ok := prompts[Prompt(name)]
	return Prompt(name), ok
}

func (p Prompt) Label() string   { return prompts[p].label }
func (p Prompt) Hint() string    { return prompts[p].hint }
func (p Prompt) BuiltIn() string { return prompts[p].builtIn }

// Slots are the slots a template for this Prompt must carry.
func (p Prompt) Slots() []string { return slices.Clone(prompts[p].slots) }

// DefaultPromptTemplate is the translation prompt shipped with Scriptorium.
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

const defaultTermExtractionTemplate = `List candidate recurring names, places, and coined terms verbatim from these source-language Text Nodes. Output one source term per line and nothing else.

{{TEXT_NODES}}`

const defaultTermTranslationTemplate = `Render this complete Dictionary Term set coherently from {{SOURCE_LANGUAGE}} to {{TARGET_LANGUAGE}}. Output only raw TSV: exactly one row per supplied Term, with original, translation, note columns; leave note blank when unnecessary. Do not add Markdown, headings, labels, or commentary.

{{TERMS}}`

const numberedNodeInstructions = `Return exactly one block for every supplied Text Node, retaining each global index exactly once.
Use this exact form and return no headings, Markdown, commentary, or other text:
[[NODE <index>]]
<translation>
[[/NODE]]`

type continuityWindow struct {
	source       []format.TextNode
	translations []format.TextNode
}

// WorkspacePromptFile is where the workspace-wide override of p lives,
// relative to the workspace root.
func WorkspacePromptFile(p Prompt) string {
	return filepath.Join(library.PromptsDir, string(p)+".md")
}

// BookPromptFile is where one Book's override of p lives, relative to the
// workspace root.
func BookPromptFile(seriesCode, bookCode string, p Prompt) string {
	return filepath.Join(seriesCode, library.BooksDir, bookCode, library.PromptsDir, string(p)+".md")
}

// PromptFiles are the files that may override p for one Book, most specific
// first: the Book's own, then — for translation only — the Series' prompt.md,
// then the workspace's. The first that exists replaces the built-in template
// outright; the rest are not read.
func PromptFiles(seriesCode, bookCode string, p Prompt) []string {
	files := []string{BookPromptFile(seriesCode, bookCode, p)}
	if p == PromptTranslation {
		files = append(files, filepath.Join(seriesCode, library.PromptFile))
	}
	return append(files, WorkspacePromptFile(p))
}

// FindPrompt returns the first of files that exists under root, and the file
// it came from, or p's built-in template and an empty file when none does. It
// does not validate what it found; see ValidatePrompt.
func FindPrompt(root string, files []string, p Prompt) (template, file string, err error) {
	for _, rel := range files {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", "", fmt.Errorf("read %s: %w", rel, err)
		}
		return string(body), rel, nil
	}
	return p.BuiltIn(), "", nil
}

// ValidatePrompt fails when template lacks any slot p requires, naming every
// missing slot and the file the template came from.
func ValidatePrompt(p Prompt, file, template string) error {
	var missing []string
	for _, slot := range prompts[p].slots {
		if !strings.Contains(template, slot) {
			missing = append(missing, slot)
		}
	}
	switch len(missing) {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("%s is missing the required prompt slot %s", file, missing[0])
	default:
		return fmt.Errorf("%s is missing the required prompt slots %s", file, strings.Join(missing, ", "))
	}
}

// LoadPrompt returns the template of p for one Book: the most specific
// override in PromptFiles, or the built-in template when there is none.
//
// An override is validated here, on load, so that one missing a required slot
// fails before any request is made rather than after a book is translated
// without it.
func LoadPrompt(root, seriesCode, bookCode string, p Prompt) (string, error) {
	template, file, err := FindPrompt(root, PromptFiles(seriesCode, bookCode, p), p)
	if err != nil {
		return "", err
	}
	if file == "" {
		return template, nil
	}
	if err := ValidatePrompt(p, file, template); err != nil {
		return "", err
	}
	return template, nil
}

// SavePrompt validates template and writes it to file under root, creating
// the folders leading down to it.
func SavePrompt(root, file string, p Prompt, template string) error {
	if err := ValidatePrompt(p, file, template); err != nil {
		return err
	}
	path := filepath.Join(root, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create the folder for %s: %w", file, err)
	}
	if err := os.WriteFile(path, []byte(template), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", file, err)
	}
	return nil
}

// RemovePrompt deletes the override at file, so the level above it applies
// again. A file that is already gone is not an error.
func RemovePrompt(root, file string) error {
	if err := os.Remove(filepath.Join(root, file)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", file, err)
	}
	return nil
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
