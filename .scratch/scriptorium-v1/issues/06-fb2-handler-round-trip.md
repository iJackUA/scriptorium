# 06 — fb2 format handler round-trip

**What to build:** A format handler that parses an fb2 Source File into an ordered list of Text Nodes plus the structure needed to reassemble it, and accepts translated Text Nodes back to produce an output file. Text Nodes are paragraph-level text content; Chapters come from the file's own section structure.

This is the ticket that proves ADR-0002. The load-bearing test is a round-trip property: parse, splice back with an identity translation, and assert the output is equivalent to the input. ADR-0002 claims structure is preserved *by construction*; this is the test that proves it. If parse-and-splice cannot reproduce an fb2 under an identity translation, ADR-0002 does not hold and several downstream decisions move — which is why this needs no Agent and can start immediately, ahead of its place in the delivery order.

The Agent is never shown markup. This boundary is what makes epub and docx cheap later: a new format means a new handler, not a change to the pipeline.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] Parsing yields an ordered list of Text Nodes and a Chapter structure taken from the fb2's own sections
- [ ] Splicing an identity translation back reproduces a file equivalent to the input
- [ ] The round-trip property runs over a corpus of real fb2 files, including the one in `test_data/`
- [ ] The corpus includes awkward cases: nested markup, footnotes, empty paragraphs, poetry blocks
- [ ] Splicing is by position into the original document tree; the document is never taken apart and rejoined
- [ ] The handler is a pure function tested directly, with no substitution seam
