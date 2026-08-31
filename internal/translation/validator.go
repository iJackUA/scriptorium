package translation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ijackua/scriptorium/internal/format"
)

var conversationalPrefix = regexp.MustCompile(`(?is)^\s*(?:sure[!,]?\s*)?(?:here (?:is|are) (?:the )?(?:translation|translated text)|(?:the )?translation)\s*:\s*`)

var nodeMarker = regexp.MustCompile(`^\[\[NODE ([0-9]+)\]\]\n`)

const nodeEndMarker = "\n[[/NODE]]"

// SerializeNodes encodes Text Nodes in the numbered-node protocol used by
// Chunk artifacts and Agent requests.
func SerializeNodes(nodes []format.TextNode) string {
	var serialized strings.Builder
	for i, node := range nodes {
		if i > 0 {
			serialized.WriteByte('\n')
		}
		fmt.Fprintf(&serialized, "[[NODE %d]]\n%s\n[[/NODE]]", node.Index, node.Text)
	}
	return serialized.String()
}

// ValidateTranslation decodes a numbered-node response and ensures it maps
// exactly to source. Leading, known conversational framing is ignored.
func ValidateTranslation(source []format.TextNode, response string) ([]format.TextNode, error) {
	response = conversationalPrefix.ReplaceAllString(response, "")
	response = strings.TrimSpace(response)
	translations, err := parseNumberedNodes(response)
	if err != nil {
		return nil, err
	}
	if len(translations) != len(source) {
		return nil, fmt.Errorf("translation has %d Text Nodes, want %d", len(translations), len(source))
	}

	byIndex := make(map[int]string, len(translations))
	for _, translation := range translations {
		if _, exists := byIndex[translation.Index]; exists {
			return nil, fmt.Errorf("translation repeats Text Node index %d", translation.Index)
		}
		byIndex[translation.Index] = translation.Text
	}

	validated := make([]format.TextNode, len(source))
	unchanged := true
	for i, node := range source {
		text, exists := byIndex[node.Index]
		if !exists {
			return nil, fmt.Errorf("translation is missing Text Node index %d", node.Index)
		}
		validated[i] = format.TextNode{Index: node.Index, Chapter: node.Chapter, Text: text}
		if text != node.Text {
			unchanged = false
		}
	}
	if unchanged {
		return nil, fmt.Errorf("translation is identical to its source")
	}
	return validated, nil
}

func parseNumberedNodes(response string) ([]format.TextNode, error) {
	var nodes []format.TextNode
	for response != "" {
		match := nodeMarker.FindStringSubmatchIndex(response)
		if match == nil {
			return nil, fmt.Errorf("translation does not begin with a Text Node marker")
		}
		index, err := strconv.Atoi(response[match[2]:match[3]])
		if err != nil {
			return nil, fmt.Errorf("invalid Text Node index: %w", err)
		}
		response = response[match[1]:]
		end := strings.Index(response, nodeEndMarker)
		if end < 0 {
			return nil, fmt.Errorf("Text Node %d is truncated", index)
		}
		text := response[:end]
		if text == "" {
			return nil, fmt.Errorf("Text Node %d has no translation", index)
		}
		nodes = append(nodes, format.TextNode{Index: index, Text: text})
		response = response[end+len(nodeEndMarker):]
		if response == "" {
			break
		}
		if !strings.HasPrefix(response, "\n") {
			return nil, fmt.Errorf("unexpected content after Text Node %d", index)
		}
		response = strings.TrimPrefix(response, "\n")
	}
	return nodes, nil
}
