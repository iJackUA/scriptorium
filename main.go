// Command scriptorium opens the Scriptorium desktop window.
//
// Wails supplies the window; everything inside it is served by an embedded
// HTTP server bound to loopback (see internal/ui). Wails v2 keeps the webview
// on its own asset protocol and will not follow a navigation to an http URL,
// so the asset handler is a reverse proxy onto the loopback listener rather
// than a redirect to it. Every request the window makes therefore travels over
// the real listener, which is where the origin check lives.
//
// The native folder picker lives here rather than in internal/, because this
// is the only binary that has a window to hang a dialog from (ADR-0004).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"github.com/ijackua/scriptorium/internal/desktop"
	"github.com/ijackua/scriptorium/internal/ui"
	"github.com/ijackua/scriptorium/internal/workspace"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func main() {
	settingsPath, err := workspace.UserSettingsPath()
	if err != nil {
		log.Fatalf("scriptorium: %v", err)
	}
	picker := &windowPicker{}
	session := workspace.NewSession(picker, workspace.NewSettings(settingsPath))

	server, err := ui.NewServer(session)
	if err != nil {
		log.Fatalf("scriptorium: %v", err)
	}
	go func() {
		if err := server.Serve(); err != nil {
			log.Fatalf("scriptorium: serve: %v", err)
		}
	}()
	log.Printf("scriptorium: serving on %s", server.URL())

	proxy, err := windowProxy(server.URL())
	if err != nil {
		log.Fatalf("scriptorium: %v", err)
	}

	err = wails.Run(&options.App{
		Title:     "Scriptorium",
		Width:     1200,
		Height:    800,
		OnStartup: picker.attach,
		AssetServer: &assetserver.Options{
			Handler: proxy,
		},
	})
	if err != nil {
		log.Fatalf("scriptorium: %v", err)
	}
}

// windowPicker is the native folder chooser, presented from the application
// window.
//
// Wails hands out the context that dialogs need only once the window exists,
// which is after the server is already serving — so the picker is constructed
// empty and attached at startup. Until then it declines rather than crashing
// the window, which is the difference between a user seeing a message and a
// user seeing nothing.
type windowPicker struct {
	mu  sync.RWMutex
	ctx context.Context
}

func (p *windowPicker) attach(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ctx = ctx
}

func (p *windowPicker) PickFolder() (string, error) {
	p.mu.RLock()
	ctx := p.ctx
	p.mu.RUnlock()
	if ctx == nil {
		return "", errors.New("the window is not ready yet")
	}

	folder, err := wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
		Title: "Choose a folder for your Scriptorium library",
	})
	if err != nil {
		return "", fmt.Errorf("open the folder chooser: %w", err)
	}
	if folder == "" {
		return "", desktop.ErrCancelled
	}
	return folder, nil
}

// windowProxy forwards what the window asks for to the embedded server.
//
// It presents the server's own origin, because a request arriving through this
// proxy came from inside this process and is trusted by construction. The
// origin check exists to stop everything else on the machine — a page in the
// user's browser, which can reach a loopback port but cannot forge an Origin.
func windowProxy(serverURL string) (http.Handler, error) {
	target, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.Host = target.Host
		r.Header.Set("Origin", serverURL)
	}
	return proxy, nil
}
