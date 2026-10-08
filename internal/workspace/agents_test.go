package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ijackua/scriptorium/internal/agent"
)

func openConfig(t *testing.T, body string) (Workspace, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ConfigFile), []byte(body), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	return Open(root)
}

func TestTheDefaultConfigChoosesHaikuAtMediumEffortForBothTasks(t *testing.T) {
	ws, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	mechanical, translation := ws.Config.Mechanical, ws.Config.Translation
	if mechanical != (Choice{Agent: "claude", Model: "claude-haiku-5-5", Effort: agent.EffortMedium}) {
		t.Errorf("mechanical = %+v, want Haiku 5.5 at medium Effort", mechanical)
	}
	if translation != (Choice{Agent: "claude", Model: "claude-haiku-5-5", Effort: agent.EffortMedium}) {
		t.Errorf("translation = %+v, want Haiku 5.5 at medium Effort", translation)
	}
	written := read(t, filepath.Join(ws.Root, ConfigFile))
	for _, want := range []string{"[mechanical]", "[translation]"} {
		if !strings.Contains(written, want) {
			t.Errorf("workspace.toml does not carry %s:\n%s", want, written)
		}
	}
}

func TestEachTaskReadsItsOwnTable(t *testing.T) {
	ws, err := openConfig(t, "languages = []\n\n[mechanical]\nagent = \"claude\"\nmodel = \"cheap\"\neffort = \"medium\"\n\n[translation]\nmodel = \"strong\"\neffort = \"max\"\n")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := ws.Config.Mechanical; got != (Choice{Agent: "claude", Model: "cheap", Effort: agent.EffortMedium}) {
		t.Errorf("mechanical = %+v", got)
	}
	if got := ws.Config.Translation; got != (Choice{Agent: "claude", Model: "strong", Effort: agent.EffortMax}) {
		t.Errorf("translation = %+v", got)
	}
}

// Workspaces written before each Task had its own table name one Agent and a
// [models] or [agents.<name>] table, and Scriptorium never rewrites
// workspace.toml, so those must still be read.
func TestLegacyAgentAndModelsAreStillRead(t *testing.T) {
	for name, body := range map[string]string{
		"[models]":        "agent = \"claude\"\n\n[models]\nmechanical = \"cheap\"\ntranslation = \"strong\"\n",
		"[agents.claude]": "agent = \"claude\"\n\n[agents.claude]\nmechanical = \"cheap\"\ntranslation = \"strong\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			ws, err := openConfig(t, body)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if ws.Config.Mechanical.Model != "cheap" || ws.Config.Translation.Model != "strong" {
				t.Errorf("Choices = %+v, want the legacy Models", ws.Config.Choices)
			}
		})
	}
}

func TestAnUnsetFieldFallsBackToItsBuiltInDefault(t *testing.T) {
	ws, err := openConfig(t, "[translation]\nmodel = \"strong\"\n")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := ws.Config.Translation; got != (Choice{Agent: "claude", Model: "strong", Effort: agent.EffortMedium}) {
		t.Errorf("translation = %+v, want the built-in Agent and Effort around the configured Model", got)
	}
	if got := ws.Config.Mechanical.Model; got != BuiltInModel("claude", TaskMechanical) {
		t.Errorf("mechanical Model = %q, want the built-in one", got)
	}
}

func TestUnknownAgentsAndEffortsAreRejectedOnOpen(t *testing.T) {
	for body, want := range map[string]string{
		"[translation]\nagent = \"gpt\"\n":     "gpt",
		"[mechanical]\neffort = \"extreme\"\n": "extreme",
		"[agents.gpt]\ntranslation = \"x\"\n":  "gpt",
		"agent = \"codex\"\n":                  "codex",
	} {
		if _, err := openConfig(t, body); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Open(%q) = %v, want %q named", body, err, want)
		}
	}
}

func TestSettingAChoiceRewritesOnlyItsTable(t *testing.T) {
	ws, err := openConfig(t, "# my own note, please keep\nlanguages = [\"de\"]\n\n[translation]\n# the strong one\nmodel = \"strong\"\n")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	updated, err := ws.SetChoice(TaskMechanical, Choice{Agent: "claude", Model: "claude-sonnet-5", Effort: agent.EffortHigh})
	if err != nil {
		t.Fatalf("SetChoice: %v", err)
	}
	if got := updated.Config.Mechanical; got != (Choice{Agent: "claude", Model: "claude-sonnet-5", Effort: agent.EffortHigh}) {
		t.Errorf("mechanical = %+v, want the saved Choice applied immediately", got)
	}
	written := read(t, filepath.Join(ws.Root, ConfigFile))
	for _, want := range []string{"# my own note, please keep", "# the strong one", "model = \"strong\"", "[mechanical]\n", "model = \"claude-sonnet-5\""} {
		if !strings.Contains(written, want) {
			t.Errorf("workspace.toml does not keep %q:\n%s", want, written)
		}
	}
	reopened, err := Open(ws.Root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.Config.Choices != updated.Config.Choices {
		t.Errorf("reopened Choices = %+v, want %+v", reopened.Config.Choices, updated.Config.Choices)
	}
}

func TestAnInvalidChoiceIsNotSaved(t *testing.T) {
	ws, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	before := read(t, filepath.Join(ws.Root, ConfigFile))
	for _, choice := range []Choice{
		{Agent: "gpt", Model: "x", Effort: agent.EffortLow},
		{Agent: "claude", Model: "", Effort: agent.EffortLow},
		{Agent: "claude", Model: "x", Effort: "extreme"},
	} {
		if _, err := ws.SetChoice(TaskTranslation, choice); err == nil {
			t.Errorf("SetChoice(%+v) succeeded", choice)
		}
	}
	if read(t, filepath.Join(ws.Root, ConfigFile)) != before {
		t.Error("a rejected Choice rewrote workspace.toml")
	}
}

func TestABookOverrideWinsFieldByField(t *testing.T) {
	config := Config{Choices: Choices{
		Mechanical:  Choice{Agent: "claude", Model: "cheap", Effort: agent.EffortLow},
		Translation: Choice{Agent: "claude", Model: "strong", Effort: agent.EffortMedium},
	}}

	resolved, err := config.Resolve(Choices{Translation: Choice{Model: "strongest", Effort: agent.EffortMax}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Translation != (Choice{Agent: "claude", Model: "strongest", Effort: agent.EffortMax}) {
		t.Errorf("translation = %+v, want the override over the inherited Agent", resolved.Translation)
	}
	if resolved.Mechanical != config.Mechanical {
		t.Errorf("mechanical = %+v, want it inherited untouched", resolved.Mechanical)
	}
}

// Model names are an Agent's own vocabulary. A Book that switches a Task to
// another Agent must not quietly borrow the workspace's Model for it and fail
// halfway through a Book.
func TestAnOverriddenAgentWithoutAModelIsRejected(t *testing.T) {
	config := Config{Choices: Choices{
		Mechanical:  Choice{Agent: "claude", Model: "claude-haiku-4-5", Effort: agent.EffortLow},
		Translation: Choice{Agent: "codex", Model: "gpt-strong", Effort: agent.EffortMedium},
	}}

	_, err := config.Resolve(Choices{Translation: Choice{Agent: "claude"}})
	if err == nil {
		t.Fatal("Resolve accepted claude with codex's Model")
	}
	for _, want := range []string{`"claude"`, `"gpt-strong"`, "translation"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to name %s", err, want)
		}
	}
}

func TestAnOverrideNamingItsInheritedAgentNeedsNoModel(t *testing.T) {
	ws, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	resolved, err := ws.Config.Resolve(Choices{Translation: Choice{Agent: "claude"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved != ws.Config.Choices {
		t.Errorf("Resolve = %+v, want the workspace Choices", resolved)
	}
}
