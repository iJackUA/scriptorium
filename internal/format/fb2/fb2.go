// Package fb2 handles FictionBook 2 documents without exposing their markup to
// the translation pipeline.
package fb2

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ijackua/scriptorium/internal/format"
)

// TextNode and Chapter are aliases so every format handler shares the same
// pipeline data model.
type TextNode = format.TextNode
type Chapter = format.Chapter

// Handler adapts the FB2 parser to the format-neutral handler boundary.
type Handler struct{}

var _ format.Handler = Handler{}
var _ format.Document = Document{}

// Parse implements format.Handler.
func (Handler) Parse(source []byte) (format.Document, error) {
	return Parse(source)
}

// Document is a parsed FB2 document and the structure required to splice
// translations back into its original bytes.
type Document struct {
	Nodes    []TextNode
	Chapters []Chapter

	source []byte
	refs   []nodeReference
}

type nodeReference struct {
	spans    []textSpan
	insertAt int
}

type textSpan struct {
	start int
	end   int
}

type sectionState struct {
	index int
}

type nodeState struct {
	kind     string
	chapter  int
	text     []string
	spans    []textSpan
	insertAt int
}

type titleState struct {
	chapter int
	text    []string
}

// Parse reads an FB2 source into plain Text Nodes and its section structure.
// The source is retained internally so Splice can preserve every original
// element, attribute, namespace, comment, and byte of formatting around the
// translated text.
func Parse(source []byte) (Document, error) {
	decoder := xml.NewDecoder(bytes.NewReader(source))
	var document Document
	document.source = bytes.Clone(source)

	sections := make([]sectionState, 0)
	titles := make([]titleState, 0)
	var activeNode *nodeState
	bodyDepth := 0
	depth := 0
	rootSeen := false

	for {
		tokenStart := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Document{}, fmt.Errorf("parse FB2 XML: %w", err)
		}
		tokenEnd := int(decoder.InputOffset())

		switch token := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				if rootSeen {
					return Document{}, fmt.Errorf("parse FB2 XML: multiple root elements")
				}
				rootSeen = true
				if token.Name.Local != "FictionBook" {
					return Document{}, fmt.Errorf("parse FB2 XML: root element is %q, want FictionBook", token.Name.Local)
				}
			}
			depth++

			switch token.Name.Local {
			case "body":
				bodyDepth++
			case "section":
				if bodyDepth > 0 {
					parent := -1
					if len(sections) > 0 {
						parent = sections[len(sections)-1].index
					}
					chapter := Chapter{Index: len(document.Chapters), Parent: parent}
					document.Chapters = append(document.Chapters, chapter)
					sections = append(sections, sectionState{index: chapter.Index})
				}
			case "title":
				if bodyDepth > 0 && len(sections) > 0 {
					titles = append(titles, titleState{chapter: sections[len(sections)-1].index})
				}
			case "p", "v":
				if bodyDepth > 0 && activeNode == nil {
					chapter := currentChapter(&document, sections)
					activeNode = &nodeState{
						kind:     token.Name.Local,
						chapter:  chapter,
						insertAt: tokenEnd,
					}
				}
			}

		case xml.CharData:
			text := string(token)
			if activeNode != nil {
				activeNode.text = append(activeNode.text, text)
				activeNode.spans = append(activeNode.spans, textSpan{start: tokenStart, end: tokenEnd})
			}
			if len(titles) > 0 {
				titles[len(titles)-1].text = append(titles[len(titles)-1].text, text)
			}

		case xml.EndElement:
			switch token.Name.Local {
			case "p", "v":
				if activeNode != nil && activeNode.kind == token.Name.Local {
					appendNode(&document, *activeNode)
					activeNode = nil
				}
			case "title":
				if len(titles) > 0 {
					title := titles[len(titles)-1]
					titles = titles[:len(titles)-1]
					document.Chapters[title.chapter].Title = strings.TrimSpace(strings.Join(title.text, ""))
				}
			case "section":
				if len(sections) > 0 {
					sections = sections[:len(sections)-1]
				}
			case "body":
				bodyDepth--
			}
			depth--
		}
	}

	if !rootSeen {
		return Document{}, fmt.Errorf("parse FB2 XML: document has no root element")
	}
	return document, nil
}

func currentChapter(document *Document, sections []sectionState) int {
	if len(sections) > 0 {
		return sections[len(sections)-1].index
	}
	if len(document.Chapters) == 0 {
		document.Chapters = append(document.Chapters, Chapter{Index: 0, Parent: -1})
	}
	return document.Chapters[len(document.Chapters)-1].Index
}

func appendNode(document *Document, node nodeState) {
	index := len(document.Nodes)
	document.Nodes = append(document.Nodes, TextNode{
		Index:   index,
		Text:    strings.Join(node.text, ""),
		Chapter: node.chapter,
	})
	document.refs = append(document.refs, nodeReference{
		spans:    node.spans,
		insertAt: node.insertAt,
	})
	document.Chapters[node.chapter].Nodes = append(document.Chapters[node.chapter].Nodes, index)
}

type edit struct {
	start int
	end   int
	text  []byte
}

// Splice returns the original FB2 with each translated Text Node placed at its
// original position. It requires the same nodes, in the same indexed order,
// so a missing or shifted translation cannot silently corrupt later chapters.
func (d Document) Splice(translations []TextNode) ([]byte, error) {
	if len(translations) != len(d.Nodes) {
		return nil, fmt.Errorf("splice FB2: got %d Text Nodes, want %d", len(translations), len(d.Nodes))
	}
	if len(d.refs) != len(d.Nodes) {
		return nil, fmt.Errorf("splice FB2: document has inconsistent node structure")
	}

	edits := make([]edit, 0, len(translations))
	for i, translated := range translations {
		original := d.Nodes[i]
		if translated.Index != original.Index {
			return nil, fmt.Errorf("splice FB2: Text Node %d has index %d", i, translated.Index)
		}
		if translated.Text == original.Text {
			continue
		}
		escaped, err := escapeText(translated.Text)
		if err != nil {
			return nil, fmt.Errorf("splice FB2 Text Node %d: %w", i, err)
		}
		ref := d.refs[i]
		if len(ref.spans) == 0 {
			edits = append(edits, edit{start: ref.insertAt, end: ref.insertAt, text: escaped})
			continue
		}
		edits = append(edits, edit{start: ref.spans[0].start, end: ref.spans[0].end, text: escaped})
		for _, span := range ref.spans[1:] {
			edits = append(edits, edit{start: span.start, end: span.end})
		}
	}

	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var output bytes.Buffer
	output.Grow(len(d.source))
	cursor := 0
	for _, change := range edits {
		if change.start < cursor || change.end < change.start || change.end > len(d.source) {
			return nil, fmt.Errorf("splice FB2: overlapping or invalid source ranges")
		}
		output.Write(d.source[cursor:change.start])
		output.Write(change.text)
		cursor = change.end
	}
	output.Write(d.source[cursor:])
	return output.Bytes(), nil
}

// TextNodes implements format.Document.
func (d Document) TextNodes() []TextNode { return d.Nodes }

// ChaptersList implements format.Document.
func (d Document) ChaptersList() []Chapter { return d.Chapters }

func escapeText(text string) ([]byte, error) {
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(text)); err != nil {
		return nil, err
	}
	return escaped.Bytes(), nil
}
