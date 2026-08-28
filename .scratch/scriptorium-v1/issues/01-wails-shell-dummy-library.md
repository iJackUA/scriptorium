# 01 — Wails shell with a dummy library page

**What to build:** Launching the application opens a desktop window showing a library page — a list of Series, each with its Books and each Book's Status. The data is hardcoded fixtures at this stage; the point is a running application with the full UI stack wired end to end, so every later ticket has somewhere to render into.

The Wails CLI is not installed on the development machine, so installing it is part of this ticket. Go 1.24.2 is present.

Per the spec's UI decision: Wails v2 wrapping an embedded Go HTTP server serving `html/template` plus htmx, with Tailwind and daisyUI for components. The server binds to loopback on a random port and checks request origin. Handlers stay thin adapters — a design obligation, since the spec deliberately does not test them.

**Blocked by:** None — can start immediately.

**Status:** done

- [x] `wails dev` and a production build both produce a working application window
- [x] The embedded server binds to loopback on a random port, never a fixed one
- [x] Requests with a foreign `Origin` are rejected
- [x] The library page lists fixture Series, Books, and Statuses using the six Status values from `CONTEXT.md`
- [x] htmx, Tailwind, and daisyUI are wired and demonstrably working (one interactive element is enough)
- [x] Assets are embedded in the binary, not read from disk at runtime

## Comments

Implemented. Wails CLI v2.15.0 installed via `go install`.

**Wails will not navigate the webview to an http URL.** The first attempt served a
redirect page from the Wails asset protocol pointing at the loopback server; the
webview never followed it, and the loopback listener logged no requests. The asset
handler is therefore a reverse proxy onto the loopback listener (`windowProxy` in
`main.go`), so every request the window makes does travel over the real listener.
Verified by instrumenting the listener and running the packaged app: it logs
`GET /`, `GET /static/app.css`, `GET /static/htmx.min.js`. The same check passes
under `wails dev`.

The proxy presents the server's own Origin, because a request arriving through it
came from inside the process. The origin check guards the listener against
everything else on the machine — a page in the user's browser can reach a loopback
port but cannot forge an Origin.

**Open risk for #16:** the reason for choosing an embedded server was server-sent
events, and SSE now passes through the Wails asset protocol, which may buffer. If
it does, the fix is to make the webview navigate to the loopback URL directly
rather than to keep proxying.

**Go version:** Wails v2.15.0 requires Go >= 1.25, so `go.mod` says `go 1.25.0`.
The ticket's note about 1.24.2 is stale — Go 1.26.7 is now installed, and
`.tool-versions` pins the project to it. `~/.tool-versions` still pins
`golang 1.24.2` globally, so anything resolving through the asdf shims outside this
directory gets 1.24 and falls back to Go's automatic toolchain download. Build and
tests pass under `GOTOOLCHAIN=local` on 1.26.7.

Generated assets (`internal/ui/static/app.css`, `htmx.min.js`) are committed so
`go build` and `go test` work without running npm; `npm run build` in `frontend/`
regenerates them.

Reviewed with `/code-review`. Five findings, all fixed: the Origin check now also
validates `Host` (an Origin-less DNS-rebinding request was getting through),
templates render into a buffer so a mid-execution failure is a 500 rather than a
200 with half a page, the `http.Server` has header/idle timeouts and a `Close`,
`/static/` no longer serves a directory index, and the layout no longer pins
`data-theme="light"` over daisyUI's `prefersdark`.

The window was not confirmed visually — this shell lacks assistive access and
`screencapture` only reaches the frontmost Space, so the window never landed in a
screenshot. Worth an eyeball.

**Verification follow-up (agent-verification 02):** `make headless` served the
same library handler tree over loopback. An `agent-browser` snapshot found the
library Book button; clicking it swapped the `book-detail` region to the Book's
details and Translation Targets. Screenshot:
`.verification/library-book-detail.png` (ignored by git).
