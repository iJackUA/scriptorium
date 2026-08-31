// Package txt handles plain-text Source Files without changing their
// Text Node separators or line endings during Book Composition.
package txt

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/ijackua/scriptorium/internal/format"
)

// TextNode and Chapter are aliases so TXT and FB2 handlers expose the same
// pipeline data model.
type TextNode = format.TextNode
type Chapter = format.Chapter

// Handler adapts the TXT parser to the format-neutral handler boundary.
type Handler struct {
	Options Options
}

var _ format.Handler = Handler{}
var _ format.Document = Document{}

// Options controls how Chapters are detected for one Book. A non-nil
// ChapterPatterns value replaces the built-in patterns; an empty non-nil slice
// therefore deliberately disables automatic detection for that Book.
type Options struct {
	ChapterPatterns []string
}

// DefaultChapterPatterns are the built-in heuristics used when no per-Book
// override is supplied. Patterns are matched against one complete Text Node,
// after surrounding whitespace has been removed.
var DefaultChapterPatterns = []string{
	`(?i)^\s*(?:chapter|chap\.?|ch\.?)\s+(?:[0-9]+|[ivxlcdm]+|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty)(?:\s*[-:.—)]\s*.*|\s+.+)?\s*$`,
	`(?i)^\s*(?:part|book|volume|vol\.?)\s+(?:[0-9]+|[ivxlcdm]+|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty)(?:\s*[-:.—)]\s*.*|\s+.+)?\s*$`,
	`(?i)^\s*(?:prologue|epilogue|preface|introduction|foreword|afterword|appendix)\s*$`,
	`(?i)^\s*(?:[0-9]{1,3}|[ivxlcdm]{1,12})[.)]?(?:\s+[^,;!?]{1,120})?\s*$`,
}

// Document is a parsed TXT Source File and the source ranges required to
// splice translated Text Nodes back into their original positions.
type Document struct {
	Nodes    []TextNode
	Chapters []Chapter

	source []byte
	refs   []textRange
}

type textRange struct {
	start int
	end   int
}

type lineRange struct {
	start      int
	contentEnd int
}

type edit struct {
	start int
	end   int
	text  []byte
}

// Parse reads a TXT Source File into blank-line-separated Text Nodes. The
// source is retained internally so Splice preserves every byte not belonging
// to translated Text Node text.
func Parse(source []byte, options ...Options) (Document, error) {
	if len(options) > 1 {
		return Document{}, fmt.Errorf("parse TXT: got %d options, want at most 1", len(options))
	}

	patterns := DefaultChapterPatterns
	if len(options) == 1 && options[0].ChapterPatterns != nil {
		patterns = options[0].ChapterPatterns
	}
	detectors, err := compilePatterns(patterns)
	if err != nil {
		return Document{}, fmt.Errorf("parse TXT chapter patterns: %w", err)
	}

	document := Document{source: bytes.Clone(source)}
	nodeRanges := textNodeRanges(source)
	document.Chapters = append(document.Chapters, Chapter{Index: 0, Parent: -1})
	for _, nodeRange := range nodeRanges {
		text := string(source[nodeRange.start:nodeRange.end])
		heading := matchesAny(detectors, strings.TrimSpace(text))
		chapter := &document.Chapters[len(document.Chapters)-1]
		if len(chapter.Nodes) > 0 && heading {
			chapter = &Chapter{Index: len(document.Chapters), Parent: -1}
			document.Chapters = append(document.Chapters, *chapter)
			chapter = &document.Chapters[len(document.Chapters)-1]
		}

		index := len(document.Nodes)
		document.Nodes = append(document.Nodes, TextNode{
			Index:   index,
			Text:    text,
			Chapter: chapter.Index,
		})
		document.refs = append(document.refs, nodeRange)
		chapter.Nodes = append(chapter.Nodes, index)
		if chapter.Title == "" && heading {
			chapter.Title = strings.TrimSpace(text)
		}
	}

	return document, nil
}

func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for i, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("pattern %d: %w", i, err)
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

func matchesAny(patterns []*regexp.Regexp, text string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func textNodeRanges(source []byte) []textRange {
	lines := splitLines(source)
	nodeRanges := make([]textRange, 0)
	var nodeRange *textRange
	for _, line := range lines {
		if strings.TrimSpace(string(source[line.start:line.contentEnd])) == "" {
			if nodeRange != nil {
				nodeRanges = append(nodeRanges, *nodeRange)
				nodeRange = nil
			}
			continue
		}
		if nodeRange == nil {
			nodeRange = &textRange{start: line.start, end: line.contentEnd}
		} else {
			nodeRange.end = line.contentEnd
		}
	}
	if nodeRange != nil {
		nodeRanges = append(nodeRanges, *nodeRange)
	}
	return nodeRanges
}

func splitLines(source []byte) []lineRange {
	lines := make([]lineRange, 0)
	for start := 0; start < len(source); {
		separator := start
		for separator < len(source) && source[separator] != '\n' && source[separator] != '\r' {
			separator++
		}
		lineEnd := separator
		if separator < len(source) {
			lineEnd++
			if source[separator] == '\r' && lineEnd < len(source) && source[lineEnd] == '\n' {
				lineEnd++
			}
		}
		lines = append(lines, lineRange{start: start, contentEnd: separator})
		if lineEnd == start {
			break
		}
		start = lineEnd
	}
	return lines
}

// Splice returns the original TXT Source File with each translated Text Node
// placed at its original position. It validates the complete indexed list
// before producing output so a missing or shifted translation cannot silently
// corrupt later Text Nodes.
func (d Document) Splice(translations []TextNode) ([]byte, error) {
	if len(translations) != len(d.Nodes) {
		return nil, fmt.Errorf("splice TXT: got %d Text Nodes, want %d", len(translations), len(d.Nodes))
	}
	if len(d.refs) != len(d.Nodes) {
		return nil, fmt.Errorf("splice TXT: document has inconsistent node structure")
	}

	edits := make([]edit, 0, len(translations))
	for i, translated := range translations {
		original := d.Nodes[i]
		if translated.Index != original.Index {
			return nil, fmt.Errorf("splice TXT: Text Node %d has index %d", i, translated.Index)
		}
		if translated.Text == original.Text {
			continue
		}
		ref := d.refs[i]
		edits = append(edits, edit{start: ref.start, end: ref.end, text: []byte(translated.Text)})
	}

	var output bytes.Buffer
	output.Grow(len(d.source))
	cursor := 0
	for _, change := range edits {
		if change.start < cursor || change.end < change.start || change.end > len(d.source) {
			return nil, fmt.Errorf("splice TXT: overlapping or invalid source ranges")
		}
		output.Write(d.source[cursor:change.start])
		output.Write(change.text)
		cursor = change.end
	}
	output.Write(d.source[cursor:])
	return output.Bytes(), nil
}

// Parse implements format.Handler for a per-Book parser configuration.
func (h Handler) Parse(source []byte) (format.Document, error) {
	return Parse(source, h.Options)
}

// TextNodes implements format.Document.
func (d Document) TextNodes() []TextNode { return d.Nodes }

// ChaptersList implements format.Document.
func (d Document) ChaptersList() []Chapter { return d.Chapters }
