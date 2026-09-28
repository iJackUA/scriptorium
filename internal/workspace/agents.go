package workspace

import (
	"fmt"
	"slices"

	"github.com/ijackua/scriptorium/internal/agent"
)

// Models are the Models used per kind of task, named in the Agent's own
// vocabulary. Mechanical work is cheap and high-volume; translation is where
// quality is the entire point.
type Models struct {
	Mechanical  string `toml:"mechanical"`
	Translation string `toml:"translation"`
}

// AgentSettings is the Agent a Book's work is sent to and the Models it asks
// that Agent for. As an override, empty fields inherit.
type AgentSettings struct {
	Agent  string
	Models Models
}

// Settings resolves the Agent and Models for a Book whose book.toml carries
// override, which may be empty. The override wins field by field.
//
// Models inherit only within one Agent's namespace: a Book that names another
// Agent takes that Agent's Models from workspace.toml, never the default
// Agent's. A pairing with nothing to inherit is rejected here, before any
// request, rather than discovered as a failed request halfway through a book.
func (c Config) Settings(override AgentSettings) (AgentSettings, error) {
	resolved := AgentSettings{Agent: c.Agent, Models: c.Agents[c.Agent]}
	if override.Agent != "" {
		resolved = AgentSettings{Agent: override.Agent, Models: c.Agents[override.Agent]}
	}
	if override.Models.Mechanical != "" {
		resolved.Models.Mechanical = override.Models.Mechanical
	}
	if override.Models.Translation != "" {
		resolved.Models.Translation = override.Models.Translation
	}

	for _, task := range []struct {
		name          string
		model, borrow string
	}{
		{"mechanical", resolved.Models.Mechanical, c.Agents[c.Agent].Mechanical},
		{"translation", resolved.Models.Translation, c.Agents[c.Agent].Translation},
	} {
		if task.model != "" {
			continue
		}
		if resolved.Agent != c.Agent && task.borrow != "" {
			return AgentSettings{}, fmt.Errorf("Agent %q has no %s Model of its own, and %q is the %s Model of Agent %q, which does not belong with it; set the %s Model in book.toml or under [agents.%s] in %s",
				resolved.Agent, task.name, task.borrow, task.name, c.Agent, task.name, resolved.Agent, ConfigFile)
		}
		return AgentSettings{}, fmt.Errorf("Agent %q has no %s Model; set it under [agents.%s] in %s", resolved.Agent, task.name, resolved.Agent, ConfigFile)
	}
	return resolved, nil
}

// builtInModels are the Models each known Agent uses where workspace.toml
// names none, so a hand-trimmed config still works with no setup. They match
// what the default workspace.toml writes out.
var builtInModels = map[string]Models{
	agent.ClaudeName: {Mechanical: "claude-haiku-4-5", Translation: "claude-opus-5"},
}

// resolveAgents validates the per-Agent Models of a decoded workspace.toml,
// folds a legacy top-level [models] table into the default Agent's, and fills
// any Model left unset from the built-in defaults of its own Agent.
func resolveAgents(config *Config, legacy Models) error {
	if config.Agents == nil {
		config.Agents = map[string]Models{}
	}
	if legacy != (Models{}) {
		if _, ok := config.Agents[config.Agent]; ok {
			return fmt.Errorf("both [models] and [agents.%s] set Models for Agent %q; keep only [agents.%s]", config.Agent, config.Agent, config.Agent)
		}
		config.Agents[config.Agent] = legacy
	}

	names := make([]string, 0, len(config.Agents))
	for name := range config.Agents {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := agent.ValidateName(name); err != nil {
			return fmt.Errorf("[agents.%s]: %w", name, err)
		}
	}
	for name, defaults := range builtInModels {
		models := config.Agents[name]
		if models.Mechanical == "" {
			models.Mechanical = defaults.Mechanical
		}
		if models.Translation == "" {
			models.Translation = defaults.Translation
		}
		config.Agents[name] = models
	}
	return nil
}
