// Package translation contains pure building blocks for the translation
// pipeline.
package translation

import (
	"strings"

	"github.com/ijackua/scriptorium/internal/format"
)

// DefaultWordBudget is the approximate number of words sent in one Agent
// request. A Text Node larger than the budget is kept whole in its own Chunk.
const DefaultWordBudget = 2000

// Chunk is a consecutive run of Text Nodes from one Chapter.
type Chunk struct {
	Index   int
	Chapter int
	Nodes   []format.TextNode
}

// ChunkNodes groups consecutive Text Nodes without splitting a node or
// crossing a Chapter boundary. A non-positive wordBudget uses DefaultWordBudget.
func ChunkNodes(nodes []format.TextNode, wordBudget int) []Chunk {
	if wordBudget <= 0 {
		wordBudget = DefaultWordBudget
	}

	var chunks []Chunk
	var current *Chunk
	words := 0
	for _, node := range nodes {
		nodeWords := len(strings.Fields(node.Text))
		if current == nil || current.Chapter != node.Chapter || (len(current.Nodes) > 0 && words+nodeWords > wordBudget) {
			chunks = append(chunks, Chunk{Index: len(chunks), Chapter: node.Chapter})
			current = &chunks[len(chunks)-1]
			words = 0
		}
		current.Nodes = append(current.Nodes, node)
		words += nodeWords
	}
	return chunks
}
