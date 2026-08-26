# 14 — Resume and hash invalidation

**What to build:** Close the application mid-translation and resume where it left off, so a long book does not require one uninterrupted sitting. On restart only `pending` and `failed` Chunks are re-requested; completed work is never paid for twice.

The same mechanism handles invalidation. `state.json` holds a hash of each Chunk's source text, so editing the Dictionary or changing the source invalidates the affected Chunks rather than all of them — a small terminology fix must not force a full re-translation. Re-running is always safe.

The user can also retry only the Chunks that failed, without re-translating the book.

**Blocked by:** 12 — Translation tracer bullet: happy path.

**Status:** ready-for-agent

- [ ] Scripting a mid-book failure and re-running re-requests only the unfinished Chunks, asserted against the recorded requests
- [ ] A killed process mid-write leaves `state.json` readable and the book resumable
- [ ] Editing the Dictionary and re-running re-requests exactly the affected Chunks and no others
- [ ] Changing the Source File invalidates the Chunks whose source text changed
- [ ] Retrying failed Chunks re-requests only those Chunks
- [ ] Re-running a fully completed Translation Target requests nothing
- [ ] Resumption preserves accumulated cost rather than resetting it
