package translation

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/format"
	"github.com/ijackua/scriptorium/internal/library"
)

// DefaultTermOccurrenceThreshold keeps a Dictionary focused on vocabulary
// that recurs across a Book. It is deliberately a variable on DictionaryBuilder
// so early real Books can tune it without changing the extraction protocol.
const DefaultTermOccurrenceThreshold = 2

// Term is one proposed original-to-translation Dictionary mapping.
type Term = library.Term

// DictionaryProgress says how many extraction Chunks have completed.
type DictionaryProgress struct {
	Completed int
	Total     int
}

// DictionaryBuilder performs the two Dictionary Building passes. The Agent is
// injected because it is the boundary for nondeterministic external work.
type DictionaryBuilder struct {
	Agent               agent.Agent
	MechanicalModel     agent.Model
	OccurrenceThreshold int
	ChunkWordBudget     int
}

// Build extracts candidate Terms from each Chunk, retains only recurring
// candidates, then translates the complete surviving set in one request.
func (b DictionaryBuilder) Build(ctx context.Context, nodes []format.TextNode, sourceLanguage, targetLanguage string, report func(DictionaryProgress)) ([]Term, error) {
	if b.Agent == nil {
		return nil, errors.New("Dictionary Building needs an Agent")
	}
	if strings.TrimSpace(b.MechanicalModel) == "" {
		return nil, errors.New("Dictionary Building needs a mechanical Model")
	}
	threshold := b.OccurrenceThreshold
	if threshold <= 0 {
		threshold = DefaultTermOccurrenceThreshold
	}
	chunks := ChunkNodes(nodes, b.ChunkWordBudget)
	counts := make(map[string]int)
	for index, chunk := range chunks {
		response, err := b.Agent.Call(ctx, agent.Request{Model: b.MechanicalModel, Prompt: extractionPrompt(chunk)})
		if err != nil {
			return nil, fmt.Errorf("extract Terms from Chunk %d: %w", chunk.Index+1, err)
		}
		for term := range termsFromExtraction(response.Result) {
			counts[term]++
		}
		if report != nil {
			report(DictionaryProgress{Completed: index + 1, Total: len(chunks)})
		}
	}

	surviving := make([]string, 0, len(counts))
	for term, occurrences := range counts {
		if occurrences >= threshold {
			surviving = append(surviving, term)
		}
	}
	slices.Sort(surviving)
	if len(surviving) == 0 {
		return nil, nil
	}
	response, err := b.Agent.Call(ctx, agent.Request{
		Model:  b.MechanicalModel,
		Prompt: termTranslationPrompt(surviving, sourceLanguage, targetLanguage),
	})
	if err != nil {
		return nil, fmt.Errorf("translate Dictionary Terms: %w", err)
	}
	terms, err := translatedTerms(response.Result, surviving)
	if err != nil {
		return nil, err
	}
	return terms, nil
}

func extractionPrompt(chunk Chunk) string {
	var prompt strings.Builder
	prompt.WriteString("List candidate recurring names, places, and coined terms verbatim from these source-language Text Nodes. Output one source term per line and nothing else.\n\n")
	for _, node := range chunk.Nodes {
		fmt.Fprintf(&prompt, "[%d]\n%s\n\n", node.Index, node.Text)
	}
	return prompt.String()
}

func termsFromExtraction(result string) map[string]struct{} {
	terms := make(map[string]struct{})
	for _, line := range strings.Split(result, "\n") {
		term := strings.TrimSpace(line)
		if term != "" {
			terms[term] = struct{}{}
		}
	}
	return terms
}

func termTranslationPrompt(terms []string, sourceLanguage, targetLanguage string) string {
	return fmt.Sprintf("Render this complete Dictionary Term set coherently from %s to %s. Return exactly one TSV row per supplied Term: original, translation, note. Leave note blank when unnecessary.\n\n%s", sourceLanguage, targetLanguage, strings.Join(terms, "\n"))
}

func translatedTerms(result string, expected []string) ([]Term, error) {
	translations := make(map[string]Term, len(expected))
	for lineNumber, line := range strings.Split(strings.TrimSpace(result), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || len(fields) > 3 {
			return nil, fmt.Errorf("read translated Dictionary Terms: line %d is not TSV original, translation, note", lineNumber+1)
		}
		original, translation := strings.TrimSpace(fields[0]), strings.TrimSpace(fields[1])
		if original == "" || translation == "" {
			return nil, fmt.Errorf("read translated Dictionary Terms: line %d has an empty original or translation", lineNumber+1)
		}
		note := ""
		if len(fields) == 3 {
			note = strings.TrimSpace(fields[2])
		}
		translations[original] = Term{Original: original, Translation: translation, Note: note}
	}
	terms := make([]Term, 0, len(expected))
	for _, original := range expected {
		term, ok := translations[original]
		if !ok {
			return nil, fmt.Errorf("read translated Dictionary Terms: no translation for %q", original)
		}
		terms = append(terms, term)
	}
	return terms, nil
}
