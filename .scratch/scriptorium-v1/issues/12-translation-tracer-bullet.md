# 12 — Translation tracer bullet: happy path

**What to build:** Start the translation on a Translation Target whose Dictionary is ready, and get back a finished file in the same format and structure as the original, with the original untouched. This is the first end-to-end path through the whole pipeline.

Chunks are translated **serially** (ADR-0003), each request carrying:

- the merged Dictionary, injected unfiltered, so the Terms the user approved are respected throughout the book rather than only where they were first found;
- a **Continuity Window** — the tail of the previous Chunk's source text *and its accepted translation*, explicitly marked as reference material not to be translated, so register, tense, and forms of address stay consistent across boundaries. Continuity resets at Chapter boundaries;
- the numbered-node instructions from the validator's protocol.

The prompt is a template with documented slots for source and target language, the Dictionary, the Continuity Window, and the numbered-node instructions. The built-in default ships with the application; per-Series overrides come later.

Before the first Agent request, parsing and chunking persist a Book-level Chunk Materialization under `chunks/original/`, with stable global Text Node indices and a manifest. Each accepted response is persisted under the Translation Target's `chunks/translated/` directory before `state.json` marks that Chunk completed. `state.json` records the Translation Target status, active Source File and Dictionary fingerprints, and per Chunk index, status, cost, and attempts; it never stores prose. Every artifact and state update is written to a temporary file and renamed, so an interrupted write cannot silently lose accepted work.

Tests drive the service layer over a temporary workspace and observe the output file, persisted original and translated Chunk artifacts, `state.json`, and the recorded Agent requests.

**Blocked by:** 05 — Source File upload; 08 — Agent interface, `claude` adapter, and the fake; 09 — Chunker and numbered-node validator; 11 — Dictionary review, override, and promotion.

**Status:** ready-for-agent

- [ ] A real fb2 in, a translated fb2 out, with structure asserted identical — ADR-0002's central claim
- [ ] The output lands at `translations/<pair>/out/<book-code>.<target>.<ext>`, so translations of one Book never collide
- [ ] The Source File is byte-for-byte unchanged after a full run
- [ ] Original and accepted translated Chunks are persisted with stable global Text Node indices before final Book Composition
- [ ] The final output is composed from persisted Chunk Translations rather than in-memory Agent responses
- [ ] Recorded requests are strictly ordered and never overlap, as ADR-0003 requires
- [ ] Request *N* contains the translation returned for request *N-1*, and the Continuity Window is marked as not to be translated
- [ ] The Continuity Window is empty at the start of each Chapter
- [ ] Every translation request contains the merged Dictionary, with Book Dictionary entries overriding Series ones
- [ ] Recorded requests respect the word budget and never cross a Chapter
- [ ] `state.json` records index, status, source hash, and cost per Chunk, written temp-file-and-rename
- [ ] Status moves to `Translating` and then `Translated`
- [ ] A `.txt` with no detectable Chapters translates end to end
- [ ] The prompt template's required slots are documented in one place
