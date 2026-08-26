// Command scriptorium opens the Scriptorium desktop window.
//
// Wails supplies the window; everything inside it is served by an embedded
// HTTP server bound to loopback (see internal/ui). Wails v2 keeps the webview
// on its own asset protocol and will not follow a navigation to an http URL,
// so the asset handler is a reverse proxy onto the loopback listener rather
// than a redirect to it. Every request the window makes therefore travels over
// the real listener, which is where the origin check lives.
package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/ijackua/scriptorium/internal/library"
	"github.com/ijackua/scriptorium/internal/ui"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	server, err := ui.NewServer(library.Fixture())
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
		Title:  "Scriptorium",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Handler: proxy,
		},
	})
	if err != nil {
		log.Fatalf("scriptorium: %v", err)
	}
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
