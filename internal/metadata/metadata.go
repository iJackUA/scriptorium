// Package metadata obtains Book details from the cheapest reliable source.
package metadata

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/ijackua/scriptorium/internal/agent"
)

// Fields are the editable details displayed for a Book.
type Fields struct {
	Title              string `json:"title"`
	Author             string `json:"author"`
	SourceFileLanguage string `json:"language"`
}

// FB2 reads title, author and language from the FB2 description block. A
// missing or malformed description is ordinary absent metadata, not a source
// file failure.
func FB2(source []byte) Fields {
	var document struct {
		Description struct {
			TitleInfo struct {
				Title  string `xml:"book-title"`
				Lang   string `xml:"lang"`
				Author []struct {
					First  string `xml:"first-name"`
					Middle string `xml:"middle-name"`
					Last   string `xml:"last-name"`
					Nick   string `xml:"nickname"`
				} `xml:"author"`
			} `xml:"title-info"`
		} `xml:"description"`
	}
	if err := xml.Unmarshal(source, &document); err != nil {
		return Fields{}
	}
	fields := Fields{
		Title:              strings.TrimSpace(document.Description.TitleInfo.Title),
		SourceFileLanguage: strings.TrimSpace(document.Description.TitleInfo.Lang),
	}
	if len(document.Description.TitleInfo.Author) > 0 {
		author := document.Description.TitleInfo.Author[0]
		fields.Author = strings.Join(nonEmpty(author.First, author.Middle, author.Last, author.Nick), " ")
	}
	return fields
}

// InferText asks the Agent for the title and author apparent in a plain-text
// Book's opening pages. Language is intentionally absent: it belongs to the
// source chosen for the Series, not a text inference.
func InferText(ctx context.Context, client agent.Agent, model agent.Model, source []byte) (Fields, error) {
	const openingPageBytes = 12_000
	if len(source) > openingPageBytes {
		source = source[:openingPageBytes]
	}
	response, err := client.Call(ctx, agent.Request{
		Model:  model,
		Effort: agent.EffortLow,
		Prompt: "Infer the title and author of this plain-text Book from its opening pages. Return only a JSON object with string fields title and author. Use empty strings when a field is unknown.\n\nOPENING PAGES:\n" + string(source),
	})
	if err != nil {
		return Fields{}, err
	}
	var fields Fields
	if err := json.Unmarshal([]byte(response.Result), &fields); err != nil {
		return Fields{}, fmt.Errorf("decode inferred metadata: %w", err)
	}
	fields.Title = strings.TrimSpace(fields.Title)
	fields.Author = strings.TrimSpace(fields.Author)
	fields.SourceFileLanguage = ""
	return fields, nil
}

func nonEmpty(values ...string) []string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			kept = append(kept, value)
		}
	}
	return kept
}
