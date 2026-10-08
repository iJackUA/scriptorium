Status: ready-for-agent
Blocked by: 01-book-progress-wizard.md

# Library rows show Status and the next action

Parent: `.scratch/success-flow-ui/spec.md`

## What to build

Each Book row in the Library shows its Status badge and one next-action button, taken from the Book progress model. Each row still opens its Book. Add Book, Add Series, the header-gear Workspace settings modal and the Welcome screen do not change.

Rows show one Translation Target per row, or one row per Book with the Target's next action. Decide this in the ticket and record the choice in its Comments; the spec does not choose.

## Acceptance criteria

- Each row shows the Status badge and the same next action the Book page shows for the same Translation Target (stories 38, 39).
- Opening a row opens its Book (story 39).
- Add Book stays the primary action in the Library header (story 41).
- The Workspace settings modal is unchanged (story 40).
- The Welcome screen is unchanged (story 42).

## Tests

- Through the UI handler tree: for each Status, the Library row shows the same action as the Book page.
- Prior art: `internal/ui/authoring_test.go` (the Library tests).

## Notes

- The Library row must read from the same progress model as the Book page, so the two cannot disagree.
