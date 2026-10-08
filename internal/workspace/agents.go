package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/tomledit"
)

// Task is a kind of work sent to an Agent. Each is configured on its own,
// because mechanical work is cheap and high-volume while translation is where
// quality is the entire point.
type Task string

const (
	// TaskMechanical is metadata inference and Dictionary Building.
	TaskMechanical Task = "mechanical"
	// TaskTranslation is translating Chunks.
	TaskTranslation Task = "translation"
)

// Tasks lists every Task in the order they are shown.
func Tasks() []Task { return []Task{TaskMechanical, TaskTranslation} }

// Choice is the Agent, Model and Effort one Task is sent with. The Model is
// named in that Agent's own vocabulary and means nothing to any other Agent.
// As an override, empty fields inherit.
type Choice struct {
	Agent  string       `toml:"agent"`
	Model  string       `toml:"model"`
	Effort agent.Effort `toml:"effort"`
}

// Choices holds a Choice per Task.
type Choices struct {
	Mechanical  Choice `toml:"mechanical"`
	Translation Choice `toml:"translation"`
}

// For returns the Choice for task.
func (c Choices) For(task Task) Choice {
	if task == TaskMechanical {
		return c.Mechanical
	}
	return c.Translation
}

// With returns c with task's Choice replaced.
func (c Choices) With(task Task, choice Choice) Choices {
	if task == TaskMechanical {
		c.Mechanical = choice
	} else {
		c.Translation = choice
	}
	return c
}

// BuiltInModel is the Model a Task uses with agentName when nothing names
// one, so a hand-trimmed config still works with no setup. It matches what
// the default workspace.toml writes out.
func BuiltInModel(agentName string, _ Task) string {
	if agentName != agent.ClaudeName {
		return ""
	}
	return "claude-haiku-5-5"
}

// builtInEffort is the Effort a Task uses when nothing names one.
func builtInEffort(Task) agent.Effort {
	return agent.EffortMedium
}

// Resolve applies a Book's override to the workspace Choices, field by field.
//
// Models never carry across Agents: a Book that switches a Task to another
// Agent must name that Agent's Model too. The pairing is rejected here, before
// any request, rather than discovered as a failed request halfway through a
// Book.
func (c Config) Resolve(override Choices) (Choices, error) {
	resolved := c.Choices
	for _, task := range Tasks() {
		choice, set := c.Choices.For(task), override.For(task)
		if set.Agent != "" {
			if err := agent.ValidateName(set.Agent); err != nil {
				return Choices{}, fmt.Errorf("[%s]: %w", task, err)
			}
			if set.Agent != choice.Agent && set.Model == "" {
				return Choices{}, fmt.Errorf("[%s] sets Agent %q but no Model, and %q is a Model of Agent %q, which does not belong with it; set the %s model too", task, set.Agent, choice.Model, choice.Agent, task)
			}
			choice.Agent = set.Agent
		}
		if set.Model != "" {
			choice.Model = set.Model
		}
		if set.Effort != "" {
			if err := agent.ValidateEffort(set.Effort); err != nil {
				return Choices{}, fmt.Errorf("[%s]: %w", task, err)
			}
			choice.Effort = set.Effort
		}
		resolved = resolved.With(task, choice)
	}
	return resolved, nil
}

// legacyConfig is how workspace.toml named its Agent and Models before each
// Task had its own table. Scriptorium never rewrites the file, so these are
// still read, underneath the [mechanical] and [translation] tables.
type legacyConfig struct {
	Agent  string                  `toml:"agent"`
	Models legacyModels            `toml:"models"`
	Agents map[string]legacyModels `toml:"agents"`
}

type legacyModels struct {
	Mechanical  string `toml:"mechanical"`
	Translation string `toml:"translation"`
}

func (m legacyModels) For(task Task) string {
	if task == TaskMechanical {
		return m.Mechanical
	}
	return m.Translation
}

// resolveChoices fills every field of the workspace Choices: from the Task's
// own table, then the legacy settings, then the built-in defaults. A legacy
// Model is inherited only by a Task still on the Agent it was written for.
func resolveChoices(config *Config, legacy legacyConfig) error {
	legacyAgent := legacy.Agent
	if legacyAgent == "" {
		legacyAgent = agent.ClaudeName
	}
	if err := agent.ValidateName(legacyAgent); err != nil {
		return err
	}
	names := make([]string, 0, len(legacy.Agents))
	for name := range legacy.Agents {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := agent.ValidateName(name); err != nil {
			return fmt.Errorf("[agents.%s]: %w", name, err)
		}
	}

	for _, task := range Tasks() {
		choice := config.Choices.For(task)
		if choice.Agent == "" {
			choice.Agent = legacyAgent
		}
		if err := agent.ValidateName(choice.Agent); err != nil {
			return fmt.Errorf("[%s]: %w", task, err)
		}
		if choice.Model == "" {
			choice.Model = legacy.Agents[choice.Agent].For(task)
		}
		if choice.Model == "" && choice.Agent == legacyAgent {
			choice.Model = legacy.Models.For(task)
		}
		if choice.Model == "" {
			choice.Model = BuiltInModel(choice.Agent, task)
		}
		if choice.Model == "" {
			return fmt.Errorf("[%s] names Agent %q but no Model for it", task, choice.Agent)
		}
		if choice.Effort == "" {
			choice.Effort = builtInEffort(task)
		}
		if err := agent.ValidateEffort(choice.Effort); err != nil {
			return fmt.Errorf("[%s]: %w", task, err)
		}
		config.Choices = config.Choices.With(task, choice)
	}
	return nil
}

// SetChoice records the workspace default for one Task and returns the
// immediately-applicable Workspace. Only that Task's three keys are written,
// so everything else in the hand-written file is kept as it was.
func (w Workspace) SetChoice(task Task, choice Choice) (Workspace, error) {
	if !slices.Contains(Tasks(), task) {
		return Workspace{}, fmt.Errorf("unknown task %q", task)
	}
	if err := agent.ValidateName(choice.Agent); err != nil {
		return Workspace{}, err
	}
	if choice.Model == "" {
		return Workspace{}, fmt.Errorf("the %s Model is required", task)
	}
	if err := agent.ValidateEffort(choice.Effort); err != nil {
		return Workspace{}, err
	}

	path := filepath.Join(w.Root, ConfigFile)
	body, err := os.ReadFile(path)
	if err != nil {
		return Workspace{}, fmt.Errorf("read %s: %w", ConfigFile, err)
	}
	updated := string(body)
	for _, field := range []struct{ key, value string }{
		{"agent", choice.Agent},
		{"model", choice.Model},
		{"effort", string(choice.Effort)},
	} {
		updated = tomledit.Set(updated, string(task), field.key, strconv.Quote(field.value))
	}
	if err := writeConfigAtomic(w.Root, path, updated); err != nil {
		return Workspace{}, err
	}
	return Open(w.Root)
}
