# 13 — The failure ladder

**What to build:** A Chunk whose response the validator rejects does not stall the book, and never silently scrambles it. A fixed ladder runs:

1. **Retry once** with a stricter instruction appended, so a one-off glitch costs nothing but a second request.
2. **Fall back to one request per Text Node**, where misalignment is impossible, so a persistently malformed Chunk still completes.
3. **Mark the Chunk failed**, preserve the rejected response for diagnostics, and surface the failure count. A failed Chunk is never included in automatic final Book Composition. If the user explicitly chooses **Compose translated book**, the source text is used in that slot as a visible fallback rather than allowing a wrong translation to shift later nodes.

The user can then see how many Chunks failed and choose **Validate and Repair** or use **Compose translated book** as a diagnostic escape hatch.

**Blocked by:** 12 — Translation tracer bullet: happy path.

**Status:** done

- [x] Scripting the fake to drop a node index triggers exactly one retry, with a stricter instruction present in the retry request
- [x] A second rejection triggers per-Text-Node requests, one per node, asserted against the recorded requests
- [x] Nodes that succeed under the per-node fallback keep their translations
- [x] A Chunk failing every rung is marked `failed` in `state.json` and its rejected response is preserved
- [x] Automatic final Book Composition is blocked by a failed Chunk
- [x] Explicit **Compose translated book** produces a structurally valid partial output with source-text fallback for a failed Chunk
- [x] The count of failed Chunks is recorded and readable from the service layer
- [x] Each rung of the ladder has its own test
- [x] Validator rejections — conversational prefixes, unchanged output, truncated responses — each drive the ladder end to end

## Comments

2026-09-03: Implemented the fixed failure ladder in the translation service. A
rejected Chunk is retried once with a strict protocol instruction, then split
into one request per Text Node. Rejected responses are retained under
`chunks/rejected/`; failed Chunks and the aggregate failure count are durable in
`state.json`, and later Chunks continue without carrying untrusted continuity.
Automatic composition is refused when any Chunk failed. The explicit
`ComposeTranslatedBook` service action writes a clearly named partial output
using source Text Nodes for unavailable translations without invoking the Agent
or changing state. Service tests cover all validator rejection types and each
ladder rung; `make check` passes.
