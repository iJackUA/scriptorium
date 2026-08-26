# 03 — Series and Books on disk

**What to build:** Create a Series with a name and a source language, and add Books to it. Each Book is given a Book Code by hand, which names the folder it is stored under. A Book can also be added standalone without thinking about Series first — behind the scenes it still lives in a real Series of one, so a sequel can be added later without migrating anything. A Series holding exactly one Book renders without a group header; that is a rendering choice only, not a second concept in the model.

The library list now reads the workspace instead of fixtures, and opening a Book shows its details page.

**Blocked by:** 02 — Workspace folder picker.

**Status:** ready-for-agent

- [ ] Creating a Series writes `<series-code>/series.toml` with name, source language, and defaults
- [ ] Adding a Book writes `<series-code>/books/<book-code>/book.toml`
- [ ] A Book Code already used in that Series is rejected with a clear message
- [ ] A Book Code containing characters that are not allowed in a folder name is rejected with a clear message
- [ ] Both rejections happen before anything is written to disk
- [ ] Adding a standalone Book creates a Series of one without asking the user about Series
- [ ] A Series of one renders flat, with no group header; a Series of two or more renders grouped
- [ ] The library list and Book details page read from the workspace files; no fixtures remain
- [ ] Service-layer tests drive a temporary workspace directory and assert on the files written
