# 13 — The failure ladder

**What to build:** A Chunk whose response the validator rejects does not stall the book, and never silently scrambles it. A fixed ladder runs:

1. **Retry once** with a stricter instruction appended, so a one-off glitch costs nothing but a second request.
2. **Fall back to one request per Text Node**, where misalignment is impossible, so a persistently malformed Chunk still completes.
3. **Mark the Chunk failed, leave the source text in that slot**, and surface the failure count. A gap is never shipped silently — the user gets a readable book with visible gaps rather than one that is quietly wrong.

The user can then see how many Chunks failed and decide whether to retry them or accept the result.

**Blocked by:** 12 — Translation tracer bullet: happy path.

**Status:** ready-for-agent

- [ ] Scripting the fake to drop a node index triggers exactly one retry, with a stricter instruction present in the retry request
- [ ] A second rejection triggers per-Text-Node requests, one per node, asserted against the recorded requests
- [ ] Nodes that succeed under the per-node fallback keep their translations
- [ ] A Chunk failing every rung is marked `failed` in `state.json` with the source text left in place in the output
- [ ] The output file for a run with a failed Chunk is still structurally valid and opens as an ebook
- [ ] The count of failed Chunks is recorded and readable from the service layer
- [ ] Each rung of the ladder has its own test
- [ ] Validator rejections — conversational prefixes, unchanged output, truncated responses — each drive the ladder end to end
