# 04 — Translation Targets

**What to build:** From a Book's details page, pick a target language from the workspace list to create a Translation Target. Several Translation Targets can exist for one Book and are wholly independent — a finished Ukrainian translation is never confused with an unstarted German one, because Status and progress belong to the Translation Target rather than to the Book. A Translation Target can be deleted, abandoning that language without touching the Book or its other translations.

The source language is not asked for here: it is set once on the Series.

**Blocked by:** 03 — Series and Books on disk.

**Status:** ready-for-agent

- [ ] Target languages offered come from `workspace.toml`
- [ ] Creating a Translation Target creates `books/<book-code>/translations/<pair>/`
- [ ] A Book can hold several Translation Targets, each with its own Status, starting at `New`
- [ ] The Book details page shows each Translation Target with its own Status
- [ ] Deleting a Translation Target removes only that pair's directory; the Book, its Source File, and its other Translation Targets are untouched
- [ ] Creating a Translation Target for a language that already has one is rejected
