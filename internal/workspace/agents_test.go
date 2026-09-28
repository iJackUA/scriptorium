package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openConfig(t *testing.T, body string) (Workspace, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ConfigFile), []byte(body), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	return Open(root)
}

func TestTheDefaultConfigHoldsModelsForEveryKnownAgent(t *testing.T) {
	ws, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	settings, err := ws.Config.Settings(AgentSettings{})
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if settings.Agent != "claude" {
		t.Errorf("default Agent = %q, want claude", settings.Agent)
	}
	if settings.Models.Mechanical == "" || settings.Models.Translation == "" || settings.Models.Mechanical == settings.Models.Translation {
		t.Errorf("default Models = %+v, want a distinct cheap and strong Model", settings.Models)
	}
	written := read(t, filepath.Join(ws.Root, ConfigFile))
	if !strings.Contains(written, "[agents.claude]") {
		t.Errorf("workspace.toml does not hold Models under the Agent they belong to:\n%s", written)
	}
}

func TestModelsAreReadPerAgent(t *testing.T) {
	ws, err := openConfig(t, "agent = \"claude\"\nlanguages = []\n\n[agents.claude]\nmechanical = \"cheap\"\ntranslation = \"strong\"\n")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	settings, err := ws.Config.Settings(AgentSettings{})
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if settings.Models != (Models{Mechanical: "cheap", Translation: "strong"}) {
		t.Errorf("Models = %+v, want cheap and strong", settings.Models)
	}
}

// Workspaces written before Models were held per Agent have a top-level
// [models] table, and Scriptorium never rewrites workspace.toml, so that
// table must still mean the default Agent's Models.
func TestALegacyModelsTableBelongsToTheDefaultAgent(t *testing.T) {
	ws, err := openConfig(t, "agent = \"claude\"\nlanguages = []\n\n[models]\nmechanical = \"cheap\"\ntranslation = \"strong\"\n")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	settings, err := ws.Config.Settings(AgentSettings{})
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if settings.Models != (Models{Mechanical: "cheap", Translation: "strong"}) {
		t.Errorf("Models = %+v, want the legacy table", settings.Models)
	}
}

func TestModelsForAnUnknownAgentAreRejected(t *testing.T) {
	_, err := openConfig(t, "agent = \"claude\"\nlanguages = []\n\n[agents.claude]\nmechanical = \"a\"\ntranslation = \"b\"\n\n[agents.gpt]\nmechanical = \"c\"\ntranslation = \"d\"\n")
	if err == nil || !strings.Contains(err.Error(), "gpt") {
		t.Fatalf("Open = %v, want the unknown Agent named", err)
	}
}

// The application must work with no setup, including from a workspace.toml
// trimmed down by hand: a known Agent's missing Model falls back to that
// Agent's own built-in default, never to another Agent's.
func TestAnUnsetModelFallsBackToItsAgentsBuiltInDefault(t *testing.T) {
	ws, err := openConfig(t, "agent = \"claude\"\n\n[agents.claude]\nmechanical = \"cheap\"\n")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	settings, err := ws.Config.Settings(AgentSettings{})
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if settings.Models.Mechanical != "cheap" || settings.Models.Translation != builtInModels["claude"].Translation {
		t.Errorf("Models = %+v, want the configured mechanical and the built-in translation Model", settings.Models)
	}
}

func TestTheWrittenDefaultsAreTheBuiltInOnes(t *testing.T) {
	ws, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	written := read(t, filepath.Join(ws.Root, ConfigFile))
	for name, models := range builtInModels {
		for _, model := range []string{models.Mechanical, models.Translation} {
			if !strings.Contains(written, `"`+model+`"`) {
				t.Errorf("workspace.toml does not write %s's built-in Model %q", name, model)
			}
		}
	}
}

func TestABookOverrideWinsModelByModel(t *testing.T) {
	config := Config{Agent: "claude", Agents: map[string]Models{"claude": {Mechanical: "cheap", Translation: "strong"}}}

	settings, err := config.Settings(AgentSettings{Agent: "claude", Models: Models{Translation: "strongest"}})
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if settings != (AgentSettings{Agent: "claude", Models: Models{Mechanical: "cheap", Translation: "strongest"}}) {
		t.Errorf("Settings = %+v, want the translation override over the inherited mechanical Model", settings)
	}
}

func TestAnOverriddenAgentInheritsOnlyItsOwnModels(t *testing.T) {
	config := Config{Agent: "claude", Agents: map[string]Models{
		"claude": {Mechanical: "claude-cheap", Translation: "claude-strong"},
		"codex":  {Mechanical: "codex-cheap", Translation: "codex-strong"},
	}}

	settings, err := config.Settings(AgentSettings{Agent: "codex"})
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if settings.Models != (Models{Mechanical: "codex-cheap", Translation: "codex-strong"}) {
		t.Errorf("Models = %+v, want codex's own Models", settings.Models)
	}
}

// Model names are an Agent's own vocabulary. A Book that switches Agent with
// nothing to inherit in that Agent's namespace must not quietly borrow the
// default Agent's Models and fail halfway through a book.
func TestAnOverriddenAgentWithoutModelsOfItsOwnIsRejected(t *testing.T) {
	config := Config{Agent: "claude", Agents: map[string]Models{"claude": {Mechanical: "claude-cheap", Translation: "claude-strong"}}}

	_, err := config.Settings(AgentSettings{Agent: "codex", Models: Models{Mechanical: "codex-cheap"}})
	if err == nil {
		t.Fatal("Settings accepted codex with claude's translation Model")
	}
	for _, want := range []string{`"codex"`, `"claude-strong"`, "translation"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to name %s", err, want)
		}
	}
}
