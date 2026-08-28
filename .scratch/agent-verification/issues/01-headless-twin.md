# 01 — The headless twin and its Makefile target

**What to build:** `cmd/headless`, a second `main` that constructs the same embedded server the window uses, serves it until interrupted, and prints the URL it bound as its first line of output. Plus `make headless`, which is the only documented way to launch it.

Per ADR-0004 this is a separate binary rather than flags on the production one: an injectable fake Agent and a bypassed folder picker are each a hole in an HTTP server running on the user's machine, and keeping them in their own `main` means the shipped binary has no code path that reaches them.

It takes no address. `ui.NewServer` already binds `127.0.0.1:0`, and a twin that announces its port serves a browser just as well as one that was told which port to use — so the server is left alone. It takes no other flags either, until there is a workspace to serve or a translation to watch.

Today that means `cmd/headless` is a handful of lines over `ui.NewServer(library.Fixture())`. That is the point.

**Blocked by:** None.

**Status:** ready-for-agent

- [x] `cmd/headless` serves the same handler tree the window serves, with the Host and Origin checks active and unchanged
- [x] Its first line of output is the URL, in a form a script can read without parsing prose
- [x] It serves until interrupted, and exits cleanly on SIGINT
- [x] `make headless` launches it, is listed by `make help`, and is the only place launch instructions live
- [x] The root `main.go` imports nothing from `cmd/headless`, gains no flags, and is otherwise untouched
- [x] `make check` passes and the Wails production build is unchanged in behaviour

## Comments

Implemented with an isolated `cmd/headless` entrypoint. It creates an ephemeral
fixture Workspace for every session, and its picker always declines so a browser
can never open the native folder chooser. The first line from `make headless`
was the bare loopback URL. Verified the page and its Host/Origin middleware
through the unchanged `ui.NewServer` handler tree.
