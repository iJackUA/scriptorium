Status: ready-for-agent
Blocked by: 01-book-progress-wizard.md

# Failure cause and matching recovery actions

Parent: `.scratch/success-flow-ui/spec.md`

## What to build

Record a Failure cause alongside `Failed` wherever a Translation Target becomes `Failed` today. The three causes are Dictionary Building, Chunks, and stopped by the user. Store it in the Target's state; the new field is optional.

Show the matching recovery as the next action on the card:

- Dictionary Building cause → Restart Dictionary Building. It resets the Target to `New` with the cause cleared, then runs Dictionary Building.
- Chunks cause → Retry failed Chunks (the existing action).
- Stopped by the user → Resume translation, which is Validate and Repair under a clearer label. It re-requests only the incomplete Chunks and keeps the completed ones.

A `Failed` Target with no recorded cause (from before this change) offers the full recovery set in its menus and no single primary recovery.

## Acceptance criteria

- A Dictionary Building failure shows Restart Dictionary Building on the card, and the Chunks recovery is not offered for it (stories 22, 23).
- A Chunks failure shows Retry failed Chunks on the card (story 24).
- A stopped translation shows Resume translation on the card. Using it continues only the incomplete Chunks; completed Chunks are not requested again (stories 25, 26).
- A `Failed` Target without a cause still offers all recovery actions and loses none (story 30).
- Restart Dictionary Building is refused while the Target is Analyzing or Translating (ticket 02).
- Validate and Repair is refused on a Target with no Dictionary (story 29).

## Tests

- Through the UI handler tree: each Failure cause shows the expected card action and not the others.
- Resume after a stop requests only the incomplete Chunks (prior art: the existing test that asserts only the incomplete Chunk is re-requested in `internal/translation/service_test.go`).
- Restart Dictionary Building returns the Target to `New` and runs Dictionary Building.

## Notes

- Open item to settle in this ticket: whether Restart Dictionary Building keeps a partial Dictionary or discards it. Check the code and record the answer in the ticket's Comments before implementing.
- The new Failure cause is a stored-state change. Add a test that loads a state file without the field.
