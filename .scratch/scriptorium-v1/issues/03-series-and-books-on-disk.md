# 03 — Series and Books on disk

**What to build:** Create a Series with a name and a source language, and add Books to it. Each Book is given a Book Code by hand, which names the folder it is stored under. A Book can also be added standalone without thinking about Series first — behind the scenes it still lives in a real Series of one, so a sequel can be added later without migrating anything. A Series holding exactly one Book renders without a group header; that is a rendering choice only, not a second concept in the model.

The library list now reads the workspace instead of fixtures, and opening a Book shows its details page.

**Blocked by:** 02 — Workspace folder picker.

**Status:** done

- [x] Creating a Series writes `<series-code>/series.toml` with name, source language, and defaults
- [x] Adding a Book writes `<series-code>/books/<book-code>/book.toml`
- [x] A Book Code already used in that Series is rejected with a clear message
- [x] A Book Code containing characters that are not allowed in a folder name is rejected with a clear message
- [x] Both rejections happen before anything is written to disk
- [x] Adding a standalone Book creates a Series of one without asking the user about Series
- [x] A Series of one renders flat, with no group header; a Series of two or more renders grouped
- [x] The library list and Book details page read from the workspace files; no fixtures remain
- [x] Service-layer tests drive a temporary workspace directory and assert on the files written

## Comments

### Implementation notes

**The service layer is `library.Store`, and it holds nothing.** Every read goes
to disk. The library is plain files the user may back up, sync or hand-edit
while the application is open, so a cache would be a second answer to a
question the filesystem already answers — and reading at every render is what
turns "the drive was unmounted mid-session" into a message on screen. That
closes the second of the two gaps ticket 02 left open for this one.

**The Series code is derived; only the Book Code is typed.** `CONTEXT.md` makes
the Book Code the thing "assigned by hand", and asking for a Series code as
well would mean naming two folders to add one book — the ceremony a standalone
Book exists to avoid. So a Series folder is slugified from the Series name (from
the Book Code, for a Series of one), and steps aside to `-2`, `-3` when the name
it derives is taken. Every entry in the workspace root counts as taken, not
only the Series, so a name that slugs to `workspace.toml` or to a folder of the
user's own notes cannot write into it.

**Only the Book Code is asked for.** The ticket says "each Book is given a Book
Code by hand" and nothing else, and it is the only field the user is in a
position to fill: a Book is added before its Source File exists, and title and
author come out of that file (06, 07) and are corrected by hand later (15). So
both are optional, offered to a user who happens to know them, and `Book.Label`
falls back to the Book Code for a Book that has no title yet. Requiring a title
here would have manufactured data guaranteed to be replaced.

**The Book Code rule is an allowlist.** Letters, digits, hyphens and
underscores, starting with a letter or digit, at most 64 characters. A denylist
would have to enumerate what means something to a filesystem, a shell and a URL
on three platforms; the Book Code has to name the same folder on all of them.
Two codes differing only in case are rejected as taken, because they are two
folders on Linux and one on macOS and Windows, and a library has to mean the
same thing on both.

**Rejection happens before the first byte.** `AddBook` finds the Series, checks
the draft and scans the existing Books, all before `writeBook` is called, so a
refused Book leaves the Series exactly as it was — a test asserts the `books/`
directory is not even created.

**A standalone Book is a Series of one on disk, identical to any other.** Its
`series.toml` takes the Book's title as the Series name, which is never shown
while the Series holds one Book. Adding the sequel two years later is an
ordinary `AddBook`; a test walks that path and watches the group header appear.

**`series.toml` and `book.toml` are written from templates, never re-encoded.**
Same reason as `workspace.toml`: TOML was chosen so hand-written comments
survive, and they only survive if nothing rewrites the file. Both templates
document the Agent and Model defaults they may carry, commented out, rather than
modelling fields nothing reads yet — 17 owns those, and decoding them now would
be a promise this ticket does not keep. Titles are quoted through `tomlString`,
so an apostrophe or a backslash in a book title cannot break the file.

**Superseded language-input decision.** Issue 04 replaces the plain source
language input with a constrained full-catalog picker. The Series stores one
immutable canonical Source Language tag; Books added to it inherit that tag and
never submit their own language. The Workspace target-language allowlist remains
distinct from the full source-language catalog.

**Forms hand back what the user typed.** Both panels post the whole library
screen back to `#app`, so a rejection arrives with the fields still filled in
and the panel it came from still open. There is no client-side state to lose,
which is what makes the round trip affordable — and it keeps the handlers thin
adapters, which the spec asks for because nothing tests them.

**The Series of one keeps the Book Code as typed.** Story 6 gives the user
control of the folder name, so `Solaris` stays `Solaris/books/Solaris/`. Clashes
are still compared without case, because macOS and Windows would fold them into
one folder.

**No Statuses on the library page yet.** Status belongs to a Translation Target
and no Translation Target can exist until 04, so the badges render nothing. The
`status-class` template is left in place for 04 to fill.

### Findings left standing

Three review findings were considered and deliberately not acted on.

**`series.toml` carries its defaults as comments, not values.** The checkbox
asks for "name, source language, and defaults", and the file documents the
`agent` and `[models]` it may carry rather than writing them out. Writing them
as values would duplicate `workspace.toml` into every Series, so a workspace-wide
change to the default Model would stop reaching the Series that inherited it —
which is the opposite of a default. Decoding fields nothing reads would be a
promise this ticket does not keep; 17 owns them.

**`Store` and `BookDraft` are not in `CONTEXT.md`.** Neither is a domain concept
— one is the reader/writer over the files, the other a parameter object — but
`Store` sits close to the "storage" the Workspace entry tells us to avoid. Noted
for `/domain-modeling` rather than renamed on a guess.

**`TranslationTarget`, `Status` and the `status-class` template render nothing.**
They are the model `CONTEXT.md` defines, left in place from 01, and 04 is what
fills them. Deleting them to re-add them one ticket later is churn.

### Verification

`make check`, `make build` and `make assets` all pass. Twenty-three service-layer
tests drive a temporary workspace and assert on the files written — including that a
rejection writes nothing, that the same Book Code in two Series is not a clash,
and that a broken hand-edited `series.toml` is reported with the file named
rather than dropping a Series the user can see on disk.

The screens are driven through `httptest` (`internal/ui/authoring_test.go`):
creating a Series, adding a Book to it, adding a Book on its own and watching it
render flat, then adding the sequel and watching the group header appear; both
rejections reported on screen with the form still filled in; a workspace removed
mid-session reported with the picker offered again; and both new routes rejected
from a foreign origin without writing anything.

Both review axes were run. The service layer gained tests for the two findings
worth fixing: a Book added with nothing but its code, and a Series of one whose
Book Code keeps its capitals. The screens gained the one ticket 02 left for this
ticket — a workspace that opens read-only and only finds out on the first write
now reports it on screen, from both panels.

The daisyUI form classes needed rewriting for v5 (`form-control`, `label-text`,
`input-bordered` are gone) — caught by grepping the rebuilt bundle for each
class used, since nothing else would have.

Not confirmed visually: the rendered HTML was dumped and read, but the window
was never eyeballed, so the `<details>` panels and the swap into `#app` are
unverified as they look to a user. `agent-verification` 01 is what fixes that.
