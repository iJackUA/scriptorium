# 09 — Chunker and numbered-node validator

**What to build:** Two pure components at the heart of the pipeline.

The **chunker** groups consecutive Text Nodes into Chunks against a word budget of roughly 2,000 words, never splitting a Text Node and never crossing a Chapter boundary. The budget follows from the ~3.5s fixed cost per Agent invocation: per-paragraph requests would mean hours of process startup for a novel, while whole-chapter requests produce outputs long enough for models to drift and summarise.

The **validator** enforces the numbered-node protocol, the load-bearing safety mechanism of the whole design. Because translations are spliced back by position, a Chunk returning fewer Text Nodes than it was sent shifts every later translation into the wrong slot — producing a structurally valid file that is quietly scrambled from that point on, which a reader would not notice until reading it. Each Text Node is therefore sent with an explicit index marker, and a response is rejected unless the markers come back with the same count and the same indices, none missing and none invented.

**Blocked by:** 07 — txt handler and Chapter detection (both handlers' Text Node and Chapter output must exist).

**Status:** done

- [x] Chunks stay within the word budget except where a single Text Node exceeds it alone
- [x] A Text Node is never split across Chunks
- [x] A Chunk never crosses a Chapter boundary
- [x] Text Nodes are serialised with explicit index markers and parsed back by marker, not by order of appearance
- [x] The validator rejects a response whose node count differs from the request
- [x] The validator rejects missing indices and invented indices
- [x] The validator strips known conversational prefixes before validating, then rejects if the remainder still fails
- [x] The validator rejects output identical to the input, which indicates a refusal or a pass-through
- [x] The validator rejects truncated trailing nodes
- [x] Both components are tested directly as pure functions, with no seam
