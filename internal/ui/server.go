// Package ui serves the Scriptorium interface over an embedded HTTP server.
//
// The server exists because the application is fundamentally a progress
// dashboard over long-running jobs, and server-sent events are the cheapest
// way to carry that progress. It binds to loopback on a port the operating
// system chooses, and serves only requests that came from the window.
package ui

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/ijackua/scriptorium/internal/library"
)

// Server is the embedded HTTP server together with the loopback listener it
// was given.
type Server struct {
	listener net.Listener
	handler  http.Handler
	http     *http.Server
}

// NewServer binds a listener to loopback on an operating-system chosen port
// and builds the handler tree over lib.
func NewServer(lib library.Library) (*Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("bind loopback listener: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	local := map[string]struct{}{
		fmt.Sprintf("127.0.0.1:%d", port): {},
		fmt.Sprintf("localhost:%d", port): {},
	}
	origins := map[string]struct{}{}
	for host := range local {
		origins["http://"+host] = struct{}{}
	}

	s := &Server{
		listener: listener,
		handler:  requireLocalCaller(local, origins, routes(lib)),
	}
	s.http = &http.Server{
		Handler: s.handler,
		// A long-lived request is a stream of progress events, so ReadTimeout
		// and WriteTimeout must stay off. These two only bound connections
		// that never get as far as a request.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	return s, nil
}

// URL is the address the window should be pointed at.
func (s *Server) URL() string {
	return "http://" + s.listener.Addr().String()
}

// Handler is the fully wrapped handler tree, exposed for tests.
func (s *Server) Handler() http.Handler { return s.handler }

// Serve blocks serving requests on the loopback listener.
func (s *Server) Serve() error { return s.http.Serve(s.listener) }

// Close stops the server and releases the loopback port.
func (s *Server) Close() error { return s.http.Close() }

// requireLocalCaller serves only requests that came from the window.
//
// Two things are checked, because either alone leaves a hole:
//
//   - The Origin header, when present, must name this server. This is what
//     stops a page the user is browsing from calling in over loopback.
//
//   - The Host header must name this server too. A same-origin request carries
//     no Origin at all, and a page on an attacker's domain whose DNS is
//     rebound to 127.0.0.1 is treated by the browser as same-origin — so the
//     Origin check alone waves it straight through. What such a request cannot
//     disguise is the Host it was addressed to.
func requireLocalCaller(hosts, origins map[string]struct{}, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := hosts[r.Host]; !ok {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if _, ok := origins[origin]; !ok {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
