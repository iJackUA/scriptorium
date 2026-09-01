# 11 — Dictionary review, override, and promotion

**What to build:** The user has the final say over how names are rendered. The proposed Dictionary is reviewed as a plain text file, editable in whatever editor the user likes rather than through a form. Terms can be added, edited, and removed freely.

The Book Dictionary overrides the Series Dictionary where they disagree, so a term unique to one volume does not pollute the terminology of the whole series. Once a name proves series-wide, the Term is promoted by hand to the Series Dictionary, so later Books inherit it. Re-running Dictionary Building extends the Dictionary without losing hand edits.

The full Dictionary is injected into every translation request without filtering, which is sound only while it stays small. Budget roughly 100 Terms; warn past that so the user knows to prune.

TSV is deliberate: Dictionaries are hand-edited *and* machine-generated, and a line-oriented format is far more robust to model output than YAML's indentation rules.

**Blocked by:** 10 — Dictionary Building.

**Status:** done

- [x] The Book Dictionary and the Series Dictionary are both openable as plain TSV from the UI
- [x] Hand edits made outside the application are picked up on next read
- [x] Merging a Book Dictionary over a Series Dictionary lets the Book entry win on conflict, asserted at the service layer
- [x] Series Dictionaries live at `<series-code>/dictionaries/<pair>.tsv`, per language pair
- [x] A Term can be promoted from a Book Dictionary to the Series Dictionary, and a later Book in that Series inherits it
- [x] Re-running Dictionary Building preserves hand-edited Terms and adds only new ones
- [x] A merged Dictionary past roughly 100 Terms shows a warning that it will bloat every request
- [x] Malformed TSV lines are reported with their line number rather than silently dropped

## Comments

2026-09-01: Verified with focused library and UI handler tests, plus `make check`. The UI test covers the TSV endpoints, promotion control, and 101-Term warning. `make headless` exited before exposing its loopback URL in this environment, so browser-level verification could not run; the handler tests exercised the same routes directly.

2026-09-01: Added in-modal Book Dictionary editing. `Edit Dictionary` swaps the read-only review for a raw TSV editor; `Save` validates and atomically rewrites the TSV, while invalid input remains in edit mode with its line error and `Cancel` returns to the persisted review. Promote and Unpromote now target the modal content region, so the open dialog is preserved. `GOCACHE=/tmp/scriptorium-gocache make check` passed. The headless twin served its root HTML after elevated loopback permission was granted; the in-app browser backend was unavailable, so no browser screenshot was captured.
