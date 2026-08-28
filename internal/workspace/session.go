package workspace

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ijackua/scriptorium/internal/desktop"
)

// Session is which workspace the running application has open, and how it came
// to have one. It is the whole of the picker flow: the handlers ask it what to
// render and tell it when the user pressed the button.
//
// Restoring happens in NewSession rather than in a method the caller must
// remember to call, so that no entrypoint can construct a Session that has
// silently skipped the workspace the user chose last time.
type Session struct {
	picker   desktop.FolderPicker
	settings *Settings

	// choosing serialises the picker itself. Two requests arriving together
	// must not put two native dialogs on the user's screen, and the dialog
	// blocks for as long as the user takes, so it cannot be held under the
	// lock that Current and Problem contend for.
	choosing sync.Mutex

	mu      sync.Mutex
	current *Workspace
	problem string
}

// NewSession builds a Session and opens the remembered workspace if there is
// one that can still be opened.
func NewSession(picker desktop.FolderPicker, settings *Settings) *Session {
	s := &Session{picker: picker, settings: settings}

	root, err := settings.Root()
	if err != nil {
		s.problem = describe(err)
		return s
	}
	if root == "" {
		return s
	}
	ws, err := Open(root)
	if err != nil {
		// The folder was chosen once and has since moved, been deleted, or
		// gone away with its drive. Say so plainly and fall through to the
		// picker; there is nothing else the user can do about it.
		s.problem = describe(err)
		return s
	}
	s.current = &ws
	return s
}

// Current is the open workspace, if one is open.
func (s *Session) Current() (Workspace, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return Workspace{}, false
	}
	return *s.current, true
}

// Problem is what to tell the user about why there is no workspace open, or
// the empty string when there is nothing to tell them.
func (s *Session) Problem() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.problem
}

// Choose shows the folder picker and opens what the user chooses, remembering
// it for the next launch.
//
// A workspace is only remembered once it has opened, so a folder that turns
// out to be unusable does not become the folder every future launch fails on.
//
// Nothing is returned, because whatever happened is already on the Session for
// the next render to read: an outcome the user needs told about is a Problem,
// and one they do not — dismissing the dialog — is silence.
func (s *Session) Choose() {
	s.choosing.Lock()
	defer s.choosing.Unlock()

	root, err := s.picker.PickFolder()
	if errors.Is(err, desktop.ErrCancelled) {
		return
	}
	if err != nil {
		s.report(fmt.Errorf("choose a folder: %w", err))
		return
	}

	ws, err := Open(root)
	if err != nil {
		s.report(err)
		return
	}
	if err := s.settings.Remember(ws.Root); err != nil {
		s.report(err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.current, s.problem = &ws, ""
}

// SetTargetLanguages updates the open Workspace and makes the new allowlist
// available to this running session immediately.
func (s *Session) SetTargetLanguages(tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return errors.New("no Workspace is open")
	}
	updated, err := s.current.SetTargetLanguages(tags)
	if err != nil {
		return err
	}
	s.current = &updated
	return nil
}

// report records a failure for the user to read, leaving any open workspace
// alone — a failed attempt to open a different folder is no reason to close
// the one that works.
func (s *Session) report(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.problem = describe(err)
}

// describe turns an error into something addressed to the user rather than to
// a log, which for the one error they can act on means saying what to do.
func describe(err error) string {
	if errors.Is(err, ErrRootMissing) {
		return fmt.Sprintf("%s. Choose where your library lives now, or pick a different folder.", err)
	}
	return err.Error()
}
