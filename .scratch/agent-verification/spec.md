# Agent verification harness

Status: ready-for-agent

## Problem Statement

Scriptorium is built largely by coding agents, and a large part of it is user interface: a library list, a Dictionary review page, and a progress dashboard over long-running jobs. An agent can run the Go test suite and know that chunking, continuity, and splice fidelity are correct. It cannot currently tell whether the page it just wrote renders, whether clicking a Book does anything, or whether progress moves. It closes UI tickets blind, and the first person to find out is me, by hand, later. `scriptorium-v1` ticket 01 closed with exactly this admission: *"The window was not confirmed visually… Worth an eyeball."*

The obvious approach — automate the real desktop window — does not work. Wails v2 renders in WKWebView, which exposes no DevTools endpoint, so no browser-automation tool can attach to it (see ADR-0004).

## Solution

The interface is served over loopback HTTP, so it is an ordinary web page and any browser tool can drive it. A second entrypoint, `cmd/headless`, constructs the same embedded server with no window and no picker, and prints the URL it bound. A verifying agent launches it, drives the page with `agent-browser`, and drops a screenshot for me to look at.

That is the whole harness. It is deliberately small, for a reason worth stating plainly.

### Most UI questions are not browser questions

Because the interface is htmx, every interaction is an HTTP request returning an HTML fragment. `internal/ui/server.go` already exposes `Handler()`, and `internal/ui/server_test.go` already drives it through `httptest`. So "does the page render", "does clicking a Book return the right fragment", "does a bad Origin get rejected" are answerable with **no new infrastructure at all** — a Go test, or `curl` against a running twin.

What HTTP cannot see is a short list, and it is the list that matters:

- **Wiring.** `library.html` carries `hx-get`, `hx-target="#book-detail"`, `hx-swap`. If that target selector matches nothing, the fragment is perfect and the screen does nothing. This is htmx's signature bug and it is invisible to every HTTP-level check.
- **Streams landing.** Server-sent events update a region of an existing page. Whether the update arrives in the right region is a DOM question.
- **Appearance.** Whether the daisyUI layout looks right, which is not an agent's judgement to make anyway.

So: **HTTP-level checks are the default and the browser is for the residue.** A verification session reaches for `agent-browser` when the question is about the DOM or the pixels, and for `httptest` or `curl` otherwise.

This is also why the harness is not a test suite. Regression coverage stays at the service layer where `scriptorium-v1`'s testing decisions put it. Nothing here runs in CI, and nothing here is committed as an executable check.

## Design

### The headless twin

`cmd/headless` builds the same server the window does, and serves it until interrupted. It takes no flags it does not need yet — a `-workspace` path once there is a workspace to serve, and a fake Agent script once there is a translation to watch.

It does **not** take an address. `ui.NewServer` already binds `127.0.0.1:0` and the root `main.go` already logs `serving on <url>`; a twin that announces its port is as useful to a browser as one that was told which port to use, and it leaves the server untouched. A verifying agent launches the twin in the background and reads the URL from its first line of output.

The Host and Origin checks stay on, unchanged. A browser navigating to the printed URL satisfies both — which is exactly why this works at all.

The production binary must contain no code path that reaches a fake Agent or bypasses the picker, checkable by reading the root `main.go`'s imports.

### Standing design obligation

The twin only stays buildable if the server's dependencies arrive from outside it. So, for the tickets that introduce them:

- The **Agent** is a constructor argument, so the twin can pass the fake.
- The **desktop shim** — one interface covering picking a workspace folder and revealing a folder in the file manager — is a constructor argument, so the twin can pass an implementation that answers from the command line and records reveals instead of performing them. This is the widening of the one-function platform shim `scriptorium-v1` already calls for.

Neither is a ticket here. They are conditions on `scriptorium-v1` 02 and 08, where that code gets written.

### Driven through the Makefile

The `Makefile` is this repo's single source of truth for repeatable commands, so the twin is launched by `make headless` and nowhere else. No launch instructions are documented anywhere but that target.

### Legibility contract

`agent-browser` works from accessibility snapshots, and daisyUI markup is div soup unless written deliberately. So: **semantics first** — real headings, `<table>` for tabular data, labelled form controls, buttons that are `<button>`, `aria-live` on regions that server-sent events update. One effort serves both a screen reader and a verifying agent, with no separate contract to keep in sync. `data-testid` is the documented escape hatch where semantics genuinely cannot identify a thing, expected chiefly on Status badges and progress regions.

### Recipes, not scripts

Each UI ticket carries a short prose **verification recipe**: what to launch, and what should be true on screen. The agent improvises the session. Committed browser scripts would be a regression suite by another name, and that decision is already made elsewhere.

### Screenshots

A session drops screenshots to `.verification/` — gitignored — and links them in its report. The agent verifies behaviour; I verify appearance.

### When verification is required

A ticket that renders anything is not done until a verification session has run and its findings are recorded in the ticket's `## Comments`. Tickets that touch no UI are unaffected.

## Deferred

Each of these is a real need with no rework cost, because `agent-browser` only ever points at a URL. None is worth building before the feature it would verify exists.

- **Fixture workspaces.** One committed workspace per interesting Status, so a session can reach any page without driving the whole flow. Wait for the first check that needs a state the existing fixture library cannot express; add one workspace at a time. When they arrive: all workspaces share one hand-authored source file per format, no real book or excerpt is ever committed, and the tree stays under 100KB — they are committed data, and a repository that accumulates ebooks is a repository nobody clones.
- **Scripted delays on the fake Agent.** Per-response delays and failure modes, so progress is watchable and each rung of the failure ladder presents itself on screen. Wait for `scriptorium-v1` 12, the translation tracer bullet.
- **A `curl` probe for SSE stream shape.** Worth separating from the browser once there is a stream, so "the stream was wrong" and "htmx rendered it wrong" stay attributable.

## Out of Scope

- **Automating the real Wails window.** No DevTools endpoint; accessibility scripting is slow, brittle, and blind to the served HTML. See ADR-0004. Screenshotting the real window via `screencapture -l<windowid>` would need a one-time Screen Recording grant and buys only the window chrome, since the page itself is identical over HTTP — worth doing by hand once, not worth automating.
- **Verification in CI.** This produces a report and screenshots, not assertions.
- **Playwright or a DevTools MCP server.** No capability `agent-browser` lacks here, at more cost per call.
- **Cross-platform verification.** macOS only, matching the application's own v1 scope.

## Further Notes

`agent-browser` and Google Chrome are installed on the development machine; `curl` is present. The Wails CLI is installed by `make tools`.

`scriptorium-v1` ticket 01 has shipped, so this modifies working code. Two facts from it matter. The Wails window reaches the server through a reverse proxy in the root `main.go` that rewrites `Origin`, because the v2 webview will not navigate to an `http` URL — the twin needs no proxy, since a browser navigates there directly. And `internal/ui/server_test.go` already tests the handler tree, which contradicts the `scriptorium-v1` spec's claim that the handlers are deliberately untested; reality picked the cheaper option, and this spec assumes reality.

Go 1.26.7 is pinned by `.tool-versions`, while the global asdf pin is 1.24.2, so commands run from outside this directory may resolve the wrong toolchain.
