# 15 — Metadata

**What to build:** A Book's details page shows a title, an author, and a language, obtained the cheapest accurate way for its format:

- **fb2** — parsed directly from the file's description block. Exact and instant, with no Agent involvement.
- **txt** — inferred by the Agent from the opening pages, on the cheap Model, so plain-text books still get a useful details page.

Any field can be corrected by hand, so a bad inference or a missing field does not stay wrong.

**Blocked by:** 05 — Source File upload; 06 — fb2 format handler round-trip; 08 — Agent interface, `claude` adapter, and the fake.

**Status:** ready-for-agent

- [x] Uploading an fb2 fills title, author, and language from its description block with no Agent request made
- [x] Uploading a `.txt` infers title and author from the opening pages using the cheap Model
- [x] An fb2 with a missing or malformed description block degrades to empty fields rather than failing the upload
- [x] A failed inference leaves the fields empty and editable rather than blocking the Book
- [x] Every metadata field is editable by hand and persists to `book.toml`
- [x] Hand edits survive any later re-parse or re-inference

## Comments

### Implementation — 2026-09-03

FB2 uploads now parse title, first listed author, and canonical Source File language from
`description/title-info` without an Agent call. TXT uploads ask the configured
mechanical Model at low effort for JSON title/author metadata from the first
12,000 bytes. Failed inference and malformed/missing FB2 descriptions leave
the Book usable with blank, editable fields.

The details page has an editable metadata form. Its submitted values persist in
`book.toml`; per-field edit markers also preserve intentional blank edits and
prevent later source parsing or inference from replacing hand corrections.

Focused UI and library tests, plus `make check`, pass. Browser verification was
attempted through `make headless`; the in-app browser backend was unavailable
in this execution environment, so no interactive screenshot could be captured.
