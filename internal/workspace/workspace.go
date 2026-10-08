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
	if err := writeConfigAtomic(w.Root, path, strings.Join(lines, "\n")); err != nil {
		return Workspace{}, err
	}
	return Open(w.Root)
}

// writeConfigAtomic replaces workspace.toml through a temporary file, so a
// crash mid-write cannot leave the hand-written file half there.
func writeConfigAtomic(root, path, body string) error {
	temporary, err := os.CreateTemp(root, ".workspace.toml-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", ConfigFile, err)
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.WriteString(body); err != nil {
		temporary.Close()
		return fmt.Errorf("write %s: %w", ConfigFile, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("write %s: %w", ConfigFile, err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("write %s: %w", ConfigFile, err)
	}
	return nil
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
	Languages                     []string `toml:"languages"`
	DictionaryOccurrenceThreshold int      `toml:"dictionary_occurrence_threshold"`
	// Choices is the default Agent, Model and Effort of each Task, with every
	// field filled. Apply a Book's override through Resolve.
	Choices
}

// configFile is the on-disk shape of workspace.toml: Config plus the settings
// written before each Task had its own table.
type configFile struct {
	Config
	legacyConfig
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
	var file configFile
	if _, err := toml.DecodeFile(path, &file); err != nil {
		return Workspace{}, fmt.Errorf("read %s: %w", ConfigFile, err)
	}
	config := file.Config
	if err := resolveChoices(&config, file.legacyConfig); err != nil {
		return Workspace{}, fmt.Errorf("read %s: %w", ConfigFile, err)
	}
	if err := validateTargetLanguages(config.Languages); err != nil {
		return Workspace{}, fmt.Errorf("read %s: %w", ConfigFile, err)
	}
	if config.DictionaryOccurrenceThreshold <= 0 {
		// Config files predate Dictionary Building. Treat an omitted value as
		// today's default without rewriting a hand-maintained file.
		config.DictionaryOccurrenceThreshold = 2
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

# The target languages offered when adding a Translation Target to a Book.
# Use canonical ISO 639-1 tags, for example "uk" for Ukrainian.
languages = []

# A proposed Term must appear in at least this many Chunks before it reaches
# Dictionary review. Raise it to shrink a noisy Dictionary; lower it to keep
# less frequent recurring Terms.
dictionary_occurrence_threshold = 2

# Each kind of work picks its own Agent, Model and Effort. The Agent is the
# command-line AI tool Scriptorium spawns; "claude" is the only value with an
# implementation today. Models are named in the Agent's own vocabulary — these
# are what the claude CLI accepts for --model. Effort is low, medium, high,
# xhigh or max. A Book may override any of them in its book.toml.

# Mechanical work: inferring metadata, proposing Dictionary Terms. High volume,
# low stakes.
[mechanical]
agent = "claude"
model = "claude-haiku-5-5"
effort = "medium"

# Translation itself.
[translation]
agent = "claude"
model = "claude-haiku-5-5"
effort = "medium"
`
