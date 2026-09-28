package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/workspace"
)

func bookWithConfig(t *testing.T, override string) (Store, string) {
	t.Helper()
	s := store(t)
	series, err := s.CreateSeries("Solaris", "pl")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if _, err := s.AddBook(series.Code, BookDraft{Code: "solaris", Title: "Solaris"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	path := filepath.Join(s.root, series.Code, BooksDir, "solaris", BookFile)
	if err := os.WriteFile(path, []byte(read(t, path)+override), 0o644); err != nil {
		t.Fatalf("edit book.toml: %v", err)
	}
	return s, series.Code
}

func TestABookReadsItsAgentAndModelOverride(t *testing.T) {
	s, seriesCode := bookWithConfig(t, "\nagent = \"claude\"\n\n[models]\ntranslation = \"claude-opus-best\"\n")

	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	_, book, _ := lib.Book(seriesCode, "solaris")
	want := workspace.AgentSettings{Agent: "claude", Models: workspace.Models{Translation: "claude-opus-best"}}
	if book.AgentOverride != want {
		t.Errorf("AgentOverride = %+v, want %+v", book.AgentOverride, want)
	}
}

func TestABookWithoutAnOverrideInheritsEverything(t *testing.T) {
	s, seriesCode := bookWithConfig(t, "")

	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	_, book, _ := lib.Book(seriesCode, "solaris")
	if book.AgentOverride != (workspace.AgentSettings{}) {
		t.Errorf("AgentOverride = %+v, want nothing overridden", book.AgentOverride)
	}
}

func TestABookNamingAnUnknownAgentIsRejectedOnLoad(t *testing.T) {
	s, _ := bookWithConfig(t, "\nagent = \"gpt\"\n")

	_, err := s.Library()
	if err == nil {
		t.Fatal("Library accepted an unknown Agent")
	}
	if !strings.Contains(err.Error(), `"gpt"`) || !strings.Contains(err.Error(), BookFile) {
		t.Errorf("error = %v, want the Agent and book.toml named", err)
	}
}

func TestEditingMetadataKeepsTheAgentAndModelOverride(t *testing.T) {
	s, seriesCode := bookWithConfig(t, "\nagent = \"claude\"\n\n[models]\ntranslation = \"claude-opus-best\"\n")

	if err := s.UpdateBookMetadata(seriesCode, "solaris", BookMetadata{Title: "Solaris, revised"}); err != nil {
		t.Fatalf("UpdateBookMetadata: %v", err)
	}
	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	_, book, _ := lib.Book(seriesCode, "solaris")
	if book.Title != "Solaris, revised" || book.AgentOverride.Models.Translation != "claude-opus-best" {
		t.Errorf("Book = %+v, want the new title and the kept override", book)
	}
}

// A hand-written book.toml may lack a metadata key and end in a [models]
// table. A key added back must stay top-level rather than land inside it.
func TestAMissingMetadataKeyIsNotWrittenIntoTheModelsTable(t *testing.T) {
	s := store(t)
	series, err := s.CreateSeries("Solaris", "pl")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if _, err := s.AddBook(series.Code, BookDraft{Code: "solaris"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	path := filepath.Join(s.root, series.Code, BooksDir, "solaris", BookFile)
	if err := os.WriteFile(path, []byte("title = \"Solaris\"\n\n[models]\ntranslation = \"claude-opus-best\"\n"), 0o644); err != nil {
		t.Fatalf("write book.toml: %v", err)
	}

	if err := s.UpdateBookMetadata(series.Code, "solaris", BookMetadata{Title: "Solaris", Author: "Lem"}); err != nil {
		t.Fatalf("UpdateBookMetadata: %v", err)
	}
	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v\n%s", err, read(t, path))
	}
	_, book, _ := lib.Book(series.Code, "solaris")
	if book.Author != "Lem" || book.AgentOverride.Models.Translation != "claude-opus-best" {
		t.Errorf("Book = %+v, want the author top-level and the override kept:\n%s", book, read(t, path))
	}
}
