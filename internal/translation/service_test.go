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
	"testing"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/format"
	"github.com/ijackua/scriptorium/internal/format/fb2"
	"github.com/ijackua/scriptorium/internal/format/txt"
	"github.com/ijackua/scriptorium/internal/library"
)

func TestTranslatorProducesAStructurallyIdenticalFB2FromPersistedChunks(t *testing.T) {
	root := t.TempDir()
	store := library.NewStore(root)
	series, err := store.CreateSeries("Sherlock Holmes", "en")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if _, err := store.AddBook(series.Code, library.BookDraft{Code: "adventures"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "test_data", "The Adventures of Sherlock Holmes.fb2"))
	if err != nil {
		t.Fatalf("read FB2 fixture: %v", err)
	}
	if err := store.UploadSourceFile(series.Code, "adventures", "adventures.fb2", bytes.NewReader(source), false); err != nil {
		t.Fatalf("UploadSourceFile: %v", err)
	}
	if _, err := store.CreateTranslationTarget(series.Code, "adventures", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	if err := store.SetTranslationTargetStatus(series.Code, "adventures", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}

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

	outputPath, err := (Translator{
		Root:             root,
		Agent:            fake,
		TranslationModel: "strong",
		ChunkWordBudget:  5000,
	}).Translate(context.Background(), series.Code, "adventures", "uk")
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	wantOutputPath := filepath.Join(root, series.Code, library.BooksDir, "adventures", library.TranslationsDir, "en-to-uk", "out", "adventures.uk.fb2")
	if outputPath != wantOutputPath {
		t.Errorf("output path = %q, want %q", outputPath, wantOutputPath)
	}

	storedSource, err := os.ReadFile(filepath.Join(root, series.Code, library.BooksDir, "adventures", "source.fb2"))
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

	bookDir := filepath.Join(root, series.Code, library.BooksDir, "adventures")
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
	root := t.TempDir()
	store := library.NewStore(root)
	series, err := store.CreateSeries("Solaris", "pl")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if _, err := store.AddBook(series.Code, library.BookDraft{Code: "solaris"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	source := []byte("Alpha Beta\n\nGamma Delta\n\nChapter 2\n\nEpsilon Zeta\n\nEta Theta")
	if err := store.UploadSourceFile(series.Code, "solaris", "solaris.txt", bytes.NewReader(source), false); err != nil {
		t.Fatalf("UploadSourceFile: %v", err)
	}
	if _, err := store.CreateTranslationTarget(series.Code, "solaris", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	if err := store.WriteSeriesDictionary(series.Code, "uk", []library.Term{
		{Original: "Alpha", Translation: "series-alpha"},
		{Original: "Solaris", Translation: "Солярис"},
	}); err != nil {
		t.Fatalf("WriteSeriesDictionary: %v", err)
	}
	if err := store.WriteDictionary(series.Code, "solaris", "uk", []library.Term{
		{Original: "Alpha", Translation: "book-alpha"},
		{Original: "Gamma", Translation: "Гамма"},
	}); err != nil {
		t.Fatalf("WriteDictionary: %v", err)
	}
	if err := store.SetTranslationTargetStatus(series.Code, "solaris", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}

	document, err := txt.Parse(source)
	if err != nil {
		t.Fatalf("parse TXT fixture: %v", err)
	}
	chunks := ChunkNodes(document.Nodes, 2)
	responses := make([]agent.Response, 0, len(chunks))
	for _, chunk := range chunks {
		translated := []format.TextNode{{
			Index: chunk.Nodes[0].Index, Chapter: chunk.Chapter,
			Text: fmt.Sprintf("accepted-translation-%d", chunk.Index),
		}}
		responses = append(responses, agent.Response{Result: SerializeNodes(translated), Cost: float64(chunk.Index+1) / 10})
	}
	fake := agent.NewFake(responses...)

	_, err = (Translator{
		Root: root, Agent: fake, TranslationModel: "strong", ChunkWordBudget: 2,
	}).Translate(context.Background(), series.Code, "solaris", "uk")
	if err != nil {
		t.Fatalf("Translate: %v", err)
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
			if i > 0 && strings.Contains(request.Prompt, fmt.Sprintf("accepted-translation-%d", i-1)) {
				t.Errorf("request %d carries continuity across a Chapter boundary", i)
			}
		} else if !strings.Contains(request.Prompt, fmt.Sprintf("accepted-translation-%d", i-1)) {
			t.Errorf("request %d lacks the preceding accepted translation", i)
		}
	}

	statePath := filepath.Join(root, series.Code, library.BooksDir, "solaris", library.TranslationsDir, "pl-to-uk", library.StateFile)
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
	root := t.TempDir()
	store := library.NewStore(root)
	series, err := store.CreateSeries("Notes", "en")
	if err != nil {
		t.Fatalf("CreateSeries: %v", err)
	}
	if _, err := store.AddBook(series.Code, library.BookDraft{Code: "notes"}); err != nil {
		t.Fatalf("AddBook: %v", err)
	}
	source := []byte("First ordinary paragraph.\n\nSecond ordinary paragraph.")
	if err := store.UploadSourceFile(series.Code, "notes", "notes.txt", bytes.NewReader(source), false); err != nil {
		t.Fatalf("UploadSourceFile: %v", err)
	}
	if _, err := store.CreateTranslationTarget(series.Code, "notes", "uk", []string{"uk"}); err != nil {
		t.Fatalf("CreateTranslationTarget: %v", err)
	}
	if err := store.SetTranslationTargetStatus(series.Code, "notes", "uk", library.StatusDictionaryReady); err != nil {
		t.Fatalf("SetTranslationTargetStatus: %v", err)
	}
	fake := agent.NewFake(agent.Response{Result: "[[NODE 0]]\nПерший абзац.\n[[/NODE]]\n[[NODE 1]]\nДругий абзац.\n[[/NODE]]"})

	outputPath, err := (Translator{
		Root: root, Agent: fake, TranslationModel: "strong", ChunkWordBudget: 100,
	}).Translate(context.Background(), series.Code, "notes", "uk")
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
	if got := len(fake.RecordedRequests()); got != 1 {
		t.Errorf("requests = %d, want one fallback-Chapter Chunk", got)
	}
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
