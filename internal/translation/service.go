package translation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/format"
	"github.com/ijackua/scriptorium/internal/format/fb2"
	"github.com/ijackua/scriptorium/internal/format/txt"
	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/workspace"
)

const (
	chunksDirectory           = "chunks"
	originalChunksDirectory   = "original"
	translatedChunksDirectory = "translated"
	manifestFile              = "manifest.json"
	outputDirectory           = "out"
	materializationVersion    = 1
)

// Translator runs the serial happy path from one ready Translation Target to
// a composed translated Book. Root is the workspace root; Agent is the sole
// external-work substitution seam.
type Translator struct {
	Root             string
	Agent            agent.Agent
	TranslationModel agent.Model
	ChunkWordBudget  int
}

type materializationManifest struct {
	Version           int               `json:"version"`
	SourceFile        string            `json:"source_file"`
	SourceFingerprint string            `json:"source_fingerprint"`
	ParserVersion     int               `json:"parser_version"`
	ChunkerVersion    int               `json:"chunker_version"`
	Chapters          []manifestChapter `json:"chapters"`
	Chunks            []manifestChunk   `json:"chunks"`
}

type manifestChapter struct {
	Index     int   `json:"index"`
	Parent    int   `json:"parent"`
	TextNodes []int `json:"text_nodes"`
}

type manifestChunk struct {
	Index      int    `json:"index"`
	Chapter    int    `json:"chapter"`
	TextNodes  []int  `json:"text_nodes"`
	SourceHash string `json:"source_hash"`
}

type translationState struct {
	Status                targetStateStatus `json:"status"`
	SourceFile            string            `json:"source_file"`
	SourceFingerprint     string            `json:"source_fingerprint"`
	DictionaryFingerprint string            `json:"dictionary_fingerprint"`
	Chunks                []chunkState      `json:"chunks"`
}

type chunkState struct {
	Index      int         `json:"index"`
	Status     chunkStatus `json:"status"`
	SourceHash string      `json:"source_hash"`
	Cost       float64     `json:"cost"`
	Attempts   int         `json:"attempts"`
}

type targetStateStatus string

const (
	targetStateTranslating targetStateStatus = "translating"
	targetStateTranslated  targetStateStatus = "translated"
)

type chunkStatus string

const (
	chunkPending   chunkStatus = "pending"
	chunkCompleted chunkStatus = "completed"
)

// Translate materializes, translates, persists, and composes one Translation
// Target. It returns the final output path.
func (t Translator) Translate(ctx context.Context, seriesCode, bookCode, targetLanguage string) (string, error) {
	if t.Agent == nil {
		return "", errors.New("translation needs an Agent")
	}
	if strings.TrimSpace(t.TranslationModel) == "" {
		return "", errors.New("translation needs a translation Model")
	}
	store := library.NewStore(t.Root)
	series, _, target, err := readyTarget(store, seriesCode, bookCode, targetLanguage)
	if err != nil {
		return "", err
	}
	source, sourceName, err := store.SourceFile(seriesCode, bookCode)
	if err != nil {
		return "", err
	}
	document, err := parseSource(sourceName, source)
	if err != nil {
		return "", err
	}
	chunks := ChunkNodes(document.TextNodes(), t.ChunkWordBudget)
	dictionary, err := store.Dictionary(seriesCode, bookCode, targetLanguage)
	if err != nil {
		return "", err
	}

	bookDirectory := filepath.Join(t.Root, seriesCode, library.BooksDir, bookCode)
	targetDirectory := filepath.Join(bookDirectory, library.TranslationsDir, languagePair(series.SourceLanguage, target.Language))
	manifest, err := materializeChunks(bookDirectory, sourceName, source, document.ChaptersList(), chunks)
	if err != nil {
		return "", err
	}
	state, err := newTranslationState(sourceName, source, dictionary, manifest.Chunks)
	if err != nil {
		return "", err
	}
	state.Status = targetStateTranslating
	statePath := filepath.Join(targetDirectory, library.StateFile)
	if err := writeJSONAtomic(statePath, state); err != nil {
		return "", err
	}

	sourceLanguage, _ := workspace.LanguageFor(series.SourceLanguage)
	targetLanguageInfo, _ := workspace.LanguageFor(targetLanguage)
	var continuity continuityWindow
	for index, chunk := range chunks {
		if index == 0 || chunks[index-1].Chapter != chunk.Chapter {
			continuity = continuityWindow{}
		}
		request := agent.Request{
			Model: t.TranslationModel,
			Prompt: translationPrompt(
				languageLabel(sourceLanguage),
				languageLabel(targetLanguageInfo),
				dictionary,
				continuity,
				chunk.Nodes,
			),
		}
		response, err := t.Agent.Call(ctx, request)
		if err != nil {
			return "", fmt.Errorf("translate Chunk %d: %w", chunk.Index, err)
		}
		translated, err := ValidateTranslation(chunk.Nodes, response.Result)
		if err != nil {
			return "", fmt.Errorf("validate Chunk %d: %w", chunk.Index, err)
		}
		translatedPath := filepath.Join(targetDirectory, chunksDirectory, translatedChunksDirectory, chunkFileName(chunk.Index))
		if err := writeAtomic(translatedPath, []byte(SerializeNodes(translated))); err != nil {
			return "", err
		}
		state.Chunks[index].Status = chunkCompleted
		state.Chunks[index].Cost = response.Cost
		state.Chunks[index].Attempts = 1
		if err := writeJSONAtomic(statePath, state); err != nil {
			return "", err
		}
		continuity = continuityWindow{
			source:       append([]format.TextNode(nil), chunk.Nodes...),
			translations: append([]format.TextNode(nil), translated...),
		}
	}

	output, err := composeFromPersisted(document, targetDirectory, chunks)
	if err != nil {
		return "", err
	}
	extension := filepath.Ext(sourceName)
	outputPath := filepath.Join(targetDirectory, outputDirectory, bookCode+"."+targetLanguage+extension)
	if err := writeAtomic(outputPath, output); err != nil {
		return "", err
	}
	state.Status = targetStateTranslated
	if err := writeJSONAtomic(statePath, state); err != nil {
		return "", err
	}
	return outputPath, nil
}

func readyTarget(store library.Store, seriesCode, bookCode, targetLanguage string) (library.Series, library.Book, library.TranslationTarget, error) {
	lib, err := store.Library()
	if err != nil {
		return library.Series{}, library.Book{}, library.TranslationTarget{}, err
	}
	series, book, ok := lib.Book(seriesCode, bookCode)
	if !ok {
		return library.Series{}, library.Book{}, library.TranslationTarget{}, fmt.Errorf("Book %q is not in Series %q", bookCode, seriesCode)
	}
	for _, target := range book.Targets {
		if target.Language != targetLanguage {
			continue
		}
		if target.Status != library.StatusDictionaryReady {
			return library.Series{}, library.Book{}, library.TranslationTarget{}, fmt.Errorf("Translation Target %q has Status %q, want %q", targetLanguage, target.Status, library.StatusDictionaryReady)
		}
		return series, book, target, nil
	}
	return library.Series{}, library.Book{}, library.TranslationTarget{}, fmt.Errorf("Translation Target %q does not exist", targetLanguage)
}

func parseSource(name string, source []byte) (format.Document, error) {
	var handler format.Handler
	switch strings.ToLower(filepath.Ext(name)) {
	case ".fb2":
		handler = fb2.Handler{}
	case ".txt":
		handler = txt.Handler{}
	default:
		return nil, fmt.Errorf("translate Source File %q: unsupported format", name)
	}
	document, err := handler.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("parse Source File %q: %w", name, err)
	}
	return document, nil
}

func materializeChunks(bookDirectory, sourceName string, source []byte, chapters []format.Chapter, chunks []Chunk) (materializationManifest, error) {
	originalDirectory := filepath.Join(bookDirectory, chunksDirectory, originalChunksDirectory)
	if err := os.MkdirAll(originalDirectory, 0o755); err != nil {
		return materializationManifest{}, fmt.Errorf("create original Chunks: %w", err)
	}
	manifest := materializationManifest{
		Version: materializationVersion, SourceFile: sourceName, SourceFingerprint: fingerprint(source),
		ParserVersion: 1, ChunkerVersion: 1,
	}
	for _, chapter := range chapters {
		manifest.Chapters = append(manifest.Chapters, manifestChapter{Index: chapter.Index, Parent: chapter.Parent, TextNodes: append([]int(nil), chapter.Nodes...)})
	}
	for _, chunk := range chunks {
		body := []byte(SerializeNodes(chunk.Nodes))
		if err := writeAtomic(filepath.Join(originalDirectory, chunkFileName(chunk.Index)), body); err != nil {
			return materializationManifest{}, err
		}
		entry := manifestChunk{Index: chunk.Index, Chapter: chunk.Chapter, SourceHash: fingerprint(body)}
		for _, node := range chunk.Nodes {
			entry.TextNodes = append(entry.TextNodes, node.Index)
		}
		manifest.Chunks = append(manifest.Chunks, entry)
	}
	if err := writeJSONAtomic(filepath.Join(bookDirectory, chunksDirectory, manifestFile), manifest); err != nil {
		return materializationManifest{}, err
	}
	return manifest, nil
}

func newTranslationState(sourceName string, source []byte, dictionary []library.Term, chunks []manifestChunk) (translationState, error) {
	dictionaryBody, err := json.Marshal(dictionary)
	if err != nil {
		return translationState{}, fmt.Errorf("fingerprint Dictionary: %w", err)
	}
	state := translationState{
		SourceFile: sourceName, SourceFingerprint: fingerprint(source), DictionaryFingerprint: fingerprint(dictionaryBody),
	}
	for _, chunk := range chunks {
		state.Chunks = append(state.Chunks, chunkState{Index: chunk.Index, Status: chunkPending, SourceHash: chunk.SourceHash})
	}
	return state, nil
}

func composeFromPersisted(document format.Document, targetDirectory string, chunks []Chunk) ([]byte, error) {
	translations := make([]format.TextNode, 0, len(document.TextNodes()))
	for _, chunk := range chunks {
		path := filepath.Join(targetDirectory, chunksDirectory, translatedChunksDirectory, chunkFileName(chunk.Index))
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read translated Chunk %d: %w", chunk.Index, err)
		}
		translated, err := ValidateTranslation(chunk.Nodes, string(body))
		if err != nil {
			return nil, fmt.Errorf("validate persisted translated Chunk %d: %w", chunk.Index, err)
		}
		translations = append(translations, translated...)
	}
	output, err := document.Splice(translations)
	if err != nil {
		return nil, fmt.Errorf("compose translated Book: %w", err)
	}
	return output, nil
}

func writeJSONAtomic(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	return writeAtomic(path, body)
}

func writeAtomic(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create folder for %s: %w", path, err)
	}
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

func fingerprint(body []byte) string {
	digest := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func chunkFileName(index int) string { return fmt.Sprintf("%d.txt", index) }

func languagePair(source, target string) string { return source + "-to-" + target }

func languageLabel(language workspace.Language) string {
	return language.Name + " (" + language.Tag + ")"
}
