Status: ready-for-agent
Blocked by: 04-rerun-confirmations.md

# Delete Translation Target: list what is lost, typed confirmation

Parent: `.scratch/success-flow-ui/spec.md`

## What to build

Delete Translation Target lists what will be lost in its confirmation: the Book Dictionary, the Chunk Translations, and the translated Book. When translated output exists, the user must type the Language Tag of the Target (for example `uk`) before the delete runs. When no output exists, a plain confirm is enough.

Delete is refused while the Target is Analyzing or Translating (ticket 02). It sits in the Target's own menu (ticket 05).

## Acceptance criteria

- The confirmation names what is lost (story 31).
- Delete without the typed Language Tag is refused when translated output exists, and succeeds with it (story 32).
- Delete with no output needs only the plain confirm (story 33).
- Delete is refused while the Target is Analyzing or Translating (story 34).
- The Series and other Targets of the same Book are unaffected.

## Tests

- Through the UI handler tree: typed-tag refusal and acceptance, plain-confirm acceptance, and refusal while Translating.
- Prior art: the existing Delete tests and the Target removal tests in `internal/library` and `internal/ui`.

## Notes

- The glossary does not define a Target name. The Language Tag identifies a Translation Target within its Book, because a Book has at most one Target per Target Language.
