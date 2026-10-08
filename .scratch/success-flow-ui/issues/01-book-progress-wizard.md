Status: ready-for-agent
Blocked by: none

# Book page as a guided wizard

Parent: `.scratch/success-flow-ui/spec.md`

## What to build

Replace the Book page's stacked action list with the five-step wizard: Source File, Chunk Materialization, Dictionary Building, Translation, Book Composition. Each step shows its state (done with a checkmark and collapsed to one line, current, or locked). A single next-action card shows the one primary action for the Target's Status. The card's content comes from the Book progress model, the new module that maps Status, Failure cause, and existing Source File, Dictionary and Chunks to step states and actions.

Every action the old page offered stays reachable: the primary one on the card, the rest in the step's menu. No action is removed in this ticket.

Progress for a running step appears inline on that step.

## Acceptance criteria

- The Book page shows the five steps in order, and each completed step collapses to one line with a checkmark (stories 1, 2).
- The next-action card shows exactly one primary action for each Status: New → Start Dictionary Building; Dictionary Ready → Start Translation; Translated → Book Composition; Failed → the recovery for its cause (ticket 03); Analyzing and Translating → no card action, progress shown inline (stories 3, 4, 5).
- Stop buttons are visible on the step that is running (stories 6, 7).
- Every action the old page offered is still reachable (story 44).
- The Book progress model has no I/O and is used by the Book page only in this ticket (the Library follows in ticket 08).

## Tests

- Through the UI handler tree: for each Status, the page shows the expected step states and the expected primary action.
- Prior art: `internal/ui/authoring_test.go`, `internal/ui/models_test.go`.

## Notes

- The Failed case can show a generic recovery until ticket 03 records the Failure cause.
- Verify in the headless session (ticket 10).
