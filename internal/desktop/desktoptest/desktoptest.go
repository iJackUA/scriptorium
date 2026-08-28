// Package desktoptest provides a FolderPicker whose answer a test decides.
//
// It lives outside _test.go so that every package driving the picker flow —
// the session that owns it and the handlers that render around it — shares one
// fake rather than each keeping its own copy in step with the interface.
package desktoptest

// Picker answers PickFolder with whatever the test put in it.
type Picker struct {
	Answer string
	Err    error
	Calls  int
}

func (p *Picker) PickFolder() (string, error) {
	p.Calls++
	return p.Answer, p.Err
}
