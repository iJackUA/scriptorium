package ui

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/translation"
)

// promptEditor edits one Prompt at one level: the workspace, or one Book.
//
// Until the level has its own file, the editor shows what the level would
// otherwise use — the level above's override, or the built-in template — so a
// first edit starts from the prompt actually in force. Saving is what creates
// the file.
type promptEditor struct {
	ID     string
	Action string
	Label  string
	Hint   string
	Slots  []string
	// File is this level's override, relative to the workspace root.
	File       string
	Customized bool
	// InheritedFile is the override the level falls back to; empty means the
	// built-in template.
	InheritedFile string
	Template      string
	Open          bool
	Problem       string
	Status        string
}

// promptLevel is where a set of editors saves, and what it falls back to.
type promptLevel struct {
	id     string
	action string
	// files are the override files for a Prompt, this level's own first, and
	// then the levels it inherits from, most specific first.
	files func(translation.Prompt) []string
}

func workspacePromptLevel() promptLevel {
	return promptLevel{
		id:     "workspace-prompt-",
		action: "/settings/prompts/",
		files:  func(p translation.Prompt) []string { return []string{translation.WorkspacePromptFile(p)} },
	}
}

func bookPromptLevel(series library.Series, book library.Book) promptLevel {
	return promptLevel{
		id:     "book-prompt-",
		action: "/series/" + series.Code + "/books/" + book.Code + "/prompts/",
		files:  func(p translation.Prompt) []string { return translation.PromptFiles(series.Code, book.Code, p) },
	}
}

func (l promptLevel) editors(root string) []promptEditor {
	editors := make([]promptEditor, 0, len(translation.Prompts()))
	for _, p := range translation.Prompts() {
		editors = append(editors, l.editor(root, p))
	}
	return editors
}

func (l promptLevel) editor(root string, p translation.Prompt) promptEditor {
	files := l.files(p)
	editor := promptEditor{
		ID:     l.id + string(p),
		Action: l.action + string(p),
		Label:  p.Label(),
		Hint:   p.Hint(),
		Slots:  p.Slots(),
		File:   files[0],
	}
	template, found, err := translation.FindPrompt(root, files, p)
	if err != nil {
		editor.Problem = err.Error()
		return editor
	}
	editor.Template = template
	editor.Customized = found == files[0]
	if !editor.Customized {
		editor.InheritedFile = found
	}
	return editor
}

func (s screens) saveWorkspacePrompt(w http.ResponseWriter, r *http.Request) {
	s.savePrompt(w, r, workspacePromptLevel())
}

func (s screens) revertWorkspacePrompt(w http.ResponseWriter, r *http.Request) {
	s.revertPrompt(w, r, workspacePromptLevel())
}

func (s screens) saveBookPrompt(w http.ResponseWriter, r *http.Request) {
	if level, ok := s.bookPromptLevel(w, r); ok {
		s.savePrompt(w, r, level)
	}
}

func (s screens) revertBookPrompt(w http.ResponseWriter, r *http.Request) {
	if level, ok := s.bookPromptLevel(w, r); ok {
		s.revertPrompt(w, r, level)
	}
}

func (s screens) bookPromptLevel(w http.ResponseWriter, r *http.Request) (promptLevel, bool) {
	store, ok := s.store(w, r)
	if !ok {
		return promptLevel{}, false
	}
	lib, err := store.Library()
	if err != nil {
		s.detail(w, r, err.Error())
		return promptLevel{}, false
	}
	series, book, found := lib.Book(r.PathValue("series"), r.PathValue("book"))
	if !found {
		http.NotFound(w, r)
		return promptLevel{}, false
	}
	return bookPromptLevel(series, book), true
}

func (s screens) savePrompt(w http.ResponseWriter, r *http.Request, level promptLevel) {
	ws, ok := s.session.Current()
	if !ok {
		s.reply(w, r, form{})
		return
	}
	p, ok := translation.ParsePrompt(r.PathValue("prompt"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	// A textarea submits CRLF line endings; the file is written with the
	// ones every other file in the workspace uses.
	template := strings.ReplaceAll(r.FormValue("template"), "\r\n", "\n")
	editor := level.editor(ws.Root, p)
	editor.Open = true
	if !editor.Customized && editor.Problem == "" && template == editor.Template {
		editor.Status = "Unchanged, so nothing was saved."
		fragment(w, "prompt-editor", editor)
		return
	}
	if err := translation.SavePrompt(ws.Root, editor.File, p, template); err != nil {
		editor.Template = template
		editor.Problem = err.Error()
		fragment(w, "prompt-editor", editor)
		return
	}
	editor = level.editor(ws.Root, p)
	editor.Open = true
	editor.Status = "Saved."
	fragment(w, "prompt-editor", editor)
}

func (s screens) revertPrompt(w http.ResponseWriter, r *http.Request, level promptLevel) {
	ws, ok := s.session.Current()
	if !ok {
		s.reply(w, r, form{})
		return
	}
	p, ok := translation.ParsePrompt(r.PathValue("prompt"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	file := level.files(p)[0]
	if err := translation.RemovePrompt(ws.Root, file); err != nil {
		editor := level.editor(ws.Root, p)
		editor.Open = true
		editor.Problem = err.Error()
		fragment(w, "prompt-editor", editor)
		return
	}
	// Leave no empty prompts folder behind for a level with no overrides.
	if dir := filepath.Join(ws.Root, filepath.Dir(file)); isEmptyDir(dir) {
		_ = os.Remove(dir)
	}
	editor := level.editor(ws.Root, p)
	editor.Open = true
	editor.Status = "Reverted."
	fragment(w, "prompt-editor", editor)
}

func isEmptyDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) == 0
}
