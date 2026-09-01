package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/format"
	"github.com/ijackua/scriptorium/internal/format/fb2"
	"github.com/ijackua/scriptorium/internal/format/txt"
	"github.com/ijackua/scriptorium/internal/library"
)

func TestTranslatorProducesAStructurallyIdenticalFB2FromPersistedChunks(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "test_data", "The Adventures of Sherlock Holmes.fb2"))
	if err != nil {
		t.Fatalf("read FB2 fixture: %v", err)
	}
	fixture := newReadyTarget(t, "Sherlock Holmes", "en", "adventures", "uk", "adventures.fb2", source)

	document, err := fb2.Parse(source)
	if err != nil {
		t.Fatalf("parse FB2 fixture: %v", err)
	}
	chunks := ChunkNodes(document.Nodes, 5000)
	responses := make([]agent.Response, 0, len(chunks))
	for _, chunk := range chunks {
		translated := make([]format.TextNode, len(chunk.Nodes))
		for i, node := range chunk.Nodes {
			translated[i] = format.TextNode{Index: node.Index, Chapter: node.Chapter, Text: "Translated Text Node " + indexString(node.Index)}
		}
		responses = append(responses, agent.Response{Result: SerializeNodes(translated), Cost: 0.01})
	}
	fake := agent.NewFake(responses...)
	if _, err := (Translator{Root: fixture.root, ChunkWordBudget: 5000}).PrepareTextChunks(fixture.series.Code, "adventures"); err != nil {
		t.Fatalf("PrepareTextChunks: %v", err)
	}

	outputPath, err := (Translator{
		Root:             fixture.root,
		Agent:            fake,
		TranslationModel: "strong",
		ChunkWordBudget:  5000,
	}).Translate(context.Background(), fixture.series.Code, "adventures", "uk")
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	wantOutputPath := filepath.Join(fixture.root, fixture.series.Code, library.BooksDir, "adventures", library.TranslationsDir, "en-to-uk", "out", "adventures.uk.fb2")
	if outputPath != wantOutputPath {
		t.Errorf("output path = %q, want %q", outputPath, wantOutputPath)
	}

	storedSource, err := os.ReadFile(filepath.Join(fixture.root, fixture.series.Code, library.BooksDir, "adventures", "source.fb2"))
	if err != nil {
		t.Fatalf("read stored Source File: %v", err)
	}
	if !bytes.Equal(storedSource, source) {
		t.Error("translation changed the Source File")
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read translated FB2: %v", err)
	}
	if got, want := xmlStructure(t, output), xmlStructure(t, source); !reflect.DeepEqual(got, want) {
		t.Error("translated FB2 structure differs from its Source File")
	}

	bookDir := filepath.Join(fixture.root, fixture.series.Code, library.BooksDir, "adventures")
	manifestBody, err := os.ReadFile(filepath.Join(bookDir, "chunks", "manifest.json"))
	if err != nil {
		t.Fatalf("read Chunk Materialization manifest: %v", err)
	}
	var manifest struct {
		Chunks []struct {
			Index      int    `json:"index"`
			Chapter    int    `json:"chapter"`
			TextNodes  []int  `json:"text_nodes"`
			SourceHash string `json:"source_hash"`
		} `json:"chunks"`
	}
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if len(manifest.Chunks) != len(chunks) {
		t.Fatalf("manifest Chunks = %d, want %d", len(manifest.Chunks), len(chunks))
	}

	var persistedTranslations []format.TextNode
	for _, chunk := range chunks {
		originalPath := filepath.Join(bookDir, "chunks", "original", indexString(chunk.Index)+".txt")
		originalBody, err := os.ReadFile(originalPath)
		if err != nil {
			t.Fatalf("read original Chunk %d: %v", chunk.Index, err)
		}
		if string(originalBody) != SerializeNodes(chunk.Nodes) {
			t.Errorf("original Chunk %d does not preserve its numbered Text Nodes", chunk.Index)
		}

		translatedPath := filepath.Join(bookDir, library.TranslationsDir, "en-to-uk", "chunks", "translated", indexString(chunk.Index)+".txt")
		translatedBody, err := os.ReadFile(translatedPath)
		if err != nil {
			t.Fatalf("read translated Chunk %d: %v", chunk.Index, err)
		}
		translated, err := ValidateTranslation(chunk.Nodes, string(translatedBody))
		if err != nil {
			t.Fatalf("validate persisted translated Chunk %d: %v", chunk.Index, err)
		}
		persistedTranslations = append(persistedTranslations, translated...)
	}
	wantOutput, err := document.Splice(persistedTranslations)
	if err != nil {
		t.Fatalf("compose persisted Chunk Translations: %v", err)
	}
	if !bytes.Equal(output, wantOutput) {
		t.Error("translated FB2 was not composed from persisted Chunk Translations")
	}
}

func TestTranslatorSendsSerialChapterBoundedRequestsAndRecordsProgress(t *testing.T) {
	source := []byte("Alpha Beta\n\nGamma Delta\n\nChapter 2\n\nEpsilon Zeta\n\nEta Theta")
	fixture := newReadyTarget(t, "Solaris", "pl", "solaris", "uk", "solaris.txt", source)
	if err := fixture.store.WriteSeriesDictionary(fixture.series.Code, "uk", []library.Term{
		{Original: "Alpha", Translation: "series-alpha"},
		{Original: "Solaris", Translation: "Солярис"},
	}); err != nil {
		t.Fatalf("WriteSeriesDictionary: %v", err)
	}
	if err := fixture.store.WriteDictionary(fixture.series.Code, "solaris", "uk", []library.Term{
		{Original: "Alpha", Translation: "book-alpha"},
		{Original: "Gamma", Translation: "Гамма"},
	}); err != nil {
		t.Fatalf("WriteDictionary: %v", err)
	}
	document, err := txt.Parse(source)
	if err != nil {
		t.Fatalf("parse TXT fixture: %v", err)
	}
	chunks := ChunkNodes(document.Nodes, 4)
	if _, err := (Translator{Root: fixture.root, ChunkWordBudget: 4}).PrepareTextChunks(fixture.series.Code, "solaris"); err != nil {
		t.Fatalf("PrepareTextChunks: %v", err)
	}
	responses := make([]agent.Response, 0, len(chunks))
	for _, chunk := range chunks {
		translated := make([]format.TextNode, len(chunk.Nodes))
		for i, node := range chunk.Nodes {
			translated[i] = format.TextNode{
				Index: node.Index, Chapter: chunk.Chapter,
				Text: fmt.Sprintf("accepted-translation-%d-%d", chunk.Index, node.Index),
			}
		}
		responses = append(responses, agent.Response{Result: SerializeNodes(translated), Cost: float64(chunk.Index+1) / 10})
	}
	fake := agent.NewFake(responses...)
	targetDirectory := filepath.Join(fixture.root, fixture.series.Code, library.BooksDir, "solaris", library.TranslationsDir, "pl-to-uk")
	observed := &observingAgent{
		fake: fake,
		before: func(requestIndex int) {
			manifestPath := filepath.Join(fixture.root, fixture.series.Code, library.BooksDir, "solaris", "chunks", "manifest.json")
			if _, err := os.Stat(manifestPath); err != nil {
				t.Errorf("request %d began before the manifest was persisted: %v", requestIndex, err)
			}
			stateBody, err := os.ReadFile(filepath.Join(targetDirectory, library.StateFile))
			if err != nil {
				t.Errorf("read state before request %d: %v", requestIndex, err)
				return
			}
			var progress struct {
				Status string `json:"status"`
				Chunks []struct {
					Status string `json:"status"`
				} `json:"chunks"`
			}
			if err := json.Unmarshal(stateBody, &progress); err != nil {
				t.Errorf("decode state before request %d: %v", requestIndex, err)
				return
			}
			if progress.Status != "translating" {
				t.Errorf("Status before request %d = %q, want translating", requestIndex, progress.Status)
			}
			if requestIndex == 0 {
				for _, chunk := range chunks {
					path := filepath.Join(fixture.root, fixture.series.Code, library.BooksDir, "solaris", "chunks", "original", indexString(chunk.Index)+".txt")
					if _, err := os.Stat(path); err != nil {
						t.Errorf("first request began before original Chunk %d was persisted: %v", chunk.Index, err)
					}
				}
				return
			}
			translatedPath := filepath.Join(targetDirectory, "chunks", "translated", indexString(requestIndex-1)+".txt")
			if _, err := os.Stat(translatedPath); err != nil {
				t.Errorf("request %d began before translated Chunk %d was persisted: %v", requestIndex, requestIndex-1, err)
			}
			if progress.Chunks[requestIndex-1].Status != "completed" {
				t.Errorf("state before request %d = %+v, want prior Chunk completed", requestIndex, progress)
			}
		},
	}

	_, err = (Translator{
		Root: fixture.root, Agent: observed, TranslationModel: "strong", ChunkWordBudget: 4,
	}).Translate(context.Background(), fixture.series.Code, "solaris", "uk")
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if observed.overlapped.Load() {
		t.Error("Agent requests overlapped")
	}

	requests := fake.RecordedRequests()
	if len(requests) != len(chunks) {
		t.Fatalf("recorded requests = %d, want %d", len(requests), len(chunks))
	}
	for i, request := range requests {
		for _, required := range []string{
			"Polish (pl)", "Ukrainian (uk)",
			"Alpha\tbook-alpha", "Solaris\tСолярис", "Gamma\tГамма",
			"REFERENCE MATERIAL ONLY. DO NOT TRANSLATE",
			"[[NODE <index>]]", "[[/NODE]]",
		} {
			if !strings.Contains(request.Prompt, required) {
				t.Errorf("request %d lacks %q:\n%s", i, required, request.Prompt)
			}
		}
		if strings.Contains(request.Prompt, "Alpha\tseries-alpha") {
			t.Errorf("request %d contains overridden Series Dictionary rendering", i)
		}
		if request.Model != "strong" {
			t.Errorf("request %d Model = %q, want strong", i, request.Model)
		}
		gotNodes := strings.TrimPrefix(request.Prompt[strings.LastIndex(request.Prompt, "TEXT NODES TO TRANSLATE:"):], "TEXT NODES TO TRANSLATE:\n")
		if want := SerializeNodes(chunks[i].Nodes); gotNodes != want {
			t.Errorf("request %d Text Nodes = %q, want %q", i, gotNodes, want)
		}

		startsChapter := i == 0 || chunks[i-1].Chapter != chunks[i].Chapter
		if startsChapter {
			if !strings.Contains(request.Prompt, "(empty — start of Chapter)") {
				t.Errorf("request %d begins a Chapter with a non-empty Continuity Window", i)
			}
			if i > 0 && strings.Contains(request.Prompt, fmt.Sprintf("accepted-translation-%d-", i-1)) {
				t.Errorf("request %d carries continuity across a Chapter boundary", i)
			}
		} else {
			for _, node := range chunks[i-1].Nodes {
				if !strings.Contains(request.Prompt, fmt.Sprintf("accepted-translation-%d-%d", i-1, node.Index)) {
					t.Errorf("request %d lacks preceding accepted translation for Text Node %d", i, node.Index)
				}
			}
		}
	}

	statePath := filepath.Join(fixture.root, fixture.series.Code, library.BooksDir, "solaris", library.TranslationsDir, "pl-to-uk", library.StateFile)
	stateBody, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	var state struct {
		Status                string `json:"status"`
		SourceFile            string `json:"source_file"`
		SourceFingerprint     string `json:"source_fingerprint"`
		DictionaryFingerprint string `json:"dictionary_fingerprint"`
		Chunks                []struct {
			Index      int     `json:"index"`
			Status     string  `json:"status"`
			SourceHash string  `json:"source_hash"`
			Cost       float64 `json:"cost"`
			Attempts   int     `json:"attempts"`
		} `json:"chunks"`
	}
	if err := json.Unmarshal(stateBody, &state); err != nil {
		t.Fatalf("decode state.json: %v", err)
	}
	if state.Status != "translated" || state.SourceFile != "source.txt" || state.SourceFingerprint == "" || state.DictionaryFingerprint == "" {
		t.Errorf("state identity/status = %+v, want translated with active fingerprints", state)
	}
	if len(state.Chunks) != len(chunks) {
		t.Fatalf("state Chunks = %d, want %d", len(state.Chunks), len(chunks))
	}
	for i, chunk := range state.Chunks {
		if chunk.Index != i || chunk.Status != "completed" || chunk.SourceHash == "" || chunk.Attempts != 1 || chunk.Cost != float64(i+1)/10 {
			t.Errorf("state Chunk %d = %+v, want completed progress with hash, cost, and one attempt", i, chunk)
		}
	}
	for _, prose := range []string{"Alpha Beta", "accepted-translation"} {
		if bytes.Contains(stateBody, []byte(prose)) {
			t.Errorf("state.json stores prose %q", prose)
		}
	}
}

func TestTranslatorHandlesTXTWithoutDetectableChapters(t *testing.T) {
	source := []byte("First ordinary paragraph.\n\nSecond ordinary paragraph.")
	fixture := newReadyTarget(t, "Notes", "en", "notes", "uk", "notes.txt", source)
	if _, err := (Translator{Root: fixture.root, ChunkWordBudget: 1}).PrepareTextChunks(fixture.series.Code, "notes"); err != nil {
		t.Fatalf("PrepareTextChunks: %v", err)
	}
	fake := agent.NewFake(
		agent.Response{Result: "[[NODE 0]]\nПерший абзац.\n[[/NODE]]"},
		agent.Response{Result: "[[NODE 1]]\nДругий абзац.\n[[/NODE]]"},
	)

	outputPath, err := (Translator{
		Root: fixture.root, Agent: fake, TranslationModel: "strong", ChunkWordBudget: 100,
	}).Translate(context.Background(), fixture.series.Code, "notes", "uk")
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read translated TXT: %v", err)
	}
	if want := "Перший абзац.\n\nДругий абзац."; string(output) != want {
		t.Errorf("translated TXT = %q, want %q", output, want)
	}
	if got := len(fake.RecordedRequests()); got != 2 {
		t.Errorf("requests = %d, want the two persisted Chunks (without rechunking)", got)
	}
}

func TestTranslatorPersistsAcceptedChunkBeforeMarkingItCompleted(t *testing.T) {
	source := []byte("A single source Text Node.")
	fixture := newReadyTarget(t, "Interrupted", "en", "interrupted", "uk", "interrupted.txt", source)
	if _, err := (Translator{Root: fixture.root}).PrepareTextChunks(fixture.series.Code, "interrupted"); err != nil {
		t.Fatalf("PrepareTextChunks: %v", err)
	}
	targetDirectory := filepath.Join(fixture.root, fixture.series.Code, library.BooksDir, "interrupted", library.TranslationsDir, "en-to-uk")
	statePath := filepath.Join(targetDirectory, library.StateFile)
	fake := agent.NewFake(agent.Response{Result: "[[NODE 0]]\nПереклад.\n[[/NODE]]"})
	blockingAgent := &afterCallAgent{
		fake: fake,
		after: func() {
			if err := os.Remove(statePath); err != nil {
				t.Errorf("remove state before simulated interruption: %v", err)
				return
			}
			if err := os.Mkdir(statePath, 0o755); err != nil {
				t.Errorf("block state rename: %v", err)
			}
		},
	}

	_, err := (Translator{
		Root: fixture.root, Agent: blockingAgent, TranslationModel: "strong",
	}).Translate(context.Background(), fixture.series.Code, "interrupted", "uk")
	if err == nil || !strings.Contains(err.Error(), library.StateFile) {
		t.Fatalf("Translate error = %v, want interrupted state.json update", err)
	}
	translatedPath := filepath.Join(targetDirectory, "chunks", "translated", "0.txt")
	translated, err := os.ReadFile(translatedPath)
	if err != nil {
		t.Fatalf("accepted Chunk was not persisted before state update: %v", err)
	}
	if string(translated) != "[[NODE 0]]\nПереклад.\n[[/NODE]]" {
		t.Errorf("persisted accepted Chunk = %q", translated)
	}
	outputPath := filepath.Join(targetDirectory, "out", "interrupted.uk.txt")
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Errorf("Book Composition ran after interrupted state update: %v", err)
	}
}

type readyTargetFixture struct {
	root   string
	store  library.Store
	series library.Series
}

func newReadyTarget(t *testing.T, seriesName, sourceLanguage, bookCode, targetLanguage, sourceName string, source []byte) readyTargetFixture {
	t.Helper()
	root := t.TempDir()
	store := library.NewStore(root)
	series, err := store.CreateSeries(seriesName, sourceLanguage)
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if _, err := store.AddBook(series.Code, library.BookDraft{Code: bookCode}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	if err := store.UploadSourceFile(series.Code, bookCode, sourceName, bytes.NewReader(source), false); err != nil {
		t.Fatalf("UploadSourceFile: %v", err)
	}
	if _, err := store.CreateTranslationTarget(series.Code, bookCode, targetLanguage, []string{targetLanguage}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	if err := store.SetTranslationTargetStatus(series.Code, bookCode, targetLanguage, library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}
	return readyTargetFixture{root: root, store: store, series: series}
}

type observingAgent struct {
	fake       *agent.Fake
	before     func(int)
	next       atomic.Int32
	active     atomic.Int32
	overlapped atomic.Bool
}

type afterCallAgent struct {
	fake  *agent.Fake
	after func()
}

func (a *afterCallAgent) Call(ctx context.Context, request agent.Request) (agent.Response, error) {
	response, err := a.fake.Call(ctx, request)
	if err == nil && a.after != nil {
		a.after()
	}
	return response, err
}

func (a *observingAgent) Call(ctx context.Context, request agent.Request) (agent.Response, error) {
	index := int(a.next.Add(1) - 1)
	if a.active.Add(1) > 1 {
		a.overlapped.Store(true)
	}
	defer a.active.Add(-1)
	if a.before != nil {
		a.before(index)
	}
	// Keep the call in flight long enough for a concurrent implementation to
	// overlap deterministically instead of being serialized by agent.Fake.
	time.Sleep(time.Millisecond)
	return a.fake.Call(ctx, request)
}

func indexString(index int) string {
	return fmt.Sprintf("%d", index)
}

func xmlStructure(t *testing.T, body []byte) []string {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(body))
	var structure []string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return structure
		}
		if err != nil {
			t.Fatalf("decode XML structure: %v", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			structure = append(structure, "start:"+token.Name.Space+":"+token.Name.Local+":"+fmt.Sprint(token.Attr))
		case xml.EndElement:
			structure = append(structure, "end:"+token.Name.Space+":"+token.Name.Local)
		case xml.Comment:
			structure = append(structure, "comment:"+string(token))
		case xml.ProcInst:
			structure = append(structure, "proc:"+token.Target+":"+string(token.Inst))
		case xml.Directive:
			structure = append(structure, "directive:"+string(token))
		}
	}
}
