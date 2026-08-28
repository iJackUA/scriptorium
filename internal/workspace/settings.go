package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Settings is the one piece of state that cannot live in a workspace, because
// it is what says which workspace to open. It sits in the user's config
// directory and holds nothing else.
type Settings struct {
	path string
}

// NewSettings reads and writes the settings file at path.
func NewSettings(path string) *Settings { return &Settings{path: path} }

// UserSettingsPath is where the settings file belongs on this host.
func UserSettingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate the user config directory: %w", err)
	}
	return filepath.Join(dir, "Scriptorium", "settings.toml"), nil
}

// settingsFile is the on-disk shape of Settings.
type settingsFile struct {
	Workspace string `toml:"workspace"`
}

// Root is the workspace folder last opened. It returns the empty string when
// nothing is remembered, which is what a first launch looks like.
//
// A settings file that exists but cannot be read is not the same thing as no
// settings file, and is not reported as one: a user who chose a folder once
// should hear why they are being asked again rather than be shown a screen
// that pretends they never did. Whether the folder still exists is Open's
// question, not this one's.
func (s *Settings) Root() (string, error) {
	var file settingsFile
	if _, err := toml.DecodeFile(s.path, &file); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read %s: %w", s.path, err)
	}
	return file.Workspace, nil
}

// Remember records root as the workspace to open on the next launch.
func (s *Settings) Remember(root string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create the settings directory: %w", err)
	}

	var body bytes.Buffer
	body.WriteString("# The workspace folder Scriptorium opens on launch.\n")
	if err := toml.NewEncoder(&body).Encode(settingsFile{Workspace: root}); err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := os.WriteFile(s.path, body.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", s.path, err)
	}
	return nil
}
