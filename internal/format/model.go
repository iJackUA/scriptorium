// Package format contains the data shared by Source File handlers.
package format

// TextNode is one addressable piece of prose in a Source File. Index is the
// stable position used to put a translated node back in its original place;
// Chapter identifies the division that bounds chunking.
type TextNode struct {
	Index   int
	Text    string
	Chapter int
}

// Chapter is one structural division of a Book. Nodes contains the indices of
// the Text Nodes directly belonging to it.
type Chapter struct {
	Index  int
	Parent int
	Title  string
	Nodes  []int
}

// Document is the format-neutral view consumed by the translation pipeline.
// Concrete handlers may expose additional format-specific data, but the
// pipeline only needs the ordered Text Nodes, Chapter boundaries, and splice
// operation.
type Document interface {
	TextNodes() []TextNode
	ChaptersList() []Chapter
	Splice([]TextNode) ([]byte, error)
}

// Handler parses a Source File into a format-neutral Document.
type Handler interface {
	Parse([]byte) (Document, error)
}
