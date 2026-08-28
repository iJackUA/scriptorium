// Package desktop is the shim over the parts of the host desktop that only the
// production binary can reach.
//
// It exists so the rest of the application depends on an interface rather than
// on Wails. A native folder picker needs a real window and a real user, and
// neither is present when the interface is driven headlessly for verification
// (ADR-0004), so the picker arrives as a constructor argument and the headless
// twin passes something that answers without one.
package desktop

import "errors"

// ErrCancelled reports that the user dismissed the picker without choosing a
// folder. It is an ordinary outcome, not a failure: nothing should change and
// nothing should be reported to the user, who already knows what they did.
var ErrCancelled = errors.New("desktop: folder selection cancelled")

// FolderPicker presents the host's native folder chooser.
//
// The interface is one method because one method is all that is needed today.
// Revealing a folder in the file manager belongs here too, and joins it with
// the ticket that first has a folder worth revealing.
type FolderPicker interface {
	// PickFolder asks the user for a directory, returning its absolute path.
	// It returns ErrCancelled if the user dismissed the picker.
	//
	// What the dialog is titled is the implementation's business, because it
	// is the implementation that knows what kind of dialog it is putting up.
	PickFolder() (string, error)
}
