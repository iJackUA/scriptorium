// Package workspace opens the folder the library lives in.
//
// The whole library is plain files under one root the user chooses, so that it
// can be backed up, synced, inspected or hand-edited without the application.
// The root carries a workspace.toml holding the defaults that apply to
// everything under it.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ConfigFile is the name of the settings file at the workspace root.
const ConfigFile = "workspace.toml"

// ErrRootMissing reports that the workspace folder is not there any more —
// renamed, deleted, or on a drive that is not mounted. It is separated from
// every other failure because it is the one the user can fix, by choosing a
// folder again.
var ErrRootMissing = errors.New("workspace folder is missing")

// Models are the Models used per kind of task, named in the Agent's own
// vocabulary. Mechanical work is cheap and high-volume; translation is where
// quality is the entire point.
type Models struct {
	Mechanical  string `toml:"mechanical"`
	Translation string `toml:"translation"`
}

// SetTargetLanguages replaces the ordered Target Language allowlist and
// returns the immediately-applicable Workspace configuration. It changes only
// the languages assignment, preserving the rest of the hand-written file.
func (w Workspace) SetTargetLanguages(tags []string) (Workspace, error) {
	if err := validateTargetLanguages(tags); err != nil {
		return Workspace{}, err
	}

	path := filepath.Join(w.Root, ConfigFile)
	body, err := os.ReadFile(path)
	if err != nil {
		return Workspace{}, fmt.Errorf("read %s: %w", ConfigFile, err)
	}
	lines := strings.Split(string(body), "\n")
	replacement := "languages = [" + quotedTags(tags) + "]"
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "languages =") {
			lines[i], found = replacement, true
			break
		}
	}
	if !found {
		return Workspace{}, fmt.Errorf("read %s: no languages setting", ConfigFile)
	}
	updated := strings.Join(lines, "\n")
	temporary, err := os.CreateTemp(w.Root, ".workspace.toml-*")
	if err != nil {
		return Workspace{}, fmt.Errorf("write %s: %w", ConfigFile, err)
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.WriteString(updated); err != nil {
		temporary.Close()
		return Workspace{}, fmt.Errorf("write %s: %w", ConfigFile, err)
	}
	if err := temporary.Close(); err != nil {
		return Workspace{}, fmt.Errorf("write %s: %w", ConfigFile, err)
	}
	if err := os.Rename(name, path); err != nil {
		return Workspace{}, fmt.Errorf("write %s: %w", ConfigFile, err)
	}
	return Open(w.Root)
}

func validateTargetLanguages(tags []string) error {
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if _, ok := LanguageFor(tag); !ok {
			return fmt.Errorf("%q is not a canonical ISO 639-1 language tag", tag)
		}
		if seen[tag] {
			return fmt.Errorf("target language %q is listed more than once", tag)
		}
		seen[tag] = true
	}
	return nil
}

func quotedTags(tags []string) string {
	quoted := make([]string, 0, len(tags))
	for _, tag := range tags {
		quoted = append(quoted, `"`+tag+`"`)
	}
	return strings.Join(quoted, ", ")
}

// Config is the contents of workspace.toml: the defaults every Series and Book
// in this workspace inherits.
type Config struct {
	Agent     string   `toml:"agent"`
	Languages []string `toml:"languages"`
	Models    Models   `toml:"models"`
}

// Workspace is an opened workspace root together with its Config.
type Workspace struct {
	Root   string
	Config Config
}

// Open opens the workspace rooted at root, writing a default workspace.toml if
// the folder does not have one yet.
//
// An existing config is read and never rewritten. That is what makes the
// choice of TOML pay off: the file is meant to be hand-edited, and comments
// only survive if nothing re-encodes them. Any future change to the config has
// to preserve that, which is why there is no Save here to reach for.
func Open(root string) (Workspace, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Workspace{}, fmt.Errorf("resolve workspace folder: %w", err)
	}

	info, err := os.Stat(root)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return Workspace{}, fmt.Errorf("%s: %w", root, ErrRootMissing)
	case err != nil:
		return Workspace{}, fmt.Errorf("open workspace folder %s: %w", root, err)
	case !info.IsDir():
		return Workspace{}, fmt.Errorf("%s is a file, not a folder", root)
	}

	path := filepath.Join(root, ConfigFile)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		// Writing the config is also how a folder proves it is writable, which
		// it has to be for anything else in the workspace to work.
		if err := os.WriteFile(path, []byte(defaultConfig), 0o644); err != nil {
			return Workspace{}, fmt.Errorf("create %s: %w", ConfigFile, err)
		}
	} else if err != nil {
		return Workspace{}, fmt.Errorf("read %s: %w", ConfigFile, err)
	}

	// The freshly written file is decoded rather than trusted, so a created
	// workspace and an opened one cannot disagree about the defaults.
	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return Workspace{}, fmt.Errorf("read %s: %w", ConfigFile, err)
	}
	if err := validateTargetLanguages(config.Languages); err != nil {
		return Workspace{}, fmt.Errorf("read %s: %w", ConfigFile, err)
	}
	return Workspace{Root: root, Config: config}, nil
}

// defaultConfig is written verbatim, comments and all, because this file is
// addressed to the person who will edit it rather than to the encoder.
const defaultConfig = `# Scriptorium workspace settings.
#
# Everything under this folder — every Series, Book, Dictionary and translation
# — is a plain file, and this is where the defaults for all of them live. Edit
# it by hand whenever you like; Scriptorium reads this file and never rewrites
# it, so these comments stay where you put them.

# The Agent is the command-line AI tool Scriptorium spawns to translate.
# "claude" is the only value with an implementation today.
agent = "claude"

# The target languages offered when adding a Translation Target to a Book.
# Use canonical ISO 639-1 tags, for example "uk" for Ukrainian.
languages = []

# Models are named in the Agent's own vocabulary — these are what the claude
# CLI accepts for --model. A Book may override them.
[models]
# Mechanical work: inferring metadata, proposing Dictionary Terms. High volume,
# low stakes, so this wants to be the cheap one.
mechanical = "claude-haiku-4-5"
# Translation itself, where quality is the whole point.
translation = "claude-opus-5"
`
