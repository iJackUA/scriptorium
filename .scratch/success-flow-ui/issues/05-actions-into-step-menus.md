Status: ready-for-agent
Blocked by: 01-book-progress-wizard.md, 04-rerun-confirmations.md

# Move re-runs and destructive actions into step menus

Parent: `.scratch/success-flow-ui/spec.md`

## What to build

Move each action off the main path into the `⋯` menu of the step it belongs to:

- Source File step: Replace Source File (confirm from ticket 04).
- Chunk Materialization step: Chunk Materialization (the UI label may read "Prepare text chunks"; the spec's name is the glossary term).
- Dictionary Building step: Promote to Series Dictionary and Unpromote (both confirmed). Restart Dictionary Building (from ticket 03).
- Translation step: Validate and Repair (confirmed, hidden while Translating).
- Delete Translation Target: in the Target's own menu (details in ticket 09).

Edit Book Dictionary stays visible on the Dictionary Building step.

## Acceptance criteria

- Each listed action appears only in its step's menu, not on the main path (stories 8, 11, 13, 15, 16).
- Edit Book Dictionary stays visible (story 15).
- Promote and Unpromote confirm before running (story 17).
- Every action is still reachable from the page (story 44).

## Tests

- Through the UI handler tree: each action is in the expected menu for each Status, and not on the card.
- Prior art: `internal/ui/authoring_test.go`, the upload and replace tests.

## Notes

- Dictionary Building reads the Source File directly and does not need Chunk Materialization, so Chunk Materialization is not a prerequisite for the Dictionary step's menu.
