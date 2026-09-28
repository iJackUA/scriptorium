package ui

import (
	"net/http"
	"slices"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/workspace"
)

// choicesPanel is the Agent, Model and Effort pickers for every Task: either
// the workspace defaults, or one Book's overrides of them. Each change posts
// the whole panel and is saved at once, so there is no Save button to forget.
type choicesPanel struct {
	ID      string
	Action  string
	Book    bool
	Tasks   []taskPicker
	Problem string
	Saved   bool
}

// modelOption is one entry of a Model picker.
type modelOption struct {
	Name  string
	Label string
}

// taskPicker is the three pickers for one Task. On a Book an empty selection
// means inheriting, and Inherited says what that currently resolves to.
type taskPicker struct {
	Task      workspace.Task
	Label     string
	Hint      string
	Selected  workspace.Choice
	Inherited workspace.Choice
	Agents    []string
	Models    []modelOption
	// Unlisted says why the Model picker offers only what is configured: the
	// Agent could not be asked for its Models.
	Unlisted string
	Efforts  []agent.Effort
}

var taskLabels = map[workspace.Task][2]string{
	workspace.TaskMechanical:  {"Mechanical work", "Metadata inference and Dictionary Building: high volume, low stakes."},
	workspace.TaskTranslation: {"Translation", "Translating Chunks, where quality is the whole point."},
}

// catalog answers which Models an Agent accepts.
type catalog func(agentName string) ([]agent.ModelInfo, error)

func newTaskPicker(models catalog, task workspace.Task, selected, inherited workspace.Choice) taskPicker {
	effectiveAgent := selected.Agent
	if effectiveAgent == "" {
		effectiveAgent = inherited.Agent
	}
	var options []modelOption
	listed, err := models(effectiveAgent)
	unlisted := ""
	if err != nil {
		unlisted = "Could not ask " + effectiveAgent + " for its Models, so only the configured one is offered: " + err.Error()
	}
	for _, model := range listed {
		options = append(options, modelOption{Name: model.Name, Label: model.Label + " (" + model.Name + ")"})
	}
	// The Agent's list is only what the picker offers. A Model written into
	// the file by hand is still shown, so opening the picker never loses it.
	if selected.Model != "" && !slices.ContainsFunc(options, func(option modelOption) bool { return option.Name == selected.Model }) {
		options = append(options, modelOption{Name: selected.Model, Label: selected.Model})
	}
	return taskPicker{
		Task:      task,
		Label:     taskLabels[task][0],
		Hint:      taskLabels[task][1],
		Selected:  selected,
		Inherited: inherited,
		Agents:    agent.Names(),
		Models:    options,
		Unlisted:  unlisted,
		Efforts:   agent.Efforts(),
	}
}

func workspaceChoicesPanel(models catalog, choices workspace.Choices) choicesPanel {
	panel := choicesPanel{ID: "workspace-choices", Action: "/settings/choices"}
	for _, task := range workspace.Tasks() {
		panel.Tasks = append(panel.Tasks, newTaskPicker(models, task, choices.For(task), workspace.Choice{}))
	}
	return panel
}

func bookChoicesPanel(models catalog, config workspace.Config, series library.Series, book library.Book) choicesPanel {
	panel := choicesPanel{ID: "book-choices", Action: "/series/" + series.Code + "/books/" + book.Code + "/choices", Book: true}
	for _, task := range workspace.Tasks() {
		panel.Tasks = append(panel.Tasks, newTaskPicker(models, task, book.Override.For(task), config.Choices.For(task)))
	}
	return panel
}

// postedChoice reads one Task's pickers from a submitted panel.
func postedChoice(r *http.Request, task workspace.Task) workspace.Choice {
	return workspace.Choice{
		Agent:  r.FormValue(string(task) + "_agent"),
		Model:  r.FormValue(string(task) + "_model"),
		Effort: agent.Effort(r.FormValue(string(task) + "_effort")),
	}
}

// followAgent keeps a Model that belongs to the Agent picked alongside it.
// Model names are an Agent's own vocabulary, so when the Agent changes, a
// Model the new Agent does not list is replaced with that Agent's built-in
// one rather than saved as a pairing that fails on the first request.
func followAgent(models catalog, task workspace.Task, before, after workspace.Choice) workspace.Choice {
	if after.Agent == "" || after.Agent == before.Agent {
		return after
	}
	listed, _ := models(after.Agent)
	if slices.ContainsFunc(listed, func(model agent.ModelInfo) bool { return model.Name == after.Model }) {
		return after
	}
	after.Model = workspace.BuiltInModel(after.Agent, task)
	return after
}

func (s screens) setWorkspaceChoices(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.session.Current()
	if !ok {
		s.reply(w, r, form{})
		return
	}
	for _, task := range workspace.Tasks() {
		current := ws.Config.Choices.For(task)
		choice := followAgent(s.catalogs.models, task, current, postedChoice(r, task))
		if choice == current {
			continue
		}
		if err := s.session.SetChoice(task, choice); err != nil {
			panel := workspaceChoicesPanel(s.catalogs.models, ws.Config.Choices)
			panel.Problem = err.Error()
			fragment(w, "choices-panel", panel)
			return
		}
	}
	ws, _ = s.session.Current()
	panel := workspaceChoicesPanel(s.catalogs.models, ws.Config.Choices)
	panel.Saved = true
	fragment(w, "choices-panel", panel)
}

func (s screens) setBookChoices(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store(w, r)
	if !ok {
		return
	}
	ws, _ := s.session.Current()
	lib, err := store.Library()
	if err != nil {
		s.detail(w, r, err.Error())
		return
	}
	series, book, found := lib.Book(r.PathValue("series"), r.PathValue("book"))
	if !found {
		http.NotFound(w, r)
		return
	}

	override := book.Override
	for _, task := range workspace.Tasks() {
		inherited := ws.Config.Choices.For(task)
		before := book.Override.For(task)
		if before.Agent == "" {
			before.Agent = inherited.Agent
		}
		override = override.With(task, followAgent(s.catalogs.models, task, before, postedChoice(r, task)))
	}
	// The whole override is resolved before anything is written, so a pairing
	// that cannot work is refused rather than half-saved.
	if _, err := ws.Config.Resolve(override); err != nil {
		panel := bookChoicesPanel(s.catalogs.models, ws.Config, series, book)
		panel.Problem = err.Error()
		fragment(w, "choices-panel", panel)
		return
	}
	for _, task := range workspace.Tasks() {
		if override.For(task) == book.Override.For(task) {
			continue
		}
		if err := store.SetBookChoice(series.Code, book.Code, task, override.For(task)); err != nil {
			panel := bookChoicesPanel(s.catalogs.models, ws.Config, series, book)
			panel.Problem = err.Error()
			fragment(w, "choices-panel", panel)
			return
		}
	}
	book.Override = override
	panel := bookChoicesPanel(s.catalogs.models, ws.Config, series, book)
	panel.Saved = true
	fragment(w, "choices-panel", panel)
}
