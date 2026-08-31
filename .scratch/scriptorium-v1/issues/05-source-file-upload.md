# 05 — Source File upload

**What to build:** Upload the original ebook to a Book so there is something to translate. `.txt` and `.fb2` are accepted; anything else is refused. The file is stored exactly as supplied and never modified afterwards. Replacing an existing Source File warns first that doing so discards all existing translation work for that Book, so hours of work cannot be destroyed by a stray click.

No parsing happens in this ticket — the file is accepted, validated by format, and stored.

**Blocked by:** 04 — Translation Targets.

**Status:** ready-for-agent

- [x] A `.txt` or `.fb2` upload is stored at `books/<book-code>/source.<ext>`
- [x] The stored bytes are identical to the supplied file
- [x] Any other format is refused with a clear message and nothing is written
- [x] Replacing an existing Source File requires confirming a warning that names the consequence
- [x] Confirming a replacement discards the existing translation artefacts for that Book
- [x] A Book with no Source File is visibly in that state and cannot start work that needs one

## Comments

### Verification — 2026-08-31

`make headless` and `agent-browser` verified the Book details interaction on
the same handler tree as the desktop application. Selecting a fixture Book
shows the empty Source File state, its explanation that Dictionary Building
and translation need an upload, and the `.txt`/`.fb2` file picker with the
Upload Source File action. Screenshot captured during the session; it remains
under the unversioned `.verification/` area. `make check` passes.
