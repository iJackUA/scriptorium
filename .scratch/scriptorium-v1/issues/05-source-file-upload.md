# 05 — Source File upload

**What to build:** Upload the original ebook to a Book so there is something to translate. `.txt` and `.fb2` are accepted; anything else is refused. The file is stored exactly as supplied and never modified afterwards. Replacing an existing Source File warns first that doing so discards all existing translation work for that Book, so hours of work cannot be destroyed by a stray click.

No parsing happens in this ticket — the file is accepted, validated by format, and stored.

**Blocked by:** 04 — Translation Targets.

**Status:** ready-for-agent

- [ ] A `.txt` or `.fb2` upload is stored at `books/<book-code>/source.<ext>`
- [ ] The stored bytes are identical to the supplied file
- [ ] Any other format is refused with a clear message and nothing is written
- [ ] Replacing an existing Source File requires confirming a warning that names the consequence
- [ ] Confirming a replacement discards the existing translation artefacts for that Book
- [ ] A Book with no Source File is visibly in that state and cannot start work that needs one
