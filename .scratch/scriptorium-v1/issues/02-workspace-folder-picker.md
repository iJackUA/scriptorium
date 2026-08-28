# 02 — Workspace folder picker

**What to build:** On first launch the application asks where the workspace folder should live and lets the user pick any directory on any drive. The choice is remembered, so a relaunch opens straight into that workspace. The workspace root gets a `workspace.toml` holding the default Agent and Model and the list of target languages.

The whole library is plain files under this one root, so it can be backed up, synced, inspected, or hand-edited without the application.

**Blocked by:** 01 — Wails shell with a dummy library page.

**Status:** done

- [x] First launch presents a native folder picker and accepts any writable directory
- [x] The chosen path persists across restarts; a second launch goes straight to the library
- [x] `workspace.toml` is created at the root with default Agent, default Models, and the target language list
- [x] An existing workspace is opened rather than overwritten
- [x] A workspace whose folder has since disappeared is reported clearly and the picker is offered again
- [x] `workspace.toml` is TOML so hand-written comments survive a rewrite

## Comments

### Implementation notes

**The picker is an interface, and the native one lives in `main.go`.** ADR-0004
wants the production binary to be the only thing that can open a dialog on the
user's desktop, so `internal/desktop.FolderPicker` is a one-method interface and
its Wails implementation sits in the root `main.go` — nothing under `internal/`
imports Wails. This also discharges the standing design obligation
`agent-verification`'s spec places on this ticket: the picker arrives as a
constructor argument, so the headless twin can pass its own. Revealing a folder
in the file manager belongs on the same interface and joins it with the ticket
that first has a folder worth revealing; adding it now would be a method every
implementation writes dead code for.

**The user is asked, but not ambushed.** The ticket says the application asks
on first launch; it asks with a screen carrying the reason and a button, rather
than throwing a native dialog up before the window has drawn. Cancelling that
dialog then leaves the user somewhere rather than nowhere, and the same screen
is what reports a folder that could not be opened.

**Nothing rewrites `workspace.toml`.** The format was chosen so comments
survive, and comments only survive if the file is never re-encoded — so `Open`
writes a default file (comments and all) when the folder has none, and
otherwise only reads. There is deliberately no `Save` to reach for; a test
asserts the file is byte-for-byte unchanged after opening.

**Two files, two homes.** `workspace.toml` holds what a workspace is;
`~/Library/Application Support/Scriptorium/settings.toml` holds which workspace
to open, because that one cannot live inside a workspace. A settings file that
is absent, unparseable or blank all mean "ask the user" and none of them fails a
launch.

**Writability is proven by writing.** The ticket asks the picker to accept any
writable directory; rather than probing permissions, `Open` creates
`workspace.toml`, and a folder that will not take it reports the real error on
screen with the picker still offered. A folder is only remembered *after* it
opens, so an unusable choice cannot become the folder every future launch fails
on.

**The library is still `library.Fixture()`.** Reading Series and Books from the
workspace root is ticket 03; this ticket ends at having a root.

### Verification

`make check` and `make build` both pass. The screens were driven through
`httptest` (`internal/ui/workspace_test.go`): first launch offers the picker,
choosing swaps in the library and remembers the folder, a relaunch skips
straight to it, a vanished folder is named on screen, and a cross-origin `POST
/workspace` is rejected without the picker ever being shown.

Two gaps left open deliberately, both belonging to later tickets. A workspace
that already has a `workspace.toml` but is not writable opens cleanly and would
fail on the first write — worth catching where the first write lives, in 03. And
the missing-folder check runs at launch; a drive unmounted while the application
is running keeps serving the library, which is only detectable once the library
is read from disk, also 03.

Not confirmed visually — the window was never eyeballed, and the `hx-target`
that swaps the chosen library into `#app` is exactly the wiring HTTP-level
checks cannot see. That is what `agent-verification` 01 and 02 exist to fix;
this ticket's screens should be run through a session once they land.
