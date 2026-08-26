# 07 — txt handler and Chapter detection

**What to build:** A format handler for plain `.txt`: Text Nodes are blank-line-separated paragraphs, and Chapters are detected with a built-in set of regex heuristics that can be overridden per Book.

Detection failure is explicitly not an error. With no Chapters found the whole file is one Chapter and the pipeline runs unchanged, losing only continuity hygiene. A `.txt` with no detectable chapter headings must still translate end to end.

**Blocked by:** 06 — fb2 format handler round-trip (establishes the handler boundary and the round-trip property).

**Status:** ready-for-agent

- [ ] Parsing splits on blank lines into paragraph Text Nodes
- [ ] Built-in regex heuristics detect common chapter headings
- [ ] The heuristics can be overridden per Book
- [ ] A file with no detectable Chapters parses as a single Chapter, with no error and no warning that blocks work
- [ ] Splicing an identity translation back reproduces an equivalent file, over a corpus including `test_data/`
- [ ] The handler satisfies the same interface as the fb2 handler, with no pipeline-side branching on format
