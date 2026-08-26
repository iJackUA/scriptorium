# 12 — Translation tracer bullet: happy path

**What to build:** Start the translation on a Translation Target whose Dictionary is ready, and get back a finished file in the same format and structure as the original, with the original untouched. This is the first end-to-end path through the whole pipeline.

Chunks are translated **serially** (ADR-0003), each request carrying:

- the merged Dictionary, injected unfiltered, so the Terms the user approved are respected throughout the book rather than only where they were first found;
- a **Continuity Window** — the tail of the previous Chunk's source text *and its accepted translation*, explicitly marked as reference material not to be translated, so register, tense, and forms of address stay consistent across boundaries. Continuity resets at Chapter boundaries;
- the numbered-node instructions from the validator's protocol.

The prompt is a template with documented slots for source and target language, the Dictionary, the Continuity Window, and the numbered-node instructions. The built-in default ships with the application; per-Series overrides come later.

`state.json` records per Chunk: index, status, a hash of the source text, and cost. It is the only file that must be crash-safe — write to a temporary file and rename, so an interrupted write can never leave a book unresumable.

Tests drive the service layer over a temporary workspace and observe exactly three things: the output file, `state.json`, and the recorded Agent requests.

**Blocked by:** 05 — Source File upload; 08 — Agent interface, `claude` adapter, and the fake; 09 — Chunker and numbered-node validator; 11 — Dictionary review, override, and promotion.

**Status:** ready-for-agent

- [ ] A real fb2 in, a translated fb2 out, with structure asserted identical — ADR-0002's central claim
- [ ] The output lands at `translations/<pair>/out/<book-code>.<target>.<ext>`, so translations of one Book never collide
- [ ] The Source File is byte-for-byte unchanged after a full run
- [ ] Recorded requests are strictly ordered and never overlap, as ADR-0003 requires
- [ ] Request *N* contains the translation returned for request *N-1*, and the Continuity Window is marked as not to be translated
- [ ] The Continuity Window is empty at the start of each Chapter
- [ ] Every translation request contains the merged Dictionary, with Book Dictionary entries overriding Series ones
- [ ] Recorded requests respect the word budget and never cross a Chapter
- [ ] `state.json` records index, status, source hash, and cost per Chunk, written temp-file-and-rename
- [ ] Status moves to `Translating` and then `Translated`
- [ ] A `.txt` with no detectable Chapters translates end to end
- [ ] The prompt template's required slots are documented in one place
