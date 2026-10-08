Status: ready-for-agent
Blocked by: 01-book-progress-wizard.md, 02-status-guard.md

# Book Composition only when Translated, with an output link

Parent: `.scratch/success-flow-ui/spec.md`

## What to build

Show Book Composition only when the Target is `Translated`, and refuse it on the server otherwise. Show "Open output folder" as a secondary link next to Book Composition once the Target is `Translated`.

Today Book Composition is offered on every Status, including New, and writes a partial Book when translation is incomplete.

## Acceptance criteria

- Book Composition is the next action only when the Target is `Translated` (story 19).
- It is refused by the server unless the Target is `Translated`, and the Target's files are unchanged (story 20).
- Open output folder is a secondary link next to Book Composition, shown only when `Translated` (story 21).
- Book Composition is refused while the Target is Translating (ticket 02).

## Tests

- Through the UI handler tree: Book Composition is refused unless `Translated`, and the page shows the link only when `Translated`.
- Prior art: the existing Book Composition tests in `internal/translation/service_test.go`.

## Notes

- Open output folder is unavailable in headless mode. The test should cover the link's presence and not the folder opening.
