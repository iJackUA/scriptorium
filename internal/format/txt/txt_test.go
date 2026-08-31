package txt

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ijackua/scriptorium/internal/format"
)

func TestParseSplitsTextNodesAndDetectsBuiltInChapterHeadings(t *testing.T) {
	source := "Front matter\r\ncontinued\r\n\r\nChapter 1: The beginning\r\n\r\nFirst paragraph.\r\nStill first paragraph.\r\n\r\nII. The next chapter\r\n\r\nSecond paragraph."

	document, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	wantText := []string{
		"Front matter\r\ncontinued",
		"Chapter 1: The beginning",
		"First paragraph.\r\nStill first paragraph.",
		"II. The next chapter",
		"Second paragraph.",
	}
	if len(document.Nodes) != len(wantText) {
		t.Fatalf("Nodes = %d, want %d", len(document.Nodes), len(wantText))
	}
	for i, want := range wantText {
		if got := document.Nodes[i].Text; got != want {
			t.Errorf("Nodes[%d].Text = %q, want %q", i, got, want)
		}
		if document.Nodes[i].Index != i {
			t.Errorf("Nodes[%d].Index = %d, want %d", i, document.Nodes[i].Index, i)
		}
	}

	if len(document.Chapters) != 3 {
		t.Fatalf("Chapters = %d, want 3", len(document.Chapters))
	}
	wantTitles := []string{"", "Chapter 1: The beginning", "II. The next chapter"}
	wantNodes := [][]int{{0}, {1, 2}, {3, 4}}
	for i := range document.Chapters {
		if got := document.Chapters[i].Title; got != wantTitles[i] {
			t.Errorf("Chapters[%d].Title = %q, want %q", i, got, wantTitles[i])
		}
		if got := document.Chapters[i].Nodes; !equalInts(got, wantNodes[i]) {
			t.Errorf("Chapters[%d].Nodes = %v, want %v", i, got, wantNodes[i])
		}
	}
}

func TestParseUsesBookChapterPatternOverride(t *testing.T) {
	source := []byte("Chapter 1\n\nOpening\n\nSCENE: The cellar\n\nMiddle\n\nSCENE: The attic\n\nEnding")
	document, err := Parse(source, Options{ChapterPatterns: []string{`^SCENE:\s+.+$`}})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got, want := len(document.Chapters), 3; got != want {
		t.Fatalf("Chapters = %d, want %d", got, want)
	}
	if got := document.Chapters[0].Title; got != "" {
		t.Errorf("default-looking heading became a Chapter title = %q", got)
	}
	if got := document.Chapters[1].Title; got != "SCENE: The cellar" {
		t.Errorf("first custom Chapter title = %q", got)
	}
	if got := document.Chapters[2].Title; got != "SCENE: The attic" {
		t.Errorf("second custom Chapter title = %q", got)
	}
}

func TestParseFallsBackToOneChapterWhenNoHeadingIsDetected(t *testing.T) {
	document, err := Parse([]byte("A paragraph.\n\nAnother paragraph."))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got, want := len(document.Chapters), 1; got != want {
		t.Fatalf("Chapters = %d, want %d", got, want)
	}
	if got, want := document.Chapters[0].Nodes, []int{0, 1}; !equalInts(got, want) {
		t.Errorf("fallback Chapter nodes = %v, want %v", got, want)
	}
}

func TestParseDoesNotTreatNumberedProseAsAChapterHeading(t *testing.T) {
	document, err := Parse([]byte("Opening\n\n1. First, choose a path.\n\nNext prose."))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := len(document.Chapters), 1; got != want {
		t.Fatalf("Chapters = %d, want %d", got, want)
	}
}

func TestIdentitySplicePreservesEveryTXTCorpusMember(t *testing.T) {
	cases := []struct {
		name   string
		source []byte
	}{
		{name: "fixture", source: []byte("\r\nFirst\r\n\r\nSecond\r\n\r\n")},
		{name: "Sherlock Holmes", source: readFile(t, filepath.Join("..", "..", "..", "test_data", "The Adventures of Sherlock Holmes.txt"))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document, err := Parse(tc.source)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			got, err := document.Splice(document.Nodes)
			if err != nil {
				t.Fatalf("Splice: %v", err)
			}
			if !bytes.Equal(got, tc.source) {
				t.Fatalf("identity splice changed the source: got %d bytes, want %d", len(got), len(tc.source))
			}
		})
	}
}

func TestSpliceReplacesTextNodeWithoutChangingSeparators(t *testing.T) {
	source := []byte("First\r\n\r\nSecond\r\n\r\nThird")
	document, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	translations := append([]TextNode(nil), document.Nodes...)
	translations[1].Text = "Translated second"

	got, err := document.Splice(translations)
	if err != nil {
		t.Fatalf("Splice: %v", err)
	}
	if want := []byte("First\r\n\r\nTranslated second\r\n\r\nThird"); !bytes.Equal(got, want) {
		t.Fatalf("spliced source = %q, want %q", got, want)
	}
}

func TestSpliceRejectsMisalignedTranslations(t *testing.T) {
	document, err := Parse([]byte("First\n\nSecond"))
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

func TestHandlerUsesTheFormatNeutralBoundary(t *testing.T) {
	var handler format.Handler = Handler{Options: Options{ChapterPatterns: []string{`^Scene .+$`}}}
	document, err := handler.Parse([]byte("Scene one\n\nText"))
	if err != nil {
		t.Fatalf("Handler.Parse: %v", err)
	}
	if got, want := len(document.TextNodes()), 2; got != want {
		t.Fatalf("TextNodes = %d, want %d", got, want)
	}
	if got, want := len(document.ChaptersList()), 1; got != want {
		t.Fatalf("Chapters = %d, want %d", got, want)
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
