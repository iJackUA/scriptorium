package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
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
)

var (
	// ErrBookCodeTaken reports a Book Code already used in that Series.
	ErrBookCodeTaken = errors.New("that Book Code is already used in this Series")
	// ErrSeriesNotFound reports a Series that is not in the workspace.
	ErrSeriesNotFound = errors.New("that Series is not in this workspace")
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
	Title  string `toml:"title"`
	Author string `toml:"author"`
}

// series reads one Series and the Books under it.
func (s Store) series(code string) (Series, error) {
	path := filepath.Join(s.root, code, SeriesFile)
	var file seriesFile
	if _, err := toml.DecodeFile(path, &file); err != nil {
		return Series{}, fmt.Errorf("read %s: %w", filepath.Join(code, SeriesFile), err)
	}

	books, err := s.books(code)
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
func (s Store) books(seriesCode string) ([]Book, error) {
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
		books = append(books, Book{Code: entry.Name(), Title: file.Title, Author: file.Author})
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

	existing, err := s.books(seriesCode)
	if err != nil {
		return Book{}, err
	}
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
	body := fmt.Sprintf(bookTemplate, tomlString(book.Title), tomlString(book.Author))
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

# The Agent and Models are inherited from this Series and from workspace.toml.
# Uncomment to spend more on this Book than on the rest, or less.
#
# agent = "claude"
#
# [models]
# mechanical = "claude-haiku-4-5"
# translation = "claude-opus-5"
`
