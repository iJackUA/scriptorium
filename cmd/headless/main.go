// Command headless serves Scriptorium's interface without opening a desktop
// window.
//
// It is the entrypoint verifying agents use to inspect the same handler tree
// as the production application. It deliberately has no native picker and
// uses an ephemeral fixture Workspace rather than the user's library.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/ui"
	"github.com/ijackua/scriptorium/internal/workspace"
)

func main() {
	session, cleanup, err := fixtureSession()
	if err != nil {
		log.Fatalf("headless: %v", err)
	}
	defer cleanup()

	server, err := ui.NewServer(session)
	if err != nil {
		log.Fatalf("headless: %v", err)
	}
	defer server.Close()

	// Keep this as the first output: verification tools can consume it without
	// parsing a log prefix or other prose.
	fmt.Println(server.URL())

	done := make(chan error, 1)
	go func() { done <- server.Serve() }()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	select {
	case <-interrupt:
		return
	case err := <-done:
		if err != nil && !errors.Is(err, os.ErrClosed) {
			log.Fatalf("headless: serve: %v", err)
		}
	}
}

// fixtureSession provides a predictable library for every verification session.
// It is removed when the server exits, and never reads or changes the user's
// saved Workspace.
func fixtureSession() (*workspace.Session, func(), error) {
	root, err := os.MkdirTemp("", "scriptorium-headless-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create fixture Workspace: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(root) }

	settings := workspace.NewSettings(filepath.Join(root, "headless-settings.toml"))
	if err := settings.Remember(root); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("configure fixture Workspace: %w", err)
	}
	session := workspace.NewSession(headlessPicker{}, settings)

	store := library.NewStore(root)
	series, err := store.CreateSeries("The Adventures of Sherlock Holmes", "en")
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("create fixture Series: %w", err)
	}
	for _, book := range []library.BookDraft{
		{Code: "adventures", Title: "The Adventures of Sherlock Holmes", Author: "Arthur Conan Doyle"},
		{Code: "memoirs", Title: "The Memoirs of Sherlock Holmes", Author: "Arthur Conan Doyle"},
		{Code: "return", Title: "The Return of Sherlock Holmes", Author: "Arthur Conan Doyle"},
	} {
		if _, err := store.AddBook(series.Code, book); err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("create fixture Book %q: %w", book.Code, err)
		}
	}
	return session, cleanup, nil
}

// headlessPicker makes the headless Session safe even if a future handler asks
// it to choose a folder.
type headlessPicker struct{}

func (headlessPicker) PickFolder() (string, error) {
	return "", errors.New("choose a Workspace from the desktop application")
}
