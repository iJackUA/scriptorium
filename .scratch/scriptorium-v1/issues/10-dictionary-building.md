# 10 — Dictionary Building

**What to build:** Start Dictionary Building for a Translation Target and, when it finishes, have a proposed Dictionary ready for review — terminology decided before any prose is translated. Progress through the Book is visible while it runs, so the user knows it is working and roughly how long is left.

Two passes, split by concern:

1. **Extraction** — per Chunk, on the cheap Model, candidate Terms only and **no translations**. Results are merged across Chunks and counted: a Term appearing throughout the book is a major character, one appearing once is a walk-on. Only Terms clearing an occurrence threshold survive, keeping the Dictionary small enough to be useful rather than listing every proper noun in the book.
2. **Translation of Terms** — a single request with the entire surviving Term list visible at once, so related names are rendered as a coherent set.

Translating Terms during extraction is explicitly rejected: it produces several spellings of the same name, which is precisely the inconsistency the Dictionary exists to prevent.

The occurrence threshold is a guess and should be tunable while the first real books go through — too low and the Dictionary bloats past the size that makes unfiltered injection sound, too high and the recurring names that matter get dropped.

**Blocked by:** 05 — Source File upload; 08 — Agent interface, `claude` adapter, and the fake; 09 — Chunker and numbered-node validator.

**Status:** done

- [x] Starting Dictionary Building moves the Translation Target's Status to `Analyzing`, and to `Dictionary Ready` on completion
- [x] Extraction requests carry no translation instruction, asserted against the recorded Agent requests
- [x] Extraction runs on the cheap Model, Term translation on whichever Model is configured for mechanical work
- [x] Candidate Terms are merged across Chunks with occurrence counts
- [x] Terms below the occurrence threshold are dropped; the threshold is configurable
- [x] Term translation happens in exactly one request with the whole surviving list present, asserted against the recorded requests
- [x] The result is written to `translations/<pair>/dictionary.tsv` as `original`, `translation`, `note`
- [x] Progress through the Book is visible while the run is in flight

## Comments

2026-08-31: Verified with focused package/UI tests and `make check`. The headless server rendered its fixture library successfully over loopback; interactive browser verification could not run because no in-app browser backend was available. The UI test covers the rendered Dictionary Building control, and the Dictionary builder test covers the in-flight progress callback and recorded Agent transcript.
