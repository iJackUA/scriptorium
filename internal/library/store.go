package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/ijackua/scriptorium/internal/workspace"
)

// The names of the files and folders a Library is made of. They are exported
// because they are part of the workspace's contract with the user: the whole
// point of plain files is that these names are the ones they see on disk.
const (
	// SeriesFile holds a Series' name, source language and defaults.
	SeriesFile = "series.toml"
	// BooksDir holds one folder per Book, named by its Book Code.
	BooksDir = "books"
	// BookFile holds a Book's metadata.
	BookFile = "book.toml"
	// TranslationsDir holds a Book's Translation Targets.
	TranslationsDir = "translations"
	// StateFile holds a Translation Target's machine Status.
	StateFile = "state.json"
	// DictionaryFile holds the proposed Dictionary for one Translation Target.
	DictionaryFile = "dictionary.tsv"
	// DictionariesDir holds the per-language-pair Dictionaries shared by a Series.
	DictionariesDir = "dictionaries"
	// SourceFilePrefix is the filename before a Source File's extension.
	SourceFilePrefix = "source"
	// ChunksDir is the Book-level Chunk Materialization directory.
	ChunksDir = "chunks"
)

var (
	// ErrBookCodeTaken reports a Book Code already used in that Series.
	ErrBookCodeTaken = errors.New("that Book Code is already used in this Series")
	// ErrSeriesNotFound reports a Series that is not in the workspace.
	ErrSeriesNotFound          = errors.New("that Series is not in this workspace")
	ErrTranslationTargetExists = errors.New("that Translation Target already exists")
	// ErrSourceReplacementNeedsConfirmation prevents a stray upload from
	// discarding the Translation Targets that belong to the old Source File.
	ErrSourceReplacementNeedsConfirmation = errors.New("replacing the Source File discards all existing translation work for this Book; confirm the replacement to continue")
)

// Store reads and writes the Series and Books of a Library as plain files
// under a workspace root.
//
// Nothing is cached. The library is files the user may back up, sync or edit
// by hand while the application is open, so every read goes to disk — which is
// also what makes a workspace that has gone away with its drive show up as an
// error the user can be told about rather than a stale screen.
type Store struct {
	root string
}

// NewStore reads the Library under the workspace rooted at root.
func NewStore(root string) Store { return Store{root: root} }

// BookDraft is the details supplied when adding a Book.
//
// Only the Code is required, because it is the only one the user is in a
// position to give: a Book is added before its Source File exists, and the
// title and author come out of that file — read from an fb2's description, or
// inferred from a txt's opening pages. Either may be typed here by a user who
// already knows them, and either may be corrected later.
type BookDraft struct {
	Code   string
	Title  string
	Author string
}

// Library reads every Series and Book under the workspace root.
func (s Store) Library() (Library, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return Library{}, fmt.Errorf("read the workspace folder: %w", err)
	}

	lib := Library{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// A folder without a series.toml is not a Series. The root holds
		// whatever else the user keeps there, and none of it is our business.
		path := filepath.Join(s.root, entry.Name(), SeriesFile)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		}
		series, err := s.series(entry.Name())
		if err != nil {
			return Library{}, err
		}
		lib.Series = append(lib.Series, series)
	}

	// Read order is the filesystem's business, and the library list's order is
	// not allowed to depend on it.
	slices.SortFunc(lib.Series, func(a, b Series) int { return strings.Compare(a.Code, b.Code) })
	return lib, nil
}

// seriesFile is the on-disk shape of series.toml.
//
// The defaults a Series may override are documented in the file rather than
// modelled here: the ticket that reads them is the one that adds the fields,
// and until then decoding them would be a promise nothing keeps.
type seriesFile struct {
	Name           string `toml:"name"`
	SourceLanguage string `toml:"source_language"`
}

// bookFile is the on-disk shape of book.toml.
type bookFile struct {
	Title                    string `toml:"title"`
	Author                   string `toml:"author"`
	SourceFileLanguage       string `toml:"source_file_language"`
	TitleEdited              bool   `toml:"title_edited"`
	AuthorEdited             bool   `toml:"author_edited"`
	SourceFileLanguageEdited bool   `toml:"source_file_language_edited"`
}

// BookMetadata is the editable detail set shown on a Book's details page.
type BookMetadata struct {
	Title              string
	Author             string
	SourceFileLanguage string
}

// series reads one Series and the Books under it.
func (s Store) series(code string) (Series, error) {
	path := filepath.Join(s.root, code, SeriesFile)
	var file seriesFile
	if _, err := toml.DecodeFile(path, &file); err != nil {
		return Series{}, fmt.Errorf("read %s: %w", filepath.Join(code, SeriesFile), err)
	}
	if _, ok := workspace.LanguageFor(file.SourceLanguage); !ok {
		return Series{}, fmt.Errorf("read %s: %q is not a canonical ISO 639-1 source language tag", filepath.Join(code, SeriesFile), file.SourceLanguage)
	}

	books, err := s.books(code, file.SourceLanguage)
	if err != nil {
		return Series{}, err
	}
	return Series{
		Code:           code,
		Name:           file.Name,
		SourceLanguage: file.SourceLanguage,
		Books:          books,
	}, nil
}

// books reads the Books of one Series.
func (s Store) books(seriesCode, sourceLanguage string) ([]Book, error) {
	dir := filepath.Join(s.root, seriesCode, BooksDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		// A Series with no Books yet is an ordinary Series, not a broken one.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Join(seriesCode, BooksDir), err)
	}

	var books []Book
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rel := filepath.Join(seriesCode, BooksDir, entry.Name(), BookFile)
		var file bookFile
		if _, err := toml.DecodeFile(filepath.Join(s.root, rel), &file); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		targets, err := s.translationTargets(seriesCode, entry.Name(), sourceLanguage)
		if err != nil {
			return nil, err
		}
		source, err := sourceFile(filepath.Join(s.root, seriesCode, BooksDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read Book %s: %w", entry.Name(), err)
		}
		books = append(books, Book{Code: entry.Name(), Title: file.Title, Author: file.Author, SourceFileLanguage: file.SourceFileLanguage, SourceFile: source, Targets: targets})
	}
	slices.SortFunc(books, func(a, b Book) int { return strings.Compare(a.Code, b.Code) })
	return books, nil
}

// CreateSeries creates a Series with a name and a source language.
//
// The folder it is stored under is derived from the name rather than asked
// for; see seriesCodeFor.
func (s Store) CreateSeries(name, sourceLanguage string) (Series, error) {
	name, sourceLanguage = strings.TrimSpace(name), strings.TrimSpace(sourceLanguage)
	if name == "" {
		return Series{}, errors.New("a Series needs a name")
	}
	if sourceLanguage == "" {
		return Series{}, errors.New("a Series needs a source language, so its Books know what they are being translated from")
	}
	if _, ok := workspace.LanguageFor(sourceLanguage); !ok {
		return Series{}, fmt.Errorf("%q is not a canonical ISO 639-1 source language tag", sourceLanguage)
	}

	code, err := s.unusedSeriesCode(seriesCodeFor(name))
	if err != nil {
		return Series{}, err
	}
	if err := s.writeSeries(code, name, sourceLanguage); err != nil {
		return Series{}, err
	}
	return Series{Code: code, Name: name, SourceLanguage: sourceLanguage}, nil
}

// AddBook adds a Book to an existing Series.
//
// Every reason to refuse is found before the first byte is written, so a
// rejected Book leaves the Series exactly as it was rather than half made.
func (s Store) AddBook(seriesCode string, draft BookDraft) (Book, error) {
	book, err := s.validatedBook(seriesCode, draft)
	if err != nil {
		return Book{}, err
	}
	if err := s.writeBook(seriesCode, book); err != nil {
		return Book{}, err
	}
	return book, nil
}

// AddStandaloneBook adds a Book without the user having been asked about
// Series, by creating the Series of one it belongs to.
//
// It is a real Series on disk, identical in every way to one created by hand,
// so the sequel that turns up two years later is an ordinary AddBook rather
// than a migration.
func (s Store) AddStandaloneBook(draft BookDraft, sourceLanguage string) (Series, Book, error) {
	sourceLanguage = strings.TrimSpace(sourceLanguage)
	if sourceLanguage == "" {
		return Series{}, Book{}, errors.New("a Book needs a source language, so it is known what it is being translated from")
	}
	if _, ok := workspace.LanguageFor(sourceLanguage); !ok {
		return Series{}, Book{}, fmt.Errorf("%q is not a canonical ISO 639-1 source language tag", sourceLanguage)
	}
	book, err := validatedDraft(draft)
	if err != nil {
		return Series{}, Book{}, err
	}

	// The Series takes its name from the Book, which is what makes it
	// invisible: a Series of one renders without a group header, so the name
	// only ever shows once the user adds a second Book and can rename it.
	code, err := s.unusedSeriesCode(book.Code)
	if err != nil {
		return Series{}, Book{}, err
	}
	if err := s.writeSeries(code, book.Title, sourceLanguage); err != nil {
		return Series{}, Book{}, err
	}
	if err := s.writeBook(code, book); err != nil {
		return Series{}, Book{}, err
	}
	return Series{Code: code, Name: book.Title, SourceLanguage: sourceLanguage, Books: []Book{book}}, book, nil
}

// CreateTranslationTarget creates the durable identity and initial Status for
// one target language of a Book. The temporary directory makes the pair appear
// only once its complete state.json exists.
func (s Store) CreateTranslationTarget(seriesCode, bookCode, targetLanguage string, allowed []string) (TranslationTarget, error) {
	series, err := s.series(seriesCode)
	if err != nil {
		return TranslationTarget{}, err
	}
	if _, _, ok := (Library{Series: []Series{series}}).Book(seriesCode, bookCode); !ok {
		return TranslationTarget{}, fmt.Errorf("Book %q is not in Series %q", bookCode, seriesCode)
	}
	if _, ok := workspace.LanguageFor(targetLanguage); !ok {
		return TranslationTarget{}, fmt.Errorf("%q is not a canonical ISO 639-1 target language tag", targetLanguage)
	}
	if !slices.Contains(allowed, targetLanguage) {
		return TranslationTarget{}, errors.New("that Target Language is not enabled in Workspace settings")
	}
	if targetLanguage == series.SourceLanguage {
		return TranslationTarget{}, errors.New("a Translation Target must differ from the Source Language")
	}
	pair := languagePair(series.SourceLanguage, targetLanguage)
	destination := filepath.Join(s.root, seriesCode, BooksDir, bookCode, TranslationsDir, pair)
	if _, err := os.Stat(destination); err == nil {
		return TranslationTarget{}, fmt.Errorf("%w: %s", ErrTranslationTargetExists, pair)
	} else if !errors.Is(err, os.ErrNotExist) {
		return TranslationTarget{}, fmt.Errorf("read %s: %w", filepath.Join(TranslationsDir, pair), err)
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return TranslationTarget{}, fmt.Errorf("create %s: %w", TranslationsDir, err)
	}
	temporary, err := os.MkdirTemp(parent, ".target-*")
	if err != nil {
		return TranslationTarget{}, fmt.Errorf("create Translation Target: %w", err)
	}
	defer os.RemoveAll(temporary)
	state, err := json.Marshal(struct {
		Status string `json:"status"`
	}{Status: "new"})
	if err != nil {
		return TranslationTarget{}, fmt.Errorf("encode %s: %w", StateFile, err)
	}
	if err := os.WriteFile(filepath.Join(temporary, StateFile), state, 0o644); err != nil {
		return TranslationTarget{}, fmt.Errorf("write %s: %w", StateFile, err)
	}
	if err := os.Rename(temporary, destination); err != nil {
		return TranslationTarget{}, fmt.Errorf("create Translation Target: %w", err)
	}
	return TranslationTarget{Language: targetLanguage, Status: StatusNew}, nil
}

// UploadSourceFile stores a Book's supplied Source File without interpreting
// or changing its bytes. Replacing one requires explicit confirmation because
// every Translation Target was made from the old Source File.
func (s Store) UploadSourceFile(seriesCode, bookCode, filename string, source io.Reader, confirmed bool) error {
	if _, _, ok := s.book(seriesCode, bookCode); !ok {
		return fmt.Errorf("Book %q is not in Series %q", bookCode, seriesCode)
	}
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if extension != ".txt" && extension != ".fb2" {
		return errors.New("a Source File must be a .txt or .fb2 file")
	}

	bookDir := filepath.Join(s.root, seriesCode, BooksDir, bookCode)
	existing, err := sourceFile(bookDir)
	if err != nil {
		return err
	}
	if existing != "" && !confirmed {
		return ErrSourceReplacementNeedsConfirmation
	}

	temporary, err := os.CreateTemp(bookDir, ".source-*")
	if err != nil {
		return fmt.Errorf("prepare Source File: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := io.Copy(temporary, source); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("store Source File: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("store Source File: %w", err)
	}

	if existing != "" {
		if err := os.RemoveAll(filepath.Join(bookDir, TranslationsDir)); err != nil {
			return fmt.Errorf("discard translation work: %w", err)
		}
		if err := os.RemoveAll(filepath.Join(bookDir, ChunksDir)); err != nil {
			return fmt.Errorf("discard Chunk Materialization: %w", err)
		}
		if existing != SourceFilePrefix+extension {
			if err := os.Remove(filepath.Join(bookDir, existing)); err != nil {
				return fmt.Errorf("replace Source File: %w", err)
			}
		}
	}
	if err := os.Rename(temporaryName, filepath.Join(bookDir, SourceFilePrefix+extension)); err != nil {
		return fmt.Errorf("store Source File: %w", err)
	}
	return nil
}

// FillBookMetadata records fields obtained from a Source File without
// replacing fields a person has corrected in the details form.
func (s Store) FillBookMetadata(seriesCode, bookCode string, fields BookMetadata) error {
	path, file, err := s.bookFile(seriesCode, bookCode)
	if err != nil {
		return err
	}
	if !file.TitleEdited {
		file.Title = strings.TrimSpace(fields.Title)
	}
	if !file.AuthorEdited {
		file.Author = strings.TrimSpace(fields.Author)
	}
	if !file.SourceFileLanguageEdited {
		file.SourceFileLanguage = strings.TrimSpace(fields.SourceFileLanguage)
	}
	return writeBookFile(path, file)
}

// UpdateBookMetadata records the values explicitly submitted from the details
// form. All three fields become authoritative, including deliberately blank
// values, so later source parsing cannot undo a correction.
func (s Store) UpdateBookMetadata(seriesCode, bookCode string, fields BookMetadata) error {
	path, file, err := s.bookFile(seriesCode, bookCode)
	if err != nil {
		return err
	}
	title, author, sourceFileLanguage := strings.TrimSpace(fields.Title), strings.TrimSpace(fields.Author), strings.TrimSpace(fields.SourceFileLanguage)
	file.TitleEdited = file.TitleEdited || title != file.Title
	file.AuthorEdited = file.AuthorEdited || author != file.Author
	file.SourceFileLanguageEdited = file.SourceFileLanguageEdited || sourceFileLanguage != file.SourceFileLanguage
	file.Title, file.Author, file.SourceFileLanguage = title, author, sourceFileLanguage
	return writeBookFile(path, file)
}

// SourceFile reads a Book's Source File and returns its on-disk name. A Book
// without one cannot begin Dictionary Building or translation.
func (s Store) SourceFile(seriesCode, bookCode string) ([]byte, string, error) {
	if _, _, ok := s.book(seriesCode, bookCode); !ok {
		return nil, "", fmt.Errorf("Book %q is not in Series %q", bookCode, seriesCode)
	}
	bookDir := filepath.Join(s.root, seriesCode, BooksDir, bookCode)
	name, err := sourceFile(bookDir)
	if err != nil {
		return nil, "", err
	}
	if name == "" {
		return nil, "", errors.New("upload a Source File before starting Dictionary Building")
	}
	body, err := os.ReadFile(filepath.Join(bookDir, name))
	if err != nil {
		return nil, "", fmt.Errorf("read Source File: %w", err)
	}
	return body, name, nil
}

// SetTranslationTargetStatus changes the persisted Status for one existing
// Translation Target. The temporary file keeps readers from observing a
// partially-written state while a long-running operation is in flight.
func (s Store) SetTranslationTargetStatus(seriesCode, bookCode, targetLanguage string, status Status) error {
	series, err := s.series(seriesCode)
	if err != nil {
		return err
	}
	if _, _, ok := (Library{Series: []Series{series}}).Book(seriesCode, bookCode); !ok {
		return fmt.Errorf("Book %q is not in Series %q", bookCode, seriesCode)
	}
	if _, ok := statusFor(strings.ToLower(strings.ReplaceAll(string(status), " ", "_"))); !ok {
		return fmt.Errorf("unknown Status %q", status)
	}
	path := filepath.Join(s.root, seriesCode, BooksDir, bookCode, TranslationsDir, languagePair(series.SourceLanguage, targetLanguage), StateFile)
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("Translation Target %q does not exist", targetLanguage)
		}
		return fmt.Errorf("read %s: %w", StateFile, err)
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(body, &state); err != nil {
		return fmt.Errorf("decode %s: %w", StateFile, err)
	}
	if state == nil {
		return fmt.Errorf("decode %s: expected an object", StateFile)
	}
	stateBody, err := json.MarshalIndent(stateWithStatus(state, strings.ToLower(strings.ReplaceAll(string(status), " ", "_"))), "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", StateFile, err)
	}
	return writeAtomic(path, stateBody)
}

func stateWithStatus(state map[string]json.RawMessage, status string) map[string]json.RawMessage {
	state["status"] = json.RawMessage(`"` + status + `"`)
	return state
}

// WriteDictionary extends a Book Dictionary with proposed Terms without
// replacing its existing, user-reviewed Terms.
func (s Store) WriteDictionary(seriesCode, bookCode, targetLanguage string, terms []Term) error {
	path, err := s.checkedBookDictionaryPath(seriesCode, bookCode, targetLanguage)
	if err != nil {
		return err
	}
	existing, err := readDictionary(path)
	if err != nil {
		return err
	}
	return writeDictionary(path, mergeTerms(terms, existing))
}

// BookDictionary reads a Translation Target's Dictionary directly from disk,
// so edits made in another editor are visible on the next application read.
func (s Store) BookDictionary(seriesCode, bookCode, targetLanguage string) ([]Term, error) {
	path, err := s.checkedBookDictionaryPath(seriesCode, bookCode, targetLanguage)
	if err != nil {
		return nil, err
	}
	return readDictionary(path)
}

// BookDictionaryTSV returns the hand-editable TSV exactly as it is stored.
func (s Store) BookDictionaryTSV(seriesCode, bookCode, targetLanguage string) ([]byte, error) {
	path, err := s.checkedBookDictionaryPath(seriesCode, bookCode, targetLanguage)
	if err != nil {
		return nil, err
	}
	return dictionaryTSV(path)
}

// UpdateBookDictionaryTSV validates and replaces a Book Dictionary from its
// hand-editable TSV representation. The replacement is atomic, so a rejected
// edit cannot leave a partially written Dictionary behind.
func (s Store) UpdateBookDictionaryTSV(seriesCode, bookCode, targetLanguage string, body []byte) error {
	path, err := s.checkedBookDictionaryPath(seriesCode, bookCode, targetLanguage)
	if err != nil {
		return err
	}
	terms, err := parseDictionary(body)
	if err != nil {
		return err
	}
	return writeDictionary(path, terms)
}

// SeriesDictionary reads the Dictionary shared by a Series for one Language Pair.
func (s Store) SeriesDictionary(seriesCode, targetLanguage string) ([]Term, error) {
	series, err := s.series(seriesCode)
	if err != nil {
		return nil, err
	}
	return readDictionary(s.seriesDictionaryPath(seriesCode, series.SourceLanguage, targetLanguage))
}

// SeriesDictionaryTSV returns the hand-editable TSV exactly as it is stored.
func (s Store) SeriesDictionaryTSV(seriesCode, targetLanguage string) ([]byte, error) {
	series, err := s.series(seriesCode)
	if err != nil {
		return nil, err
	}
	return dictionaryTSV(s.seriesDictionaryPath(seriesCode, series.SourceLanguage, targetLanguage))
}

// WriteSeriesDictionary replaces a Series Dictionary with its reviewed Terms.
func (s Store) WriteSeriesDictionary(seriesCode, targetLanguage string, terms []Term) error {
	series, err := s.series(seriesCode)
	if err != nil {
		return err
	}
	path := s.seriesDictionaryPath(seriesCode, series.SourceLanguage, targetLanguage)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", DictionariesDir, err)
	}
	return writeDictionary(path, terms)
}

// Dictionary merges the Series and Book Dictionaries, with the Book's Terms
// taking precedence when both define the same original text.
func (s Store) Dictionary(seriesCode, bookCode, targetLanguage string) ([]Term, error) {
	seriesTerms, err := s.SeriesDictionary(seriesCode, targetLanguage)
	if err != nil {
		return nil, err
	}
	bookTerms, err := s.BookDictionary(seriesCode, bookCode, targetLanguage)
	if err != nil {
		return nil, err
	}
	return mergeTerms(seriesTerms, bookTerms), nil
}

// PromoteDictionaryTerm copies one reviewed Book Term into the Series
// Dictionary, where later Books in the Series inherit it.
func (s Store) PromoteDictionaryTerm(seriesCode, bookCode, targetLanguage, original string) error {
	bookTerms, err := s.BookDictionary(seriesCode, bookCode, targetLanguage)
	if err != nil {
		return err
	}
	for _, term := range bookTerms {
		if term.Original == original {
			seriesTerms, err := s.SeriesDictionary(seriesCode, targetLanguage)
			if err != nil {
				return err
			}
			return s.WriteSeriesDictionary(seriesCode, targetLanguage, mergeTerms(seriesTerms, []Term{term}))
		}
	}
	return fmt.Errorf("Dictionary Term %q is not in Book %q", original, bookCode)
}

// UnpromoteDictionaryTerm removes a Term from the Series Dictionary without
// changing the Book Dictionary it was originally promoted from.
func (s Store) UnpromoteDictionaryTerm(seriesCode, targetLanguage, original string) error {
	seriesTerms, err := s.SeriesDictionary(seriesCode, targetLanguage)
	if err != nil {
		return err
	}
	remaining := make([]Term, 0, len(seriesTerms))
	for _, term := range seriesTerms {
		if term.Original != original {
			remaining = append(remaining, term)
		}
	}
	if len(remaining) == len(seriesTerms) {
		return fmt.Errorf("Dictionary Term %q is not in the Series Dictionary", original)
	}
	return s.WriteSeriesDictionary(seriesCode, targetLanguage, remaining)
}

func (s Store) bookDictionaryPath(seriesCode, bookCode, targetLanguage, sourceLanguage string) string {
	return filepath.Join(s.root, seriesCode, BooksDir, bookCode, TranslationsDir, languagePair(sourceLanguage, targetLanguage), DictionaryFile)
}

func (s Store) checkedBookDictionaryPath(seriesCode, bookCode, targetLanguage string) (string, error) {
	series, err := s.series(seriesCode)
	if err != nil {
		return "", err
	}
	path := s.bookDictionaryPath(seriesCode, bookCode, targetLanguage, series.SourceLanguage)
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("Translation Target %q does not exist", targetLanguage)
		}
		return "", fmt.Errorf("read Translation Target: %w", err)
	}
	return path, nil
}

func (s Store) seriesDictionaryPath(seriesCode, sourceLanguage, targetLanguage string) string {
	return filepath.Join(s.root, seriesCode, DictionariesDir, languagePair(sourceLanguage, targetLanguage)+".tsv")
}

func readDictionary(path string) ([]Term, error) {
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Dictionary: %w", err)
	}
	return parseDictionary(body)
}

func parseDictionary(body []byte) ([]Term, error) {
	var terms []Term
	for lineNumber, line := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if lineNumber == 0 && slices.Equal(fields, []string{"original", "translation", "note"}) {
			continue
		}
		if len(fields) < 2 || len(fields) > 3 {
			return nil, fmt.Errorf("read Dictionary: line %d is not TSV original, translation, note", lineNumber+1)
		}
		term := Term{Original: strings.TrimSpace(fields[0]), Translation: strings.TrimSpace(fields[1])}
		if len(fields) == 3 {
			term.Note = strings.TrimSpace(fields[2])
		}
		if term.Original == "" || term.Translation == "" {
			return nil, fmt.Errorf("read Dictionary: line %d has an empty original or translation", lineNumber+1)
		}
		terms = mergeTerms(terms, []Term{term})
	}
	return terms, nil
}

func dictionaryTSV(path string) ([]byte, error) {
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []byte("original\ttranslation\tnote\n"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Dictionary: %w", err)
	}
	return body, nil
}

func writeDictionary(path string, terms []Term) error {
	var body strings.Builder
	body.WriteString("original\ttranslation\tnote\n")
	for _, term := range terms {
		if strings.TrimSpace(term.Original) == "" || strings.TrimSpace(term.Translation) == "" {
			return errors.New("Dictionary Terms need an original and translation")
		}
		if strings.ContainsAny(term.Original+term.Translation+term.Note, "\t\n\r") {
			return errors.New("Dictionary Terms cannot contain tabs or line breaks")
		}
		fmt.Fprintf(&body, "%s\t%s\t%s\n", term.Original, term.Translation, term.Note)
	}
	return writeAtomic(path, []byte(body.String()))
}

func mergeTerms(base, overrides []Term) []Term {
	terms := append([]Term(nil), base...)
	positions := make(map[string]int, len(terms))
	for index, term := range terms {
		positions[term.Original] = index
	}
	for _, term := range overrides {
		if index, ok := positions[term.Original]; ok {
			terms[index] = term
			continue
		}
		positions[term.Original] = len(terms)
		terms = append(terms, term)
	}
	return terms
}

func writeAtomic(path string, body []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".temporary-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

// book confirms a Book exists without making callers depend on the whole
// Library's rendering-oriented traversal.
func (s Store) book(seriesCode, bookCode string) (Series, Book, bool) {
	series, err := s.series(seriesCode)
	if err != nil {
		return Series{}, Book{}, false
	}
	return (Library{Series: []Series{series}}).Book(seriesCode, bookCode)
}

func (s Store) bookFile(seriesCode, bookCode string) (string, bookFile, error) {
	if _, _, ok := s.book(seriesCode, bookCode); !ok {
		return "", bookFile{}, fmt.Errorf("Book %q is not in Series %q", bookCode, seriesCode)
	}
	path := filepath.Join(s.root, seriesCode, BooksDir, bookCode, BookFile)
	var file bookFile
	if _, err := toml.DecodeFile(path, &file); err != nil {
		return "", bookFile{}, fmt.Errorf("read %s: %w", filepath.Join(seriesCode, BooksDir, bookCode, BookFile), err)
	}
	return path, file, nil
}

func writeBookFile(path string, file bookFile) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", BookFile, err)
	}
	replacements := map[string]string{
		"title":                       tomlString(file.Title),
		"author":                      tomlString(file.Author),
		"source_file_language":        tomlString(file.SourceFileLanguage),
		"title_edited":                fmt.Sprintf("%t", file.TitleEdited),
		"author_edited":               fmt.Sprintf("%t", file.AuthorEdited),
		"source_file_language_edited": fmt.Sprintf("%t", file.SourceFileLanguageEdited),
	}
	lines := strings.Split(string(body), "\n")
	for key, value := range replacements {
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), key+" =") {
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				lines[i] = indent + key + " = " + value
				goto replaced
			}
		}
		lines = append(lines, key+" = "+value)
	replaced:
	}
	return writeAtomic(path, []byte(strings.Join(lines, "\n")))
}

// sourceFile finds the one Source File a Book may have. Both names are checked
// explicitly so an unexpected user file is never mistaken for source material.
func sourceFile(bookDir string) (string, error) {
	for _, name := range []string{SourceFilePrefix + ".txt", SourceFilePrefix + ".fb2"} {
		_, err := os.Stat(filepath.Join(bookDir, name))
		if err == nil {
			return name, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read Source File: %w", err)
		}
	}
	return "", nil
}

// DeleteTranslationTarget abandons only the requested language pair.
func (s Store) DeleteTranslationTarget(seriesCode, bookCode, targetLanguage string) error {
	series, err := s.series(seriesCode)
	if err != nil {
		return err
	}
	if _, _, ok := (Library{Series: []Series{series}}).Book(seriesCode, bookCode); !ok {
		return fmt.Errorf("Book %q is not in Series %q", bookCode, seriesCode)
	}
	pair := languagePair(series.SourceLanguage, targetLanguage)
	path := filepath.Join(s.root, seriesCode, BooksDir, bookCode, TranslationsDir, pair)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("delete Translation Target %s: %w", pair, err)
	}
	return nil
}

func (s Store) translationTargets(seriesCode, bookCode, sourceLanguage string) ([]TranslationTarget, error) {
	dir := filepath.Join(s.root, seriesCode, BooksDir, bookCode, TranslationsDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Join(seriesCode, BooksDir, bookCode, TranslationsDir), err)
	}
	var targets []TranslationTarget
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), sourceLanguage+"-to-") {
			continue
		}
		tag := strings.TrimPrefix(entry.Name(), sourceLanguage+"-to-")
		if _, ok := workspace.LanguageFor(tag); !ok {
			return nil, fmt.Errorf("read Translation Target %s: unknown target language", entry.Name())
		}
		var state struct {
			Status string `json:"status"`
		}
		if body, err := os.ReadFile(filepath.Join(dir, entry.Name(), StateFile)); err != nil {
			return nil, fmt.Errorf("read Translation Target %s: %w", entry.Name(), err)
		} else if err := json.Unmarshal(body, &state); err != nil {
			return nil, fmt.Errorf("read Translation Target %s: %w", entry.Name(), err)
		}
		status, ok := statusFor(state.Status)
		if !ok {
			return nil, fmt.Errorf("read Translation Target %s: unknown status %q", entry.Name(), state.Status)
		}
		targets = append(targets, TranslationTarget{Language: tag, Status: status})
	}
	slices.SortFunc(targets, func(a, b TranslationTarget) int { return strings.Compare(a.Language, b.Language) })
	return targets, nil
}

func languagePair(source, target string) string { return source + "-to-" + target }

func statusFor(value string) (Status, bool) {
	for _, status := range []Status{StatusNew, StatusAnalyzing, StatusDictionaryReady, StatusTranslating, StatusTranslated, StatusFailed} {
		if value == strings.ToLower(strings.ReplaceAll(string(status), " ", "_")) {
			return status, true
		}
	}
	return "", false
}

// validatedBook checks a draft against the Series it is going into, and
// returns the Book it describes.
func (s Store) validatedBook(seriesCode string, draft BookDraft) (Book, error) {
	if _, err := os.Stat(filepath.Join(s.root, seriesCode, SeriesFile)); err != nil {
		return Book{}, fmt.Errorf("%w: %q", ErrSeriesNotFound, seriesCode)
	}
	book, err := validatedDraft(draft)
	if err != nil {
		return Book{}, err
	}

	series, err := s.series(seriesCode)
	if err != nil {
		return Book{}, err
	}
	existing := series.Books
	for _, b := range existing {
		// Compared without case, because two codes differing only in case are
		// two folders on Linux and one on macOS and Windows, and a library has
		// to mean the same thing on all three.
		if strings.EqualFold(b.Code, book.Code) {
			return Book{}, fmt.Errorf("%w: %q", ErrBookCodeTaken, book.Code)
		}
	}
	return book, nil
}

// validatedDraft checks what a Book needs wherever it is going, which is its
// Book Code and nothing else.
func validatedDraft(draft BookDraft) (Book, error) {
	code := strings.TrimSpace(draft.Code)
	if err := ValidateBookCode(code); err != nil {
		return Book{}, err
	}
	return Book{
		Code:   code,
		Title:  strings.TrimSpace(draft.Title),
		Author: strings.TrimSpace(draft.Author),
	}, nil
}

// unusedSeriesCode picks a folder name for a new Series that no folder in the
// workspace has already.
//
// Every entry in the root counts as taken, not only the Series: a name that
// slugs to "workspace.toml" or to a folder of the user's own notes must step
// aside rather than write into it. What is taken is compared without case,
// because macOS and Windows would treat two such names as one folder — but the
// name asked for is returned as it was asked for, since for a Series of one it
// is the Book Code the user typed.
func (s Store) unusedSeriesCode(preferred string) (string, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return "", fmt.Errorf("read the workspace folder: %w", err)
	}
	taken := make(map[string]bool, len(entries))
	for _, entry := range entries {
		taken[strings.ToLower(entry.Name())] = true
	}
	return unusedCode(preferred, taken), nil
}

func (s Store) writeSeries(code, name, sourceLanguage string) error {
	body := fmt.Sprintf(seriesTemplate, tomlString(name), tomlString(sourceLanguage))
	return s.writeConfig(filepath.Join(code, SeriesFile), body)
}

func (s Store) writeBook(seriesCode string, book Book) error {
	body := fmt.Sprintf(bookTemplate, tomlString(book.Title), tomlString(book.Author), tomlString(book.SourceFileLanguage), book.Title != "", book.Author != "", book.SourceFileLanguage != "")
	return s.writeConfig(filepath.Join(seriesCode, BooksDir, book.Code, BookFile), body)
}

// writeConfig writes one config file and the folders leading down to it.
//
// Errors name the file the way the user sees it — relative to their workspace —
// because the message goes on screen, and an absolute path through a temporary
// directory tells them nothing about the library they are looking at.
func (s Store) writeConfig(rel, body string) error {
	path := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create the folder for %s: %w", rel, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	return nil
}

// tomlString encodes one value the way TOML wants it, so that a title holding
// a quote or a backslash cannot break the file it is written into.
func tomlString(s string) string {
	quoted := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`, "\r", `\r`).Replace(s)
	return `"` + quoted + `"`
}

// Both files are written from a template rather than encoded, for the same
// reason workspace.toml is: they are addressed to the person who will edit
// them, and comments only survive if nothing re-encodes them. Nothing here
// rewrites either file.

const seriesTemplate = `# A Series: a group of Books sharing a source language, a Dictionary and
# translation settings. Edit this file by hand whenever you like; Scriptorium
# reads it and never rewrites it, so these comments stay where you put them.

# The name shown in the library. The folder this file sits in is what the
# Series is called on disk, and renaming the folder renames the Series.
name = %s

# The language the Books in this Series are written in. Set once here rather
# than per Book, since a Series is books by one author in one language.
source_language = %s

# Everything else is inherited from workspace.toml at the root of your library:
# the Agent, the Models, and the target languages offered. Uncomment to override
# it for the Books in this Series.
#
# agent = "claude"
#
# [models]
# mechanical = "claude-haiku-4-5"
# translation = "claude-opus-5"
`

const bookTemplate = `# A Book: one work, with one Source File. The folder this file sits in is the
# Book Code — the short name you gave it, and the name to look for on disk.
# Scriptorium reads this file and never rewrites it.

title = %s
author = %s
source_file_language = %s

# Values entered on the details page are protected from later Source File
# parsing. They are machine-managed so an intentional blank is protected too.
title_edited = %t
author_edited = %t
source_file_language_edited = %t

# The Agent and Models are inherited from this Series and from workspace.toml.
# Uncomment to spend more on this Book than on the rest, or less.
#
# agent = "claude"
#
# [models]
# mechanical = "claude-haiku-4-5"
# translation = "claude-opus-5"
`
