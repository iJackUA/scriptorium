package library

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ijackua/scriptorium/internal/agent"
	"github.com/ijackua/scriptorium/internal/tomledit"
	"github.com/ijackua/scriptorium/internal/workspace"
)

// bookChoices is the part of book.toml that overrides the workspace's Agent,
// Model and Effort per Task. The top-level agent and [models] table are the
// older form of the same override, still read underneath the Task tables.
type bookChoices struct {
	Mechanical   workspace.Choice `toml:"mechanical"`
	Translation  workspace.Choice `toml:"translation"`
	LegacyAgent  string           `toml:"agent"`
	LegacyModels struct {
		Mechanical  string `toml:"mechanical"`
		Translation string `toml:"translation"`
	} `toml:"models"`
}

// override folds the legacy form under the Task tables, so the rest of the
// application sees one override per Task.
func (b bookChoices) override() workspace.Choices {
	override := workspace.Choices{Mechanical: b.Mechanical, Translation: b.Translation}
	for _, task := range workspace.Tasks() {
		choice := override.For(task)
		if choice.Agent == "" {
			choice.Agent = b.LegacyAgent
		}
		if choice.Model == "" {
			choice.Model = b.legacyModel(task)
		}
		override = override.With(task, choice)
	}
	return override
}

func (b bookChoices) legacyModel(task workspace.Task) string {
	if task == workspace.TaskMechanical {
		return b.LegacyModels.Mechanical
	}
	return b.LegacyModels.Translation
}

// validate rejects an Agent or Effort the current build does not know, when
// book.toml is read rather than when the first request fails.
func (b bookChoices) validate() error {
	for _, task := range workspace.Tasks() {
		if err := validateOverride(task, b.override().For(task)); err != nil {
			return err
		}
	}
	return nil
}

func validateOverride(task workspace.Task, choice workspace.Choice) error {
	if choice.Agent != "" {
		if err := agent.ValidateName(choice.Agent); err != nil {
			return fmt.Errorf("[%s]: %w", task, err)
		}
	}
	if choice.Effort != "" {
		if err := agent.ValidateEffort(choice.Effort); err != nil {
			return fmt.Errorf("[%s]: %w", task, err)
		}
	}
	return nil
}

// Choices resolves the Agent, Model and Effort each Task of this Book uses
// against the workspace defaults. An override that does not pair up is
// reported against the Book's own book.toml, so the user knows which file to
// fix before any request is made.
func (b Book) Choices(config workspace.Config, seriesCode string) (workspace.Choices, error) {
	choices, err := config.Resolve(b.Override)
	if err != nil {
		return workspace.Choices{}, fmt.Errorf("%s: %w", filepath.Join(seriesCode, BooksDir, b.Code, BookFile), err)
	}
	return choices, nil
}

// SetBookChoice records a Book's override for one Task; an empty field goes
// back to inheriting from the workspace. Only that Task's keys are written,
// so the rest of the hand-written book.toml is kept as it was.
//
// A legacy top-level agent or [models] entry is retired as it is replaced:
// left in place, it would quietly keep applying to a field the user has just
// set back to inheriting.
func (s Store) SetBookChoice(seriesCode, bookCode string, task workspace.Task, choice workspace.Choice) error {
	if err := validateOverride(task, choice); err != nil {
		return err
	}
	path, file, err := s.bookFile(seriesCode, bookCode)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", BookFile, err)
	}
	body := string(raw)
	if file.LegacyAgent != "" {
		// The legacy agent covered both Tasks; the other one keeps it.
		tables := workspace.Choices{Mechanical: file.Mechanical, Translation: file.Translation}
		for _, other := range workspace.Tasks() {
			if other != task && tables.For(other).Agent == "" {
				body = tomledit.Set(body, string(other), "agent", strconv.Quote(file.LegacyAgent))
			}
		}
		body = tomledit.Delete(body, "", "agent")
	}
	body = tomledit.Delete(body, "models", string(task))
	for _, field := range []struct{ key, value string }{
		{"agent", choice.Agent},
		{"model", choice.Model},
		{"effort", string(choice.Effort)},
	} {
		if field.value == "" {
			body = tomledit.Delete(body, string(task), field.key)
			continue
		}
		body = tomledit.Set(body, string(task), field.key, strconv.Quote(field.value))
	}
	return writeAtomic(path, []byte(body))
}
