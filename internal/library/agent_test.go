package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/agent"
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

func loadBook(t *testing.T, s Store, seriesCode string) Book {
	t.Helper()
	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	_, book, ok := lib.Book(seriesCode, "solaris")
	if !ok {
		t.Fatal("Book not found")
	}
	return book
}

func TestABookReadsItsOverridePerTask(t *testing.T) {
	s, seriesCode := bookWithConfig(t, "\n[translation]\nmodel = \"claude-opus-best\"\neffort = \"max\"\n")

	book := loadBook(t, s, seriesCode)
	want := workspace.Choices{Translation: workspace.Choice{Model: "claude-opus-best", Effort: agent.EffortMax}}
	if book.Override != want {
		t.Errorf("Override = %+v, want %+v", book.Override, want)
	}
}

func TestABookWithoutAnOverrideInheritsEverything(t *testing.T) {
	s, seriesCode := bookWithConfig(t, "")

	if book := loadBook(t, s, seriesCode); book.Override != (workspace.Choices{}) {
		t.Errorf("Override = %+v, want nothing overridden", book.Override)
	}
}

func TestALegacyAgentAndModelsOverrideIsStillRead(t *testing.T) {
	s, seriesCode := bookWithConfig(t, "\nagent = \"claude\"\n\n[models]\ntranslation = \"claude-opus-best\"\n")

	book := loadBook(t, s, seriesCode)
	want := workspace.Choices{
		Mechanical:  workspace.Choice{Agent: "claude"},
		Translation: workspace.Choice{Agent: "claude", Model: "claude-opus-best"},
	}
	if book.Override != want {
		t.Errorf("Override = %+v, want %+v", book.Override, want)
	}
}

func TestABookNamingAnUnknownAgentOrEffortIsRejectedOnLoad(t *testing.T) {
	for override, want := range map[string]string{
		"\n[translation]\nagent = \"gpt\"\n":     `"gpt"`,
		"\n[mechanical]\neffort = \"extreme\"\n": `"extreme"`,
		"\nagent = \"gpt\"\n":                    `"gpt"`,
	} {
		s, _ := bookWithConfig(t, override)
		_, err := s.Library()
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), BookFile) {
			t.Errorf("Library with %q = %v, want %s and book.toml named", override, err, want)
		}
	}
}

func TestEditingMetadataKeepsTheOverride(t *testing.T) {
	s, seriesCode := bookWithConfig(t, "\n[translation]\nmodel = \"claude-opus-best\"\n")

	if err := s.UpdateBookMetadata(seriesCode, "solaris", BookMetadata{Title: "Solaris, revised"}); err != nil {
		t.Fatalf("UpdateBookMetadata: %v", err)
	}
	book := loadBook(t, s, seriesCode)
	if book.Title != "Solaris, revised" || book.Override.Translation.Model != "claude-opus-best" {
		t.Errorf("Book = %+v, want the new title and the kept override", book)
	}
}

func TestSettingABookChoiceWritesOnlyThatTask(t *testing.T) {
	s, seriesCode := bookWithConfig(t, "\n# my own note\n[mechanical]\neffort = \"high\"\n")

	choice := workspace.Choice{Agent: "claude", Model: "claude-fable-5-1", Effort: agent.EffortXHigh}
	if err := s.SetBookChoice(seriesCode, "solaris", workspace.TaskTranslation, choice); err != nil {
		t.Fatalf("SetBookChoice: %v", err)
	}
	book := loadBook(t, s, seriesCode)
	if book.Override.Translation != choice || book.Override.Mechanical != (workspace.Choice{Effort: agent.EffortHigh}) {
		t.Errorf("Override = %+v, want the new translation and the untouched mechanical", book.Override)
	}
	if book.Title != "Solaris" {
		t.Errorf("Title = %q, want metadata untouched", book.Title)
	}
	if written := read(t, filepath.Join(s.root, seriesCode, BooksDir, "solaris", BookFile)); !strings.Contains(written, "# my own note") {
		t.Errorf("book.toml lost its comment:\n%s", written)
	}
}

func TestAnEmptyFieldGoesBackToInheriting(t *testing.T) {
	s, seriesCode := bookWithConfig(t, "\n[translation]\nagent = \"claude\"\nmodel = \"claude-fable-5-1\"\neffort = \"max\"\n")

	if err := s.SetBookChoice(seriesCode, "solaris", workspace.TaskTranslation, workspace.Choice{Effort: agent.EffortMax}); err != nil {
		t.Fatalf("SetBookChoice: %v", err)
	}
	if got := loadBook(t, s, seriesCode).Override.Translation; got != (workspace.Choice{Effort: agent.EffortMax}) {
		t.Errorf("translation override = %+v, want only the Effort kept", got)
	}
}

// Left in place, a legacy entry would quietly keep applying to a field the
// user has just set back to inheriting — while the other Task, which the user
// did not touch, must keep what the legacy entry gave it.
func TestSettingAChoiceRetiresTheLegacyForm(t *testing.T) {
	s, seriesCode := bookWithConfig(t, "\nagent = \"claude\"\n\n[models]\nmechanical = \"claude-sonnet-5\"\ntranslation = \"claude-opus-best\"\n")

	if err := s.SetBookChoice(seriesCode, "solaris", workspace.TaskTranslation, workspace.Choice{}); err != nil {
		t.Fatalf("SetBookChoice: %v", err)
	}
	book := loadBook(t, s, seriesCode)
	if book.Override.Translation != (workspace.Choice{}) {
		t.Errorf("translation override = %+v, want it inheriting", book.Override.Translation)
	}
	if want := (workspace.Choice{Agent: "claude", Model: "claude-sonnet-5"}); book.Override.Mechanical != want {
		t.Errorf("mechanical override = %+v, want %+v kept", book.Override.Mechanical, want)
	}
}

func TestAnInvalidBookChoiceIsNotSaved(t *testing.T) {
	s, seriesCode := bookWithConfig(t, "")
	path := filepath.Join(s.root, seriesCode, BooksDir, "solaris", BookFile)
	before := read(t, path)

	for _, choice := range []workspace.Choice{{Agent: "gpt"}, {Effort: "extreme"}} {
		if err := s.SetBookChoice(seriesCode, "solaris", workspace.TaskMechanical, choice); err == nil {
			t.Errorf("SetBookChoice(%+v) succeeded", choice)
		}
	}
	if read(t, path) != before {
		t.Error("a rejected Choice rewrote book.toml")
	}
}

// A hand-written book.toml may lack a metadata key and end in a table. A key
// added back must stay top-level rather than land inside it.
func TestAMissingMetadataKeyIsNotWrittenIntoATable(t *testing.T) {
	s := store(t)
	series, err := s.CreateSeries("Solaris", "pl")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if _, err := s.AddBook(series.Code, BookDraft{Code: "solaris"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	path := filepath.Join(s.root, series.Code, BooksDir, "solaris", BookFile)
	if err := os.WriteFile(path, []byte("title = \"Solaris\"\n\n[translation]\nmodel = \"claude-opus-best\"\n"), 0o644); err != nil {
		t.Fatalf("write book.toml: %v", err)
	}

	if err := s.UpdateBookMetadata(series.Code, "solaris", BookMetadata{Title: "Solaris", Author: "Lem"}); err != nil {
		t.Fatalf("UpdateBookMetadata: %v", err)
	}
	book := loadBook(t, s, series.Code)
	if book.Author != "Lem" || book.Override.Translation.Model != "claude-opus-best" {
		t.Errorf("Book = %+v, want the author top-level and the override kept:\n%s", book, read(t, path))
	}
}
