# 01 — Wails shell with a dummy library page

**What to build:** Launching the application opens a desktop window showing a library page — a list of Series, each with its Books and each Book's Status. The data is hardcoded fixtures at this stage; the point is a running application with the full UI stack wired end to end, so every later ticket has somewhere to render into.

The Wails CLI is not installed on the development machine, so installing it is part of this ticket. Go 1.24.2 is present.

Per the spec's UI decision: Wails v2 wrapping an embedded Go HTTP server serving `html/template` plus htmx, with Tailwind and daisyUI for components. The server binds to loopback on a random port and checks request origin. Handlers stay thin adapters — a design obligation, since the spec deliberately does not test them.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `wails dev` and a production build both produce a working application window
- [ ] The embedded server binds to loopback on a random port, never a fixed one
- [ ] Requests with a foreign `Origin` are rejected
- [ ] The library page lists fixture Series, Books, and Statuses using the six Status values from `CONTEXT.md`
- [ ] htmx, Tailwind, and daisyUI are wired and demonstrably working (one interactive element is enough)
- [ ] Assets are embedded in the binary, not read from disk at runtime
