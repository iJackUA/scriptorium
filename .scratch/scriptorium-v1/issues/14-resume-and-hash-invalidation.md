# 14 — Resume and hash invalidation

**What to build:** Persist original and translated Chunk artifacts so a long book can survive interruption without losing accepted work. Opening the application does not contact the Agent or repair anything automatically; the user explicitly chooses **Validate and Repair** to inspect the persisted state and continue.

The same mechanism handles invalidation. The Chunk manifest identifies the Source File and `state.json` identifies the active Dictionary, so editing the Dictionary invalidates the affected translated Chunks while original Chunk artifacts remain available. Replacing the Source File discards the materialization with the existing translation artifacts and starts a new one. Re-running is always safe.

The user can also retry only the Chunks that failed, without re-translating the book.

**Blocked by:** 12 — Translation tracer bullet: happy path.

**Status:** done

- [x] Scripting a mid-book failure and running **Validate and Repair** re-requests only missing, failed, or malformed translated Chunks, asserted against the recorded requests
- [x] A killed process after translated-Chunk write but before the state update promotes the valid file without re-requesting it
- [x] Editing the Dictionary and re-running re-requests exactly the affected Chunks and no others
- [x] Replacing the Source File discards the old Chunk Materialization and its translated artifacts before a new run
- [x] Retrying failed Chunks re-requests only those Chunks
- [x] Re-running a fully completed Translation Target requests nothing
- [x] Resumption preserves accumulated cost rather than resetting it
- [x] A malformed translated Chunk is preserved as the latest rejected response and is not used for Book Composition
- [x] A valid manually edited translated Chunk is used without retranslation

## Comments

- Implemented persisted resumption and explicit Validate and Repair / Retry failed Chunks actions. Added coverage for crash promotion, Dictionary invalidation, failed-only retry, malformed-response preservation, valid manual edits, cost accumulation, completed reruns, Source File replacement cleanup, and repair controls.
- UI verification: `make headless` served the loopback twin and `curl` confirmed the application page rendered successfully. The in-app browser backend was unavailable in this environment, so browser interaction/screenshot verification could not run; handler/template tests confirmed the visible **Validate and Repair** and **Retry failed Chunks** controls and their routes.
