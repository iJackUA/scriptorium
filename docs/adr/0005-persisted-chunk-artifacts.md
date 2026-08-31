# Persisted Chunk artifacts for resumable translation

**Status:** accepted

Scriptorium persists each processing stage as inspectable, indexed plain text: a Book-level Chunk Materialization contains the original Chunks, and each Translation Target contains its Chunk Translations. The translation service writes these artifacts atomically before updating `state.json`, then performs Book Composition from the persisted Chunk Translations rather than from in-memory results. This keeps accepted work recoverable after interruption, makes failures visible for debugging, and preserves the FB2 handler's responsibility as a pure document-tree splicer.

## Consequences

The original Source File remains authoritative and is never modified. `chunks/manifest.json` records the Source File identity, parser/Chunker versions, Chapter and Text Node mapping, and original-artifact hashes; it contains no prose that is not already visible in the original Chunk files. Each original Chunk is stored once at Book scope, while translated and rejected Chunk files are stored per Translation Target so multiple target languages cannot collide.

The accepted translated file is written before its Chunk is marked completed in `state.json`. A restart does not invoke the Agent or repair anything automatically. An explicit **Validate and Repair** action checks `state.json` and translated Chunk files, promotes valid files left behind by an interruption, preserves the latest rejected response, and re-requests missing or malformed translations. It does not validate original Chunk files.

Normal completion composes the final Book only when every translated Chunk is valid. An explicit **Compose translated book** action is a diagnostic escape hatch: it never calls the Agent or changes state, uses original Text Nodes as fallback for missing or malformed translations, writes a clearly marked partial output when necessary, and warns when the result is incomplete. Valid manual edits to translated Chunk files are used as-is; accepted-translation history is not retained.

## Considered options

Keeping only Chunk status and hashes in `state.json` cannot resume a stopped process because it loses accepted translated text. Keeping all Chunks only in memory has the same failure mode and makes debugging opaque. Concatenating plain-text translations into a new FB2 would discard markup. Persisted indexed artifacts plus composition through the original format handler avoid all three failures.
