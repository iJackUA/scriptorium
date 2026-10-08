Status: ready-for-agent
Blocked by: 01-book-progress-wizard.md

# Confirm before every re-run and destructive action

Parent: `.scratch/success-flow-ui/spec.md`

## What to build

Add one reusable confirm pattern and apply it to every re-run and destructive action: Chunk Materialization, Restart Dictionary Building, Promote to Series Dictionary, Unpromote, Validate and Repair (including Resume translation), and Replace Source File. The confirm names what the action will overwrite, so the user knows the cost before agreeing.

Delete Translation Target uses the same pattern, extended in ticket 09.

## Acceptance criteria

- Every re-run and destructive action asks for confirmation before it runs (stories 9, 17).
- The confirmation text names what will be overwritten or lost (stories 10, 14).
- Cancelling the confirm changes nothing on the Target.
- The confirm is a single reusable component. No action builds its own.

## Tests

- Through the UI handler tree: each action without the confirmation is refused or shows the confirm, and the Target is unchanged.
- Cancelling changes nothing.

## Notes

- The Status guard (ticket 02) still applies: a confirmed action is refused while the Target is Analyzing or Translating.
