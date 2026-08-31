package translation

import (
	"testing"

	"github.com/ijackua/scriptorium/internal/format"
)

func TestChunkNodesKeepsNodesWholeWithinTheirChapterAndBudget(t *testing.T) {
	nodes := []format.TextNode{
		{Index: 0, Chapter: 0, Text: "one two"},
		{Index: 1, Chapter: 0, Text: "three four"},
		{Index: 2, Chapter: 0, Text: "five six"},
		{Index: 3, Chapter: 1, Text: "seven eight"},
	}

	got := ChunkNodes(nodes, 4)
	if len(got) != 3 {
		t.Fatalf("chunks = %d, want 3", len(got))
	}
	if got[0].Chapter != 0 || len(got[0].Nodes) != 2 || got[0].Nodes[1].Index != 1 {
		t.Errorf("first chunk = %#v, want the first two nodes from chapter 0", got[0])
	}
	if got[1].Chapter != 0 || len(got[1].Nodes) != 1 || got[1].Nodes[0].Index != 2 {
		t.Errorf("second chunk = %#v, want the remaining chapter 0 node", got[1])
	}
	if got[2].Chapter != 1 || len(got[2].Nodes) != 1 || got[2].Nodes[0].Index != 3 {
		t.Errorf("third chunk = %#v, want the chapter 1 node", got[2])
	}
}

func TestChunkNodesAllowsAnOversizedNodeOnlyByItself(t *testing.T) {
	nodes := []format.TextNode{
		{Index: 0, Chapter: 0, Text: "one two three"},
		{Index: 1, Chapter: 0, Text: "four five six seven eight"},
		{Index: 2, Chapter: 0, Text: "nine ten"},
	}

	got := ChunkNodes(nodes, 3)
	if len(got) != 3 {
		t.Fatalf("chunks = %d, want 3", len(got))
	}
	for i, chunk := range got {
		if len(chunk.Nodes) != 1 || chunk.Nodes[0].Index != i {
			t.Errorf("chunks[%d] = %#v, want node %d alone", i, chunk, i)
		}
	}
}
