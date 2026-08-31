package fb2

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestIdentitySplicePreservesEveryFB2CorpusMember(t *testing.T) {
	cases := []struct {
		name string
		read func(t *testing.T) []byte
	}{
		{
			name: "Sherlock Holmes",
			read: func(t *testing.T) []byte {
				return readFile(t, filepath.Join("..", "..", "..", "test_data", "The Adventures of Sherlock Holmes.fb2"))
			},
		},
		{
			name: "awkward markup",
			read: func(t *testing.T) []byte { return []byte(awkwardFB2) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.read(t)
			document, err := Parse(source)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			got, err := document.Splice(document.Nodes)
			if err != nil {
				t.Fatalf("Splice: %v", err)
			}
			if !bytes.Equal(got, source) {
				t.Fatalf("identity splice changed the source: got %d bytes, want %d", len(got), len(source))
			}

			if tc.name == "Sherlock Holmes" && len(document.Nodes) < 1000 {
				t.Fatalf("parsed %d Text Nodes from the corpus, want at least 1000", len(document.Nodes))
			}
		})
	}
}

func TestParseExposesOrderedNodesAndFB2Sections(t *testing.T) {
	document, err := Parse([]byte(awkwardFB2))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got, want := len(document.Chapters), 3; got != want {
		t.Fatalf("Chapters = %d, want %d", got, want)
	}
	if got, want := document.Chapters[0].Title, "The chapter"; got != want {
		t.Errorf("first Chapter title = %q, want %q", got, want)
	}
	if got, want := document.Chapters[1].Title, "Nested scene"; got != want {
		t.Errorf("second Chapter title = %q, want %q", got, want)
	}
	if got, want := document.Chapters[2].Title, "Notes"; got != want {
		t.Errorf("third Chapter title = %q, want %q", got, want)
	}
	if got, want := document.Chapters[0].Parent, -1; got != want {
		t.Errorf("first Chapter parent = %d, want %d", got, want)
	}
	if got, want := document.Chapters[1].Parent, 0; got != want {
		t.Errorf("nested Chapter parent = %d, want %d", got, want)
	}
	if got, want := document.Chapters[2].Parent, -1; got != want {
		t.Errorf("notes Chapter parent = %d, want %d", got, want)
	}

	wantText := []string{
		"The chapter",
		"Before bold nested after [1].",
		"",
		"A line of verse.",
		"Another line.",
		"Nested scene",
		"Nested prose.",
		"Notes",
		"Footnote text.",
	}
	if len(document.Nodes) != len(wantText) {
		t.Fatalf("Nodes = %d, want %d (%q)", len(document.Nodes), len(wantText), wantText)
	}
	for i, want := range wantText {
		if got := document.Nodes[i].Text; got != want {
			t.Errorf("Nodes[%d].Text = %q, want %q", i, got, want)
		}
		if document.Nodes[i].Index != i {
			t.Errorf("Nodes[%d].Index = %d, want %d", i, document.Nodes[i].Index, i)
		}
	}
	if got, want := document.Chapters[0].Nodes, []int{0, 1, 2, 3, 4}; !equalInts(got, want) {
		t.Errorf("first Chapter nodes = %v, want %v", got, want)
	}
	if got, want := document.Chapters[1].Nodes, []int{5, 6}; !equalInts(got, want) {
		t.Errorf("second Chapter nodes = %v, want %v", got, want)
	}
	if got, want := document.Chapters[2].Nodes, []int{7, 8}; !equalInts(got, want) {
		t.Errorf("third Chapter nodes = %v, want %v", got, want)
	}
}

func TestSpliceReplacesTextWithoutRemovingInlineMarkup(t *testing.T) {
	document, err := Parse([]byte(awkwardFB2))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	translations := append([]TextNode(nil), document.Nodes...)
	translations[1].Text = "Translated prose."
	got, err := document.Splice(translations)
	if err != nil {
		t.Fatalf("Splice: %v", err)
	}
	for _, want := range []string{"Translated prose.", "<strong>", "<emphasis>", "<a type=\"note\""} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("spliced FB2 does not contain %q", want)
		}
	}
	for _, old := range []string{"Before ", "bold ", "nested", " after ", "[1]."} {
		if bytes.Contains(got, []byte(old)) {
			t.Errorf("spliced FB2 still contains source text %q", old)
		}
	}
}

func TestSpliceRejectsMisalignedTranslations(t *testing.T) {
	document, err := Parse([]byte(awkwardFB2))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if _, err := document.Splice(document.Nodes[:len(document.Nodes)-1]); err == nil {
		t.Fatal("Splice accepted the wrong number of Text Nodes")
	}
	translations := append([]TextNode(nil), document.Nodes...)
	translations[0].Index = 99
	if _, err := document.Splice(translations); err == nil {
		t.Fatal("Splice accepted a Text Node with the wrong index")
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return contents
}

func equalInts(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

const awkwardFB2 = `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
<description><title-info><book-title>Fixture</book-title></title-info></description>
<body><section id="chapter-1"><title><p>The chapter</p></title>
<p>Before <strong>bold <emphasis>nested</emphasis></strong> after <a type="note" l:href="#note-1">[1]</a>.</p>
<p></p><poem><stanza><v>A line of verse.</v><v>Another line.</v></stanza></poem>
<section id="scene"><title><p>Nested scene</p></title><p>Nested prose.</p></section>
</section></body>
<body name="notes"><section id="note-1"><title><p>Notes</p></title><p>Footnote text.</p></section></body>
</FictionBook>
`
