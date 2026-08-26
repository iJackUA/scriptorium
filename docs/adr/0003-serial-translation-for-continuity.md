# Chunks are translated serially, not in parallel

Each Chunk is translated with a Continuity Window: the tail of the previous Chunk's source *and its accepted translation*, so that register, tense, forms of address, and recurring phrasing carry across boundaries. That dependency is inherently sequential — a Chunk cannot start until its predecessor is accepted — and it rules out the parallel agent processes originally imagined.

The trade is smaller than it looks. At roughly 2,000-word Chunks and ~3.5s per invocation, a 150k-word novel is about 75 requests, or ~4.5 minutes end to end. Parallelism would buy a few minutes at the cost of the exact consistency the Continuity Window exists to provide.

## Consequences

If throughput ever matters, the upgrade path is to parallelise *across Chapters* while staying serial within them — continuity resets at a chapter break anyway, so nothing is lost. The chunker already emits Chapter boundaries, so this stays available without rework.
