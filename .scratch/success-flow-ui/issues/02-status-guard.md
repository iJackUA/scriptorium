Status: ready-for-agent
Blocked by: none

# Status guard for writes to a Translation Target

Parent: `.scratch/success-flow-ui/spec.md`

## What to build

Add one server-side check that tells whether a Translation Target is Analyzing or Translating, keyed on its Status so it still holds after a restart. The following actions must consult it before they run and are refused while it is true: Chunk Materialization, Validate and Repair, Replace Source File, Delete Translation Target, Book Composition, and Restart Dictionary Building (ticket 03).

The same actions are hidden in the UI while the Target is Analyzing or Translating. The hiding alone is not enough; the server refusal is what holds when a page is stale.

## Acceptance criteria

- While a Target is Translating, Chunk Materialization, Validate and Repair, Replace Source File, Delete and Book Composition are refused by the server, and the Target's files are unchanged (stories 12, 27, 28, 34).
- While a Target is Analyzing, the same actions are refused (story 12, 34).
- The refusal says why in plain words (for example: "Stop the translation first").
- The buttons for these actions are hidden while the guard is true (story 27).
- The guard is keyed on Status, not on in-memory state, so it holds after a restart.

## Tests

- Through the UI handler tree, with the fake Agent holding a translation open. Each refused action leaves the files unchanged.
- If the fake cannot hold a translation open, extend it with a gate the test releases. Do not add a new seam.
- Prior art: `internal/translation/service_test.go` (fake Agent), `internal/ui/authoring_test.go` (posting forms).

## Notes

- The check does not change what the actions do when the Target is not active.
