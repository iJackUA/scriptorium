# Verify the UI through a headless twin, not through the desktop window

The application ships as two entrypoints over one server. The production Wails binary — `main.go` at the repository root, where Wails wants it — opens the desktop window, presents the native folder picker, and drives the real Agent. A second binary, `cmd/headless`, constructs the same embedded server with no window, no picker, and eventually a fake Agent, and prints the URL it bound. Anything that needs to look at the UI — a coding agent verifying its own work, or a human — points a browser at that URL.

The alternative is automating the real window, and it does not work. Wails v2 renders in the platform WebView — WKWebView on macOS — which exposes no Chrome DevTools endpoint, so no browser-automation tool can attach to it. What remains is OS-level accessibility scripting: slow, brittle, platform-specific, and blind to the one thing worth checking, which is what the served HTML actually says. Meanwhile the UI is already `html/template` plus htmx over HTTP. Served over loopback it is an ordinary web page, and an ordinary web page is something every browser tool can drive.

Splitting the binary rather than adding flags to the production one is deliberate. An injectable fake Agent and a bypassed folder picker are each a hole in an HTTP server that runs on the user's machine. Keeping them in a separate `main` means the shipped binary has no code path that reaches them, and the absence is verifiable by reading the root `main.go`'s imports.

## Consequences

The server keeps choosing its own port. A twin that announces the port it bound is as useful to a browser as one that was told which port to bind, and it leaves `ui.NewServer` alone. What the twin does require is that everything *else* the server depends on — the Agent, and the two native calls — arrives as a constructor argument rather than being reached for from inside. That is a standing design obligation on the tickets that introduce them, not work of its own.

Those two native calls become a **desktop shim** interface covering both picking a workspace folder and revealing a folder in the file manager. The production implementation calls the Wails runtime; the headless implementation returns the workspace it was given on the command line and records reveals instead of performing them. This widens the one-function platform shim the spec already called for, and it is the boundary that keeps everything else browser-reachable: the picker is the only surface a browser genuinely cannot touch, so it is the only surface that stays hand-verified.

The fake Agent will be used by two drivers — the test suite and the headless binary — so when there is a translation to watch it gains scripted delays. Without them a whole book translates faster than a frame renders, and the progress dashboard, which is what this application mostly is, cannot be watched at all.

This does not move the automated test suite. Regression coverage stays at the service layer, and the headless twin is a way to *look at* the running application rather than a second suite: what it produces is a report and some screenshots, not assertions in CI.

Nor does it justify much of one. Because the interface is htmx, every interaction is an HTTP request returning an HTML fragment, and `httptest` over the handler tree already answers most questions about it cheaply — as `internal/ui/server_test.go` demonstrates. A browser is needed only for the residue that HTTP cannot see: whether an `hx-target` actually matches an element, whether a stream's updates land in the right region, and whether the result looks right. That residue is small, so the twin should stay small, and grow only when a specific check demands it.
