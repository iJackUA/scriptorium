Status: ready-for-agent
Blocked by: 01-book-progress-wizard.md

# Collapse Settings and Details on the Book page

Parent: `.scratch/success-flow-ui/spec.md`

## What to build

Put Agent, Model and Effort, and the prompts, into one collapsed Settings section at the bottom of the Book page. Put metadata into a collapsed Details section. Both keep their current behaviour: Settings saves as it changes, and metadata keeps its save action.

## Acceptance criteria

- Settings is a single collapsed section containing Agent, Model, Effort and prompts (stories 35, 36).
- Settings still saves on change, with no Save button for the pickers (story 36).
- Details is a collapsed section containing metadata (story 37).
- The current picker and prompt tests still pass.

## Tests

- Through the UI handler tree: the pickers still save on change and the prompts still save and revert.
- Prior art: `internal/ui/models_test.go` (the picker tests) and `internal/ui/prompts_test.go`.

## Notes

- The Series prompt and Series Dictionary editors are not in this ticket. Their absence is an open item in the spec.
