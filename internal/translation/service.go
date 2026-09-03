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
	rejectedChunksDirectory   = "rejected"
	manifestFile              = "manifest.json"
	outputDirectory           = "out"
	materializationVersion    = 1
)

// Translator runs serial translation from one ready Translation Target to a
// composed translated Book. Root is the workspace root; Agent is the sole
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

type preparedMaterialization struct {
	manifest materializationManifest
	chunks   []Chunk
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
	FailedChunks          int               `json:"failed_chunks"`
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
	targetStateFailed      targetStateStatus = "failed"
)

type chunkStatus string

const (
	chunkPending   chunkStatus = "pending"
	chunkCompleted chunkStatus = "completed"
	chunkFailed    chunkStatus = "failed"
)

const strictRetryInstruction = `

STRICT RETRY INSTRUCTION:
Your previous response was rejected by the validator. Return exactly one complete
numbered block for every supplied Text Node, with every global index exactly once.
Return only the requested blocks; do not add a conversational prefix, heading,
Markdown, explanation, or any text outside the blocks.`

// PrepareTextChunks parses and chunks a Book's Source File, then persists the
// numbered original Chunk artifacts and manifest. It never invokes an Agent.
func (t Translator) PrepareTextChunks(seriesCode, bookCode string) (int, error) {
	store := library.NewStore(t.Root)
	source, sourceName, err := store.SourceFile(seriesCode, bookCode)
	if err != nil {
		return 0, err
	}
	document, err := parseSource(sourceName, source)
	if err != nil {
		return 0, err
	}
	chunks := ChunkNodes(document.TextNodes(), t.ChunkWordBudget)
	bookDirectory := filepath.Join(t.Root, seriesCode, library.BooksDir, bookCode)
	if _, err := materializeChunks(bookDirectory, sourceName, source, document.ChaptersList(), chunks); err != nil {
		return 0, err
	}
	return len(chunks), nil
}

// PreparedTextChunksPresent performs the cheap preflight used by the UI. It
// checks only for a manifest and at least one original Chunk file; it does not
// parse or rechunk the Source File.
func (t Translator) PreparedTextChunksPresent(seriesCode, bookCode string) (bool, error) {
	bookDirectory := filepath.Join(t.Root, seriesCode, library.BooksDir, bookCode)
	if _, err := os.Stat(filepath.Join(bookDirectory, chunksDirectory, manifestFile)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("check Chunk Materialization: %w", err)
	}
	entries, err := os.ReadDir(filepath.Join(bookDirectory, chunksDirectory, originalChunksDirectory))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check original Chunks: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
			return true, nil
		}
	}
	return false, nil
}

// Translate loads a prepared materialization, translates, persists, and
// composes one Translation Target. It returns the final output path.
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
	dictionary, err := store.Dictionary(seriesCode, bookCode, targetLanguage)
	if err != nil {
		return "", err
	}

	bookDirectory := filepath.Join(t.Root, seriesCode, library.BooksDir, bookCode)
	targetDirectory := filepath.Join(bookDirectory, library.TranslationsDir, languagePair(series.SourceLanguage, target.Language))
	manifest, err := loadPreparedMaterialization(bookDirectory, sourceName, source, document)
	if err != nil {
		return "", err
	}
	chunks := manifest.chunks
	state, err := newTranslationState(sourceName, source, dictionary, manifest.manifest.Chunks)
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
		translated, complete, attempts, cost, err := t.translateChunk(ctx, targetDirectory, sourceLanguage, targetLanguageInfo, dictionary, continuity, chunk)
		state.Chunks[index].Attempts = attempts
		state.Chunks[index].Cost = cost
		if err != nil {
			state.FailedChunks = failedChunkCount(state.Chunks)
			_ = writeJSONAtomic(statePath, state)
			return "", fmt.Errorf("translate Chunk %d: %w", chunk.Index, err)
		}
		if !complete {
			state.Chunks[index].Status = chunkFailed
			state.FailedChunks = failedChunkCount(state.Chunks)
			continuity = continuityWindow{}
			if err := writeJSONAtomic(statePath, state); err != nil {
				return "", err
			}
			continue
		}
		translatedPath := filepath.Join(targetDirectory, chunksDirectory, translatedChunksDirectory, chunkFileName(chunk.Index))
		if err := writeAtomic(translatedPath, []byte(SerializeNodes(translated))); err != nil {
			return "", err
		}
		state.Chunks[index].Status = chunkCompleted
		state.FailedChunks = failedChunkCount(state.Chunks)
		if err := writeJSONAtomic(statePath, state); err != nil {
			return "", err
		}
		continuity = continuityWindow{
			source:       append([]format.TextNode(nil), chunk.Nodes...),
			translations: append([]format.TextNode(nil), translated...),
		}
	}
	if state.FailedChunks > 0 {
		state.Status = targetStateFailed
		if err := writeJSONAtomic(statePath, state); err != nil {
			return "", err
		}
		return "", fmt.Errorf("translation has %d failed Chunks; automatic Book Composition is blocked", state.FailedChunks)
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

func (t Translator) translateChunk(ctx context.Context, targetDirectory string, sourceLanguage, targetLanguage workspace.Language, dictionary []library.Term, continuity continuityWindow, chunk Chunk) ([]format.TextNode, bool, int, float64, error) {
	basePrompt := translationPrompt(languageLabel(sourceLanguage), languageLabel(targetLanguage), dictionary, continuity, chunk.Nodes)
	attempts := 0
	var cost float64

	call := func(prompt string, nodes []format.TextNode) ([]format.TextNode, bool, error) {
		response, err := t.Agent.Call(ctx, agent.Request{Model: t.TranslationModel, Prompt: prompt})
		attempts++
		cost += response.Cost
		if err != nil {
			return nil, false, err
		}
		translated, err := ValidateTranslation(nodes, response.Result)
		if err == nil {
			return translated, true, nil
		}
		if persistErr := writeAtomic(filepath.Join(targetDirectory, chunksDirectory, rejectedChunksDirectory, chunkFileName(chunk.Index)), []byte(response.Result)); persistErr != nil {
			return nil, false, persistErr
		}
		return nil, false, nil
	}

	translated, valid, err := call(basePrompt, chunk.Nodes)
	if err != nil {
		return nil, false, attempts, cost, err
	}
	if valid {
		return translated, true, attempts, cost, nil
	}

	translated, valid, err = call(basePrompt+strictRetryInstruction, chunk.Nodes)
	if err != nil {
		return nil, false, attempts, cost, err
	}
	if valid {
		return translated, true, attempts, cost, nil
	}

	var perNode []format.TextNode
	allNodesValid := true
	for _, node := range chunk.Nodes {
		translated, valid, err = call(translationPrompt(languageLabel(sourceLanguage), languageLabel(targetLanguage), dictionary, continuity, []format.TextNode{node}), []format.TextNode{node})
		if err != nil {
			return nil, false, attempts, cost, err
		}
		if valid {
			perNode = append(perNode, translated...)
			continue
		}
		allNodesValid = false
	}
	if !allNodesValid {
		// A Chunk cannot be accepted with a hole. Successful individual nodes
		// are deliberately kept only in memory; the whole Chunk remains
		// ineligible for automatic composition.
		return nil, false, attempts, cost, nil
	}
	return perNode, true, attempts, cost, nil
}

// FailedChunkCount reads the durable failure count without contacting the
// Agent. A missing translation state has no failed Chunks yet.
func (t Translator) FailedChunkCount(seriesCode, bookCode, targetLanguage string) (int, error) {
	targetDirectory, err := t.translationTargetDirectory(seriesCode, bookCode, targetLanguage)
	if err != nil {
		return 0, err
	}
	state, err := loadTranslationState(filepath.Join(targetDirectory, library.StateFile))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return failedChunkCount(state.Chunks), nil
}

// ComposeTranslatedBook writes a diagnostic Book from valid persisted Chunk
// Translations and source Text Node fallbacks for missing or rejected Chunks.
// It never invokes the Agent or changes state.json. A partial output is named
// explicitly so it cannot be mistaken for a complete translation.
func (t Translator) ComposeTranslatedBook(seriesCode, bookCode, targetLanguage string) (string, error) {
	store := library.NewStore(t.Root)
	source, sourceName, err := store.SourceFile(seriesCode, bookCode)
	if err != nil {
		return "", err
	}
	document, err := parseSource(sourceName, source)
	if err != nil {
		return "", err
	}
	lib, err := store.Library()
	if err != nil {
		return "", err
	}
	series, _, ok := lib.Book(seriesCode, bookCode)
	if !ok {
		return "", fmt.Errorf("Book %q is not in Series %q", bookCode, seriesCode)
	}
	bookDirectory := filepath.Join(t.Root, seriesCode, library.BooksDir, bookCode)
	manifest, err := loadPreparedMaterialization(bookDirectory, sourceName, source, document)
	if err != nil {
		return "", err
	}
	targetDirectory := filepath.Join(bookDirectory, library.TranslationsDir, languagePair(series.SourceLanguage, targetLanguage))
	if _, err := loadTranslationState(filepath.Join(targetDirectory, library.StateFile)); err != nil {
		return "", fmt.Errorf("check Translation Target state: %w", err)
	}
	output, partial, err := composeWithSourceFallback(document, targetDirectory, manifest.chunks)
	if err != nil {
		return "", err
	}
	extension := filepath.Ext(sourceName)
	name := bookCode + "." + targetLanguage
	if partial {
		name += ".partial"
	}
	outputPath := filepath.Join(targetDirectory, outputDirectory, name+extension)
	if err := writeAtomic(outputPath, output); err != nil {
		return "", err
	}
	return outputPath, nil
}

func (t Translator) translationTargetDirectory(seriesCode, bookCode, targetLanguage string) (string, error) {
	lib, err := library.NewStore(t.Root).Library()
	if err != nil {
		return "", err
	}
	series, book, ok := lib.Book(seriesCode, bookCode)
	if !ok {
		return "", fmt.Errorf("Book %q is not in Series %q", bookCode, seriesCode)
	}
	for _, target := range book.Targets {
		if target.Language == targetLanguage {
			return filepath.Join(t.Root, seriesCode, library.BooksDir, bookCode, library.TranslationsDir, languagePair(series.SourceLanguage, targetLanguage)), nil
		}
	}
	return "", fmt.Errorf("Translation Target %q does not exist", targetLanguage)
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

func loadPreparedMaterialization(bookDirectory, sourceName string, source []byte, document format.Document) (preparedMaterialization, error) {
	manifestPath := filepath.Join(bookDirectory, chunksDirectory, manifestFile)
	body, err := os.ReadFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return preparedMaterialization{}, errors.New("prepare Text Chunks before starting translation")
	}
	if err != nil {
		return preparedMaterialization{}, fmt.Errorf("read Chunk Materialization manifest: %w", err)
	}
	var manifest materializationManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return preparedMaterialization{}, fmt.Errorf("decode Chunk Materialization manifest: %w", err)
	}
	if manifest.SourceFile != sourceName || manifest.SourceFingerprint != fingerprint(source) {
		return preparedMaterialization{}, errors.New("prepared Text Chunks belong to a different Source File; prepare them again")
	}
	sourceNodes := make(map[int]format.TextNode, len(document.TextNodes()))
	for _, node := range document.TextNodes() {
		sourceNodes[node.Index] = node
	}
	chunks := make([]Chunk, 0, len(manifest.Chunks))
	seenNodes := make(map[int]bool, len(sourceNodes))
	for position, entry := range manifest.Chunks {
		if entry.Index != position || len(entry.TextNodes) == 0 {
			return preparedMaterialization{}, fmt.Errorf("Chunk Materialization manifest has invalid Chunk %d", position)
		}
		path := filepath.Join(bookDirectory, chunksDirectory, originalChunksDirectory, chunkFileName(entry.Index))
		chunkBody, err := os.ReadFile(path)
		if err != nil {
			return preparedMaterialization{}, fmt.Errorf("read original Chunk %d: %w", entry.Index, err)
		}
		if fingerprint(chunkBody) != entry.SourceHash {
			return preparedMaterialization{}, fmt.Errorf("original Chunk %d has changed; prepare Text Chunks again", entry.Index)
		}
		parsed, err := parseNumberedNodes(string(chunkBody))
		if err != nil || len(parsed) != len(entry.TextNodes) {
			return preparedMaterialization{}, fmt.Errorf("validate original Chunk %d: invalid numbered Text Nodes", entry.Index)
		}
		chunkNodes := make([]format.TextNode, len(parsed))
		for nodePosition, parsedNode := range parsed {
			if parsedNode.Index != entry.TextNodes[nodePosition] || seenNodes[parsedNode.Index] {
				return preparedMaterialization{}, fmt.Errorf("validate original Chunk %d: invalid Text Node index %d", entry.Index, parsedNode.Index)
			}
			original, ok := sourceNodes[parsedNode.Index]
			if !ok || parsedNode.Text != original.Text {
				return preparedMaterialization{}, fmt.Errorf("validate original Chunk %d: Text Node %d does not match the Source File", entry.Index, parsedNode.Index)
			}
			if entry.Chapter != original.Chapter {
				return preparedMaterialization{}, fmt.Errorf("validate original Chunk %d: Chapter boundary changed", entry.Index)
			}
			seenNodes[parsedNode.Index] = true
			chunkNodes[nodePosition] = original
		}
		chunks = append(chunks, Chunk{Index: entry.Index, Chapter: entry.Chapter, Nodes: chunkNodes})
	}
	if len(seenNodes) != len(sourceNodes) {
		return preparedMaterialization{}, errors.New("prepared Text Chunks do not cover the Source File; prepare them again")
	}
	return preparedMaterialization{manifest: manifest, chunks: chunks}, nil
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
	output, partial, err := composeWithSourceFallback(document, targetDirectory, chunks)
	if err != nil {
		return nil, err
	}
	if partial {
		return nil, errors.New("automatic Book Composition is blocked by an incomplete Chunk Translation")
	}
	return output, nil
}

func composeWithSourceFallback(document format.Document, targetDirectory string, chunks []Chunk) ([]byte, bool, error) {
	translations := make([]format.TextNode, 0, len(document.TextNodes()))
	partial := false
	for _, chunk := range chunks {
		path := filepath.Join(targetDirectory, chunksDirectory, translatedChunksDirectory, chunkFileName(chunk.Index))
		body, err := os.ReadFile(path)
		if err == nil {
			translated, validationErr := ValidateTranslation(chunk.Nodes, string(body))
			if validationErr == nil {
				translations = append(translations, translated...)
				continue
			}
		}
		partial = true
		translations = append(translations, chunk.Nodes...)
	}
	output, err := document.Splice(translations)
	if err != nil {
		return nil, partial, fmt.Errorf("compose translated Book: %w", err)
	}
	return output, partial, nil
}

func loadTranslationState(path string) (translationState, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return translationState{}, err
	}
	var state translationState
	if err := json.Unmarshal(body, &state); err != nil {
		return translationState{}, fmt.Errorf("decode Translation Target state: %w", err)
	}
	return state, nil
}

func failedChunkCount(chunks []chunkState) int {
	count := 0
	for _, chunk := range chunks {
		if chunk.Status == chunkFailed {
			count++
		}
	}
	return count
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
