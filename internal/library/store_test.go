package library

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func store(t *testing.T) Store {
	t.Helper()
	return NewStore(t.TempDir())
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestCreatingASeriesWritesItsConfig(t *testing.T) {
	s := store(t)

	series, err := s.CreateSeries("The Adventures of Sherlock Holmes", "en")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}

	if series.Code != "the-adventures-of-sherlock-holmes" {
		t.Errorf("Code = %q", series.Code)
	}
	written := read(t, filepath.Join(s.root, series.Code, SeriesFile))
	for _, want := range []string{"The Adventures of Sherlock Holmes", "en"} {
		if !strings.Contains(written, want) {
			t.Errorf("%s does not carry %q:\n%s", SeriesFile, want, written)
		}
	}
	// The file is one the user may hand-edit, so it explains itself.
	if !strings.Contains(written, "#") {
		t.Errorf("%s has no comments explaining it:\n%s", SeriesFile, written)
	}
}

func TestSeriesRequireCanonicalSourceLanguageTags(t *testing.T) {
	s := store(t)
	for _, source := range []string{"English", "EN", "zz"} {
		if _, err := s.CreateSeries("Solaris", source); err == nil {
			t.Errorf("CreateSeries accepted %q", source)
		}
	}
	if entries, _ := os.ReadDir(s.root); len(entries) != 0 {
		t.Errorf("invalid source languages wrote %v", entries)
	}
}

func TestCreatingATranslationTargetWritesNewState(t *testing.T) {
	s := store(t)
	series, err := s.CreateSeries("Solaris", "pl")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if _, err := s.AddBook(series.Code, BookDraft{Code: "solaris"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}

	target, err := s.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"})
	if err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	if target.Language != "uk" || target.Status != StatusNew {
		t.Errorf("target = %+v, want Ukrainian/New", target)
	}
	state := read(t, filepath.Join(s.root, series.Code, BooksDir, "solaris", TranslationsDir, "pl-to-uk", StateFile))
	if strings.TrimSpace(state) != `{"status":"new"}` {
		t.Errorf("state.json = %q", state)
	}

	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if got := lib.Series[0].Books[0].Targets; len(got) != 1 || got[0] != target {
		t.Errorf("Targets = %+v, want [%+v]", got, target)
	}
}

func TestDictionaryBuildingStateAndDictionaryArePersistedPerTranslationTarget(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	_, _ = s.AddBook(series.Code, BookDraft{Code: "solaris"})
	_, _ = s.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"})

	if err := s.SetTranslationTargetStatus(series.Code, "solaris", "uk", StatusAnalyzing); err != nil {
		t.Fatalf("SetTranslationTargetStatus(Analyzing): %v", err)
	}
	if err := s.WriteDictionary(series.Code, "solaris", "uk", []Term{{Original: "Solaris", Translation: "\u0421\u043e\u043b\u044f\u0440\u0456\u0441", Note: "title"}}); err != nil {
		t.Fatalf("WriteDictionary: %v", err)
	}
	if err := s.SetTranslationTargetStatus(series.Code, "solaris", "uk", StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus(Dictionary Ready): %v", err)
	}

	path := filepath.Join(s.root, series.Code, BooksDir, "solaris", TranslationsDir, "pl-to-uk", DictionaryFile)
	if got, want := read(t, path), "original\ttranslation\tnote\nSolaris\t\u0421\u043e\u043b\u044f\u0440\u0456\u0441\ttitle\n"; got != want {
		t.Errorf("dictionary.tsv = %q, want %q", got, want)
	}
	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if got := lib.Series[0].Books[0].Targets[0].Status; got != StatusDictionaryReady {
		t.Errorf("Status = %q, want %q", got, StatusDictionaryReady)
	}
}

func TestMergedDictionaryLetsTheBookOverrideTheSeries(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	_, _ = s.AddBook(series.Code, BookDraft{Code: "solaris"})
	_, _ = s.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"})
	if err := s.WriteSeriesDictionary(series.Code, "uk", []Term{{Original: "Solaris", Translation: "Солярис"}, {Original: "Rheya", Translation: "Рея"}}); err != nil {
		t.Fatalf("WriteSeriesDictionary: %v", err)
	}
	if err := s.WriteDictionary(series.Code, "solaris", "uk", []Term{{Original: "Rheya", Translation: "Реїя"}, {Original: "Sartorius", Translation: "Сарторіус"}}); err != nil {
		t.Fatalf("WriteDictionary: %v", err)
	}

	got, err := s.Dictionary(series.Code, "solaris", "uk")
	if err != nil {
		t.Fatalf("Dictionary: %v", err)
	}
	want := []Term{{Original: "Solaris", Translation: "Солярис"}, {Original: "Rheya", Translation: "Реїя"}, {Original: "Sartorius", Translation: "Сарторіус"}}
	if !slices.Equal(got, want) {
		t.Errorf("Dictionary = %#v, want %#v", got, want)
	}
}

func TestDictionaryReadsHandEditsAndReportsMalformedLineNumbers(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	_, _ = s.AddBook(series.Code, BookDraft{Code: "solaris"})
	_, _ = s.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"})
	path := filepath.Join(s.root, series.Code, BooksDir, "solaris", TranslationsDir, "pl-to-uk", DictionaryFile)
	if err := os.WriteFile(path, []byte("original\ttranslation\tnote\nSolaris\tСолярис\thand edit\n"), 0o644); err != nil {
		t.Fatalf("write hand edit: %v", err)
	}
	got, err := s.BookDictionary(series.Code, "solaris", "uk")
	if err != nil {
		t.Fatalf("BookDictionary: %v", err)
	}
	if want := []Term{{Original: "Solaris", Translation: "Солярис", Note: "hand edit"}}; !slices.Equal(got, want) {
		t.Errorf("BookDictionary = %#v, want %#v", got, want)
	}
	if err := os.WriteFile(path, []byte("original\ttranslation\tnote\nSolaris\n"), 0o644); err != nil {
		t.Fatalf("write malformed Dictionary: %v", err)
	}
	if _, err := s.BookDictionary(series.Code, "solaris", "uk"); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("BookDictionary error = %v, want line 2", err)
	}
}

func TestPromotingABookTermMakesItAvailableToLaterBooks(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	_, _ = s.AddBook(series.Code, BookDraft{Code: "solaris"})
	_, _ = s.AddBook(series.Code, BookDraft{Code: "solaris-2"})
	_, _ = s.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"})
	_, _ = s.CreateTranslationTarget(series.Code, "solaris-2", "uk", []string{"uk"})
	if err := s.WriteDictionary(series.Code, "solaris", "uk", []Term{{Original: "Ocean", Translation: "Океан"}}); err != nil {
		t.Fatalf("WriteDictionary: %v", err)
	}
	if err := s.PromoteDictionaryTerm(series.Code, "solaris", "uk", "Ocean"); err != nil {
		t.Fatalf("PromoteDictionaryTerm: %v", err)
	}
	got, err := s.Dictionary(series.Code, "solaris-2", "uk")
	if err != nil {
		t.Fatalf("Dictionary: %v", err)
	}
	if want := []Term{{Original: "Ocean", Translation: "Океан"}}; !slices.Equal(got, want) {
		t.Errorf("Dictionary = %#v, want %#v", got, want)
	}
	path := filepath.Join(s.root, series.Code, DictionariesDir, "pl-to-uk.tsv")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Series Dictionary is not at %s: %v", path, err)
	}
}

func TestWritingDictionaryPreservesHandEditsAndAddsNewTerms(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	_, _ = s.AddBook(series.Code, BookDraft{Code: "solaris"})
	_, _ = s.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"})
	if err := s.WriteDictionary(series.Code, "solaris", "uk", []Term{{Original: "Solaris", Translation: "ручне"}}); err != nil {
		t.Fatalf("WriteDictionary: %v", err)
	}
	if err := s.WriteDictionary(series.Code, "solaris", "uk", []Term{{Original: "Solaris", Translation: "запропоноване"}, {Original: "Ocean", Translation: "Океан"}}); err != nil {
		t.Fatalf("WriteDictionary: %v", err)
	}
	got, err := s.BookDictionary(series.Code, "solaris", "uk")
	if err != nil {
		t.Fatalf("BookDictionary: %v", err)
	}
	want := []Term{{Original: "Solaris", Translation: "ручне"}, {Original: "Ocean", Translation: "Океан"}}
	if !slices.Equal(got, want) {
		t.Errorf("BookDictionary = %#v, want %#v", got, want)
	}
}

func TestUploadingASourceFileStoresItsOriginalBytes(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	_, _ = s.AddBook(series.Code, BookDraft{Code: "solaris"})
	supplied := []byte("\xfforiginal\x00text\n")

	if err := s.UploadSourceFile(series.Code, "solaris", "novel.txt", bytes.NewReader(supplied), false); err != nil {
		t.Fatalf("UploadSourceFile: %v", err)
	}

	path := filepath.Join(s.root, series.Code, BooksDir, "solaris", "source.txt")
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stored Source File: %v", err)
	}
	if !bytes.Equal(stored, supplied) {
		t.Errorf("stored bytes = %q, want %q", stored, supplied)
	}
}

func TestUploadingAnUnsupportedSourceFileLeavesTheBookAlone(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	_, _ = s.AddBook(series.Code, BookDraft{Code: "solaris"})
	bookDir := filepath.Join(s.root, series.Code, BooksDir, "solaris")

	err := s.UploadSourceFile(series.Code, "solaris", "novel.epub", strings.NewReader("not an ebook we accept"), false)
	if err == nil || !strings.Contains(err.Error(), ".txt or .fb2") {
		t.Fatalf("UploadSourceFile error = %v, want a clear format rejection", err)
	}
	entries, err := os.ReadDir(bookDir)
	if err != nil {
		t.Fatalf("read Book folder: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != BookFile {
		t.Errorf("unsupported upload wrote %v", entries)
	}
}

func TestReplacingASourceFileNeedsConfirmationAndDiscardsTranslationWork(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	_, _ = s.AddBook(series.Code, BookDraft{Code: "solaris"})
	if err := s.UploadSourceFile(series.Code, "solaris", "old.txt", strings.NewReader("old"), false); err != nil {
		t.Fatalf("initial UploadSourceFile: %v", err)
	}
	if _, err := s.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}

	if err := s.UploadSourceFile(series.Code, "solaris", "replacement.fb2", strings.NewReader("replacement"), false); !errors.Is(err, ErrSourceReplacementNeedsConfirmation) {
		t.Fatalf("unconfirmed replacement error = %v, want confirmation", err)
	}
	if got := read(t, filepath.Join(s.root, series.Code, BooksDir, "solaris", "source.txt")); got != "old" {
		t.Errorf("unconfirmed replacement changed the Source File to %q", got)
	}
	if _, err := os.Stat(filepath.Join(s.root, series.Code, BooksDir, "solaris", TranslationsDir, "pl-to-uk", StateFile)); err != nil {
		t.Errorf("unconfirmed replacement discarded translation work: %v", err)
	}

	if err := s.UploadSourceFile(series.Code, "solaris", "replacement.fb2", strings.NewReader("replacement"), true); err != nil {
		t.Fatalf("confirmed UploadSourceFile: %v", err)
	}
	bookDir := filepath.Join(s.root, series.Code, BooksDir, "solaris")
	if got := read(t, filepath.Join(bookDir, "source.fb2")); got != "replacement" {
		t.Errorf("replacement Source File = %q", got)
	}
	if _, err := os.Stat(filepath.Join(bookDir, "source.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("old Source File survives replacement: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bookDir, TranslationsDir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("translation work survives confirmed replacement: %v", err)
	}
}

func TestTranslationTargetRejectsInvalidPairsWithoutWriting(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	_, _ = s.AddBook(series.Code, BookDraft{Code: "solaris"})
	for _, target := range []string{"pl", "Ukrainian", "zz", "uk"} {
		_, err := s.CreateTranslationTarget(series.Code, "solaris", target, []string{"uk"})
		if target == "uk" && err == nil {
			continue
		}
		if err == nil {
			t.Errorf("CreateTranslationTarget(%q) succeeded", target)
		}
	}
	if _, err := s.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"}); err == nil {
		t.Error("duplicate Translation Target succeeded")
	}
	if _, err := s.CreateTranslationTarget(series.Code, "solaris", "de", []string{"uk"}); err == nil {
		t.Error("disabled Target Language succeeded")
	}
}

func TestMalformedTranslationTargetStateIsAReadError(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	_, _ = s.AddBook(series.Code, BookDraft{Code: "solaris"})
	path := filepath.Join(s.root, series.Code, BooksDir, "solaris", TranslationsDir, "pl-to-uk")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, StateFile), []byte(`{"status":"wat"}`), 0o644); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	if _, err := s.Library(); err == nil || !strings.Contains(err.Error(), "pl-to-uk") {
		t.Errorf("Library error = %v, want target-specific read error", err)
	}
}

// The defaults a Series may override are what makes series.toml worth
// hand-editing, so the written file has to name them.
func TestTheCreatedSeriesConfigNamesTheDefaultsItCanCarry(t *testing.T) {
	s := store(t)
	series, err := s.CreateSeries("Solaris", "pl")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}

	written := read(t, filepath.Join(s.root, series.Code, SeriesFile))
	for _, want := range []string{"agent", "models", "workspace.toml"} {
		if !strings.Contains(written, want) {
			t.Errorf("%s does not mention %q:\n%s", SeriesFile, want, written)
		}
	}
}

func TestASeriesNeedsANameAndASourceLanguage(t *testing.T) {
	s := store(t)

	for name, args := range map[string][2]string{
		"no name":            {"  ", "en"},
		"no source language": {"Solaris", ""},
	} {
		if _, err := s.CreateSeries(args[0], args[1]); err == nil {
			t.Errorf("%s: CreateSeries was accepted", name)
		}
	}
	if entries, _ := os.ReadDir(s.root); len(entries) != 0 {
		t.Errorf("a rejected Series left %d entries behind", len(entries))
	}
}

// The Series code is derived rather than typed, so the user never gets to
// resolve a clash themselves; two Series of the same name must not land in one
// folder.
func TestTwoSeriesOfTheSameNameGetDifferentFolders(t *testing.T) {
	s := store(t)

	first, err := s.CreateSeries("Solaris", "pl")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	second, err := s.CreateSeries("Solaris", "pl")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}

	if first.Code == second.Code {
		t.Fatalf("both Series took the folder %q", first.Code)
	}
	if got := read(t, filepath.Join(s.root, strings.ToLower(second.Code), SeriesFile)); !strings.Contains(got, "Solaris") {
		t.Errorf("the second Series overwrote nothing but carries no name:\n%s", got)
	}
}

func TestAddingABookWritesItsConfigUnderTheSeries(t *testing.T) {
	s := store(t)
	series, err := s.CreateSeries("The Adventures of Sherlock Holmes", "en")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}

	book, err := s.AddBook(series.Code, BookDraft{Code: "memoirs", Title: "The Memoirs of Sherlock Holmes", Author: "Arthur Conan Doyle"})
	if err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	if book.Code != "memoirs" {
		t.Errorf("Code = %q, want %q", book.Code, "memoirs")
	}

	written := read(t, filepath.Join(s.root, series.Code, BooksDir, "memoirs", BookFile))
	for _, want := range []string{"The Memoirs of Sherlock Holmes", "Arthur Conan Doyle"} {
		if !strings.Contains(written, want) {
			t.Errorf("%s does not carry %q:\n%s", BookFile, want, written)
		}
	}
}

func TestABookCodeAlreadyUsedInThatSeriesIsRejected(t *testing.T) {
	s := store(t)
	series, err := s.CreateSeries("Solaris", "pl")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if _, err := s.AddBook(series.Code, BookDraft{Code: "solaris", Title: "Solaris"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}

	_, err = s.AddBook(series.Code, BookDraft{Code: "solaris", Title: "Something Else"})
	if !errors.Is(err, ErrBookCodeTaken) {
		t.Fatalf("got %v, want ErrBookCodeTaken", err)
	}
	if !strings.Contains(err.Error(), "solaris") {
		t.Errorf("the message does not name the code at fault: %v", err)
	}

	// Nothing was written: the Book that was already there is untouched.
	if got := read(t, filepath.Join(s.root, series.Code, BooksDir, "solaris", BookFile)); !strings.Contains(got, "Solaris") {
		t.Errorf("the existing Book was overwritten:\n%s", got)
	}
}

// The same Book Code in a *different* Series is not a clash at all — codes are
// unique within a Series, not across the workspace.
func TestTheSameBookCodeInAnotherSeriesIsFine(t *testing.T) {
	s := store(t)
	first, _ := s.CreateSeries("Solaris", "pl")
	second, _ := s.CreateSeries("Eden", "pl")

	if _, err := s.AddBook(first.Code, BookDraft{Code: "novel", Title: "Solaris"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	if _, err := s.AddBook(second.Code, BookDraft{Code: "novel", Title: "Eden"}); err != nil {
		t.Fatalf("AddBook into a second Series: %v", err)
	}
}

// Two codes differing only in case are two folders on Linux and one on macOS
// and Windows. The library has to mean the same thing on all three, so the
// clash is reported rather than left to the filesystem.
func TestABookCodeDifferingOnlyInCaseIsRejected(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	if _, err := s.AddBook(series.Code, BookDraft{Code: "solaris", Title: "Solaris"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}

	if _, err := s.AddBook(series.Code, BookDraft{Code: "Solaris", Title: "Solaris"}); !errors.Is(err, ErrBookCodeTaken) {
		t.Fatalf("got %v, want ErrBookCodeTaken", err)
	}
}

func TestABookCodeThatCannotNameAFolderIsRejectedBeforeAnythingIsWritten(t *testing.T) {
	s := store(t)
	series, err := s.CreateSeries("Solaris", "pl")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}

	if _, err := s.AddBook(series.Code, BookDraft{Code: "../escape", Title: "Solaris"}); !errors.Is(err, ErrInvalidBookCode) {
		t.Fatalf("got %v, want ErrInvalidBookCode", err)
	}

	// Not even the books directory was created, so a rejection leaves the
	// Series exactly as it was.
	if _, err := os.Stat(filepath.Join(s.root, series.Code, BooksDir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a rejected Book Code created %s: %v", BooksDir, err)
	}
	// And nothing landed outside the Series either.
	if entries, _ := os.ReadDir(s.root); len(entries) != 1 {
		t.Errorf("the workspace root holds %d entries, want 1", len(entries))
	}
}

// A Book is added before its Source File exists, so its title is not the
// user's to supply yet — the Book Code is the only thing they are asked for,
// and the only thing they can be held to.
func TestABookNeedsNothingButItsCode(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")

	book, err := s.AddBook(series.Code, BookDraft{Code: "solaris"})
	if err != nil {
		t.Fatalf("AddBook with no title: %v", err)
	}
	// Until a title is known, the Book Code is what the Book is called.
	if book.Label() != "solaris" {
		t.Errorf("Label() = %q, want the Book Code", book.Label())
	}

	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if got := lib.Series[0].Books[0]; got.Title != "" || got.Label() != "solaris" {
		t.Errorf("read back %+v, want an untitled Book called by its code", got)
	}
}

func TestAddingABookToASeriesThatIsNotThereIsReported(t *testing.T) {
	s := store(t)

	_, err := s.AddBook("nope", BookDraft{Code: "solaris", Title: "Solaris"})
	if !errors.Is(err, ErrSeriesNotFound) {
		t.Fatalf("got %v, want ErrSeriesNotFound", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("the message does not name the Series: %v", err)
	}
}

// A standalone Book is not a second concept: it is a Series of one, on disk
// like any other, so a sequel can be added later without migrating anything.
func TestAStandaloneBookLivesInARealSeriesOfOne(t *testing.T) {
	s := store(t)

	series, book, err := s.AddStandaloneBook(BookDraft{Code: "solaris", Title: "Solaris", Author: "Stanisław Lem"}, "pl")
	if err != nil {
		t.Fatalf("AddStandaloneBook: %v", err)
	}

	if got := read(t, filepath.Join(s.root, series.Code, SeriesFile)); !strings.Contains(got, "pl") {
		t.Errorf("the Series of one carries no source language:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(s.root, series.Code, BooksDir, book.Code, BookFile)); err != nil {
		t.Fatalf("the Book was not written: %v", err)
	}

	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if len(lib.Series) != 1 || len(lib.Series[0].Books) != 1 {
		t.Fatalf("read back %+v, want one Series of one Book", lib.Series)
	}
	if !lib.Series[0].Standalone() {
		t.Error("the Series of one does not render flat")
	}

	// The sequel arrives with no migration: the Series was real all along.
	if _, err := s.AddBook(series.Code, BookDraft{Code: "eden", Title: "Eden"}); err != nil {
		t.Fatalf("AddBook into the Series of one: %v", err)
	}
	lib, err = s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if len(lib.Series) != 1 || len(lib.Series[0].Books) != 2 {
		t.Fatalf("read back %+v, want one Series of two Books", lib.Series)
	}
	if lib.Series[0].Standalone() {
		t.Error("a Series of two still renders flat")
	}
}

// Two standalone Books whose codes match must not share a Series folder, or
// the second would silently join the first's Series.
func TestTwoStandaloneBooksOfTheSameCodeGetTheirOwnSeries(t *testing.T) {
	s := store(t)

	first, _, err := s.AddStandaloneBook(BookDraft{Code: "novel", Title: "Solaris"}, "pl")
	if err != nil {
		t.Fatalf("AddStandaloneBook: %v", err)
	}
	second, _, err := s.AddStandaloneBook(BookDraft{Code: "novel", Title: "Eden"}, "pl")
	if err != nil {
		t.Fatalf("AddStandaloneBook: %v", err)
	}
	if first.Code == second.Code {
		t.Fatalf("both standalone Books took the Series folder %q", first.Code)
	}

	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if len(lib.Series) != 2 {
		t.Fatalf("read back %d Series, want 2", len(lib.Series))
	}
}

// The Series of one is named after the Book Code, and the Book Code is the
// user's to control down to its capitals (story 6). Lowercasing the folder
// would quietly overrule them.
func TestTheSeriesOfOneKeepsTheBookCodeAsTyped(t *testing.T) {
	s := store(t)

	series, book, err := s.AddStandaloneBook(BookDraft{Code: "Solaris", Title: "Solaris"}, "pl")
	if err != nil {
		t.Fatalf("AddStandaloneBook: %v", err)
	}
	if series.Code != "Solaris" || book.Code != "Solaris" {
		t.Errorf("stored as %q/%q, want both as typed", series.Code, book.Code)
	}
	if _, err := os.Stat(filepath.Join(s.root, "Solaris", BooksDir, "Solaris", BookFile)); err != nil {
		t.Errorf("the Book is not where the user would look for it: %v", err)
	}
}

// A standalone Book is added without being asked about Series, so its Book
// Code is still the only name it needs to be valid.
func TestAStandaloneBookWithAnUnusableCodeIsRejectedBeforeAnythingIsWritten(t *testing.T) {
	s := store(t)

	if _, _, err := s.AddStandaloneBook(BookDraft{Code: "no/slashes", Title: "Solaris"}, "pl"); !errors.Is(err, ErrInvalidBookCode) {
		t.Fatalf("got %v, want ErrInvalidBookCode", err)
	}
	if entries, _ := os.ReadDir(s.root); len(entries) != 0 {
		t.Errorf("a rejected standalone Book left %d entries behind", len(entries))
	}
}

func TestAnEmptyWorkspaceReadsAsAnEmptyLibrary(t *testing.T) {
	lib, err := store(t).Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if len(lib.Series) != 0 {
		t.Errorf("read %d Series out of an empty workspace", len(lib.Series))
	}
}

func TestTheLibraryIsReadInAStableOrder(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Holmes", "en")
	for _, code := range []string{"return", "adventures", "memoirs"} {
		if _, err := s.AddBook(series.Code, BookDraft{Code: code, Title: code}); err != nil {
			t.Fatalf("AddBook %s: %v", code, err)
		}
	}
	if _, err := s.CreateSeries("Anthology", "en"); err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}

	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if got := []string{lib.Series[0].Code, lib.Series[1].Code}; got[0] != "anthology" || got[1] != "holmes" {
		t.Errorf("Series read as %v, want them sorted by code", got)
	}
	var books []string
	for _, b := range lib.Series[1].Books {
		books = append(books, b.Code)
	}
	if strings.Join(books, ",") != "adventures,memoirs,return" {
		t.Errorf("Books read as %v, want them sorted by code", books)
	}
}

// The workspace root also holds workspace.toml, and the user may well leave
// notes of their own beside it. Neither is a Series.
func TestFilesAndFoldersThatAreNotSeriesAreIgnored(t *testing.T) {
	s := store(t)
	if _, err := s.CreateSeries("Solaris", "pl"); err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.root, "workspace.toml"), []byte("agent = \"claude\"\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(s.root, "notes", "deeper"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}

	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if len(lib.Series) != 1 || lib.Series[0].Code != "solaris" {
		t.Errorf("read %+v, want only the Series", lib.Series)
	}
}

// Hand-editing is the point of TOML, and a typo has to be reported rather than
// quietly dropping a Series the user can see on disk.
func TestASeriesConfigThatCannotBeParsedIsReported(t *testing.T) {
	s := store(t)
	dir := filepath.Join(s.root, "broken")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, SeriesFile), []byte("name = = nope"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := s.Library()
	if err == nil {
		t.Fatal("a broken series.toml read as an empty Library")
	}
	if !strings.Contains(err.Error(), SeriesFile) || !strings.Contains(err.Error(), "broken") {
		t.Errorf("the error does not say which file is at fault: %v", err)
	}
}

// The workspace can go away while the application is running — an unmounted
// drive, a folder renamed in Finder. Reading it is where that shows up.
func TestReadingAWorkspaceThatIsNoLongerThereIsReported(t *testing.T) {
	root := filepath.Join(t.TempDir(), "gone")
	if _, err := NewStore(root).Library(); err == nil {
		t.Fatal("a missing workspace root read as an empty Library")
	}
}

func TestABookFolderWithNoConfigIsNotABook(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Solaris", "pl")
	if err := os.MkdirAll(filepath.Join(s.root, series.Code, BooksDir, "half-made"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}

	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if len(lib.Series[0].Books) != 0 {
		t.Errorf("read %+v, want no Books", lib.Series[0].Books)
	}
}

// A name and a title are prose the user typed, so they may hold anything TOML
// gives meaning to. What is written has to read back as what was typed.
func TestNamesHoldingQuotesAndBackslashesReadBackUnchanged(t *testing.T) {
	s := store(t)
	seriesName := `"Quoted" \ Series`
	series, err := s.CreateSeries(seriesName, "en")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	title := "The \"Adventures\" of C:\\Sherlock\tHolmes"
	author := `O'Brien "Jr"`
	if _, err := s.AddBook(series.Code, BookDraft{Code: "odd", Title: title, Author: author}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}

	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if got := lib.Series[0].Name; got != seriesName {
		t.Errorf("Series Name read back as %q, want %q", got, seriesName)
	}
	book := lib.Series[0].Books[0]
	if book.Title != title {
		t.Errorf("Title read back as %q, want %q", book.Title, title)
	}
	if book.Author != author {
		t.Errorf("Author read back as %q, want %q", book.Author, author)
	}
}

func TestABookIsFoundBySeriesAndCode(t *testing.T) {
	s := store(t)
	series, _ := s.CreateSeries("Holmes", "en")
	if _, err := s.AddBook(series.Code, BookDraft{Code: "memoirs", Title: "The Memoirs of Sherlock Holmes"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	lib, err := s.Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}

	found, book, ok := lib.Book(series.Code, "memoirs")
	if !ok {
		t.Fatal("the Book was not found")
	}
	if found.SourceLanguage != "en" || book.Title != "The Memoirs of Sherlock Holmes" {
		t.Errorf("found %+v / %+v", found, book)
	}
	if _, _, ok := lib.Book(series.Code, "nope"); ok {
		t.Error("a Book that is not there was found")
	}
}
