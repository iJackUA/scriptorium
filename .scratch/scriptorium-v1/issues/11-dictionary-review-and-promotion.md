# 11 — Dictionary review, override, and promotion

**What to build:** The user has the final say over how names are rendered. The proposed Dictionary is reviewed as a plain text file, editable in whatever editor the user likes rather than through a form. Terms can be added, edited, and removed freely.

The Book Dictionary overrides the Series Dictionary where they disagree, so a term unique to one volume does not pollute the terminology of the whole series. Once a name proves series-wide, the Term is promoted by hand to the Series Dictionary, so later Books inherit it. Re-running Dictionary Building extends the Dictionary without losing hand edits.

The full Dictionary is injected into every translation request without filtering, which is sound only while it stays small. Budget roughly 100 Terms; warn past that so the user knows to prune.

TSV is deliberate: Dictionaries are hand-edited *and* machine-generated, and a line-oriented format is far more robust to model output than YAML's indentation rules.

**Blocked by:** 10 — Dictionary Building.

**Status:** ready-for-agent

- [ ] The Book Dictionary and the Series Dictionary are both openable as plain TSV from the UI
- [ ] Hand edits made outside the application are picked up on next read
- [ ] Merging a Book Dictionary over a Series Dictionary lets the Book entry win on conflict, asserted at the service layer
- [ ] Series Dictionaries live at `<series-code>/dictionaries/<pair>.tsv`, per language pair
- [ ] A Term can be promoted from a Book Dictionary to the Series Dictionary, and a later Book in that Series inherits it
- [ ] Re-running Dictionary Building preserves hand-edited Terms and adds only new ones
- [ ] A merged Dictionary past roughly 100 Terms shows a warning that it will bloat every request
- [ ] Malformed TSV lines are reported with their line number rather than silently dropped
