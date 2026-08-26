// Package library holds the domain types describing a Scriptorium library.
//
// The vocabulary here is the vocabulary of CONTEXT.md: a Series groups Books,
// a Book has one Source File, and a Translation Target pairs a Book with one
// target language. Status and progress belong to the Translation Target.
package library

// Status is the stage a Translation Target has reached.
type Status string

const (
	StatusNew             Status = "New"
	StatusAnalyzing       Status = "Analyzing"
	StatusDictionaryReady Status = "Dictionary Ready"
	StatusTranslating     Status = "Translating"
	StatusTranslated      Status = "Translated"
	StatusFailed          Status = "Failed"
)

// TranslationTarget is the pairing of a Book with one target language.
type TranslationTarget struct {
	Language string
	Status   Status
}

// Book is a single work belonging to exactly one Series. Its Book Code names
// the folder it is stored under.
type Book struct {
	Code    string
	Title   string
	Author  string
	Targets []TranslationTarget
}

// Series is a named group of Books sharing a source language, a Dictionary and
// translation settings. A standalone Book is a Series containing one Book.
type Series struct {
	Code           string
	Name           string
	SourceLanguage string
	Books          []Book
}

// Standalone reports whether the Series holds exactly one Book, in which case
// the library page renders the Book without a group header.
func (s Series) Standalone() bool { return len(s.Books) == 1 }

// Library is the whole collection of Series in a workspace.
type Library struct {
	Series []Series
}

// Book finds a Book by its Series code and Book Code.
func (l Library) Book(seriesCode, bookCode string) (Series, Book, bool) {
	for _, s := range l.Series {
		if s.Code != seriesCode {
			continue
		}
		for _, b := range s.Books {
			if b.Code == bookCode {
				return s, b, true
			}
		}
	}
	return Series{}, Book{}, false
}
