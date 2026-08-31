# 07 — txt handler and Chapter detection

**What to build:** A format handler for plain `.txt`: Text Nodes are blank-line-separated paragraphs, and Chapters are detected with a built-in set of regex heuristics that can be overridden per Book.

Detection failure is explicitly not an error. With no Chapters found the whole file is one Chapter and the pipeline runs unchanged, losing only continuity hygiene. A `.txt` with no detectable chapter headings must still translate end to end.

**Blocked by:** 06 — fb2 format handler round-trip (establishes the handler boundary and the round-trip property).

**Status:** ready-for-agent

- [x] Parsing splits on blank lines into paragraph Text Nodes
- [x] Built-in regex heuristics detect common chapter headings
- [x] The heuristics can be overridden per Book
- [x] A file with no detectable Chapters parses as a single Chapter, with no error and no warning that blocks work
- [x] Splicing an identity translation back reproduces an equivalent file, over a corpus including `test_data/`
- [x] The handler satisfies the same interface as the fb2 handler, with no pipeline-side branching on format

## Comments

### Implementation — 2026-08-31

Added the pure `internal/format/txt` handler. It extracts blank-line-separated
Text Nodes, detects common numbered and named Chapter headings, and accepts
per-Book regex replacements through `txt.Options`. When no heading matches, it
keeps a single Chapter so the translation pipeline can proceed unchanged.

The handler retains exact source ranges and line endings for lossless identity
splicing, validates node count and indices like the FB2 handler, and shares the
Text Node and Chapter model with FB2. Tests cover LF/CRLF input, custom
patterns, fallback detection, Text Node replacement, numbered-prose filtering,
misalignment, and the
committed Sherlock Holmes TXT corpus. `make check` passes.
