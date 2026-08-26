# 15 — Metadata

**What to build:** A Book's details page shows a title, an author, and a language, obtained the cheapest accurate way for its format:

- **fb2** — parsed directly from the file's description block. Exact and instant, with no Agent involvement.
- **txt** — inferred by the Agent from the opening pages, on the cheap Model, so plain-text books still get a useful details page.

Any field can be corrected by hand, so a bad inference or a missing field does not stay wrong.

**Blocked by:** 05 — Source File upload; 06 — fb2 format handler round-trip; 08 — Agent interface, `claude` adapter, and the fake.

**Status:** ready-for-agent

- [ ] Uploading an fb2 fills title, author, and language from its description block with no Agent request made
- [ ] Uploading a `.txt` infers title and author from the opening pages using the cheap Model
- [ ] An fb2 with a missing or malformed description block degrades to empty fields rather than failing the upload
- [ ] A failed inference leaves the fields empty and editable rather than blocking the Book
- [ ] Every metadata field is editable by hand and persists to `book.toml`
- [ ] Hand edits survive any later re-parse or re-inference
