# 04 — Translation Targets

**What to build:** Establish the Workspace Language model, then use it from a Book's details page to create Translation Targets. The application ships a fixed ISO 639-1 base-language catalog. It stores canonical lowercase Language Tags such as `en`, `uk`, and `de`, while displaying each language's English name alongside its tag.

Workspace settings maintain an ordered, initially empty allowlist of Target Languages. Users add languages from the searchable full catalog and remove them individually. A removed language remains valid for existing Translation Targets but cannot create a new one. Settings made in the app apply immediately; external `workspace.toml` edits apply when the Workspace is next opened. There are no custom languages, free-text language fields, live configuration reloads, or migrations: a Workspace containing legacy name-based language values is rejected unchanged.

A Source Language is selected from the full catalog when a Series is created, including when a standalone Book creates its Series. It is stored once as an immutable tag in `series.toml`; Books added to an existing Series inherit it and never submit their own source language.

From a Book's details page, pick an allowed Target Language to create a Translation Target. Several Translation Targets can exist for one Book and are wholly independent — a finished Ukrainian translation is never confused with an unstarted German one, because Status and progress belong to the Translation Target rather than to the Book. A Translation Target can be deleted, abandoning that language without touching the Book or its other translations.

**Blocked by:** 03 — Series and Books on disk.

**Status:** ready-for-agent

- [ ] The application bundles the ISO 639-1 base-language catalog and resolves each canonical tag to its English display name
- [ ] `workspace.toml` starts with an empty, ordered target-language allowlist of canonical tags
- [ ] Workspace settings are reachable from the library and the empty-allowlist prompt; they add catalog languages through a searchable picker and remove them individually
- [ ] Removing an in-use allowed language preserves its existing Translation Targets and warns that it prevents only future creation
- [ ] New Series and standalone-Book creation use the searchable full-catalog Source Language picker; adding a Book to an existing Series submits no source-language value
- [ ] Source Language is stored as an immutable canonical tag in `series.toml`; legacy name-based language values and unknown tags are rejected without migration or rewriting
- [ ] Tom Select is bundled locally, configured with `create: false`, and used with DaisyUI-styled searchable Language controls that search names and tags
- [ ] Target languages offered on Book details come from the Workspace allowlist; the Source Language is excluded and existing targets are disabled with their Status
- [ ] Creating a Translation Target creates `books/<book-code>/translations/<source>-to-<target>/`
- [ ] Creation atomically writes `state.json` with `{ "status": "new" }`; later UI labels map machine status values to canonical Status names
- [ ] A Book can hold several Translation Targets, each with its own Status, starting at `New`
- [ ] The Book details page shows each Translation Target with its human-readable language name, canonical tag, and Status
- [ ] Deleting a Translation Target requires a DaisyUI confirmation modal and removes only that pair's directory; the Book, its Source File, and its other Translation Targets are untouched
- [ ] A duplicate target, an equal source and target language, or any pre-existing pair directory is rejected without modifying files
- [ ] A malformed or unknown `state.json` status is reported as a target-specific read error, is never treated as `New`, and blocks actions on that target
- [ ] A deletion failure leaves the target shown and reports the error; the details view reloads only after successful deletion

## Comments

### Design decisions

**Language Tags are the only persisted language identity.** The fixed ISO 639-1 catalog makes the initial list finite and searchable. Names remain presentation data, so spelling variants and typos cannot create distinct Targets or directories.

**The Source Language belongs to the Series.** A Series Dictionary translates one source vocabulary into each target vocabulary, so all Books in the Series must share the immutable source tag. The Source Language picker is therefore part of Series creation, not Book creation within an existing Series.

**A Language Pair names every target-scoped path.** The directory and future Series Dictionary use `<source>-to-<target>`, for example `en-to-uk`. This is readable and unambiguous even though BCP 47 tags normally contain hyphens; v1's catalog restricts the tags to lowercase ISO 639-1 codes.

**Target creation is durable before later pipeline work begins.** The pair directory is the target's identity and `state.json` records its initial machine Status. Existing or malformed target data is never overwritten or silently repaired.
