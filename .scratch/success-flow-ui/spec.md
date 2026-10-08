Status: ready-for-agent

# Success-flow UI: guided wizard, contextual actions, and status guards

Source analysis: `docs/adr/0006-guided-success-flow-ui.md`. Vocabulary follows `GLOSSARY.md`.

**Terms in this spec.** "Target" always means a Translation Target. Glossary terms are capitalised (Book, Source File, Chunk Materialization, Book Composition, Dictionary Building, Status, and so on). Terms the glossary does not define yet are listed under Further Notes as gaps to resolve; they are not invented here.

## Problem Statement

A user who wants to translate a Book sees every action for every Status at once. On the Book page, Validate and Repair, Chunk Materialization, Book Composition, Replace Source File and Delete all sit next to the one action that moves the Translation Target forward. Nothing says which step the Target has reached or what to do next. A user who finishes a Book cannot tell it is done without reading the Status badge, and the primary button on a Translated Target is Validate and Repair.

The screen also hides real risks. Validate and Repair and Chunk Materialization can be clicked while a Target is Translating or Analyzing. Both write files the translation is writing, so the last writer wins and the result can be corrupted. A Target that Failed during Dictionary Building has no recovery path in the UI: restarting it means calling a raw endpoint or deleting the Target. Stopping a translation leaves the Target `Failed` with no name for the recovery, and that recovery is an action with no label. Deleting a Translation Target removes the only organised copy of its Dictionary, Chunk Translations and translated Book, and the confirmation does not say so.

Users still need every manual action. Re-running a completed step, such as Dictionary Building or Chunk Materialization, must remain possible, but it should not compete with the main path.

## Solution

The Book page becomes a guided wizard. It shows five steps in order (Source File, Chunk Materialization, Dictionary Building, Translation, Book Composition), with a checkmark on each completed step and a single next-action card for the current one. Completed steps collapse to a line each.

Every action that is not the next step moves into a `⋯` menu on the step it belongs to. Each re-run confirms before it runs. Destructive actions move into the same menus with stronger confirmations. Agent, Model and Effort settings sit in one collapsed Settings section, and metadata sits in a collapsed Details section.

A Failed Target shows its Failure cause and offers the matching recovery: restart Dictionary Building, retry failed Chunks, or resume the translation after a stop.

The Library shows each Book's Status with one next-action button, so a user can see what needs attention without opening each Book.

Actions that would write to a Target's files are blocked while the Target is Analyzing or Translating. They are hidden in the UI and refused by the server.

## User Stories

1. As a translator, I want to see the five steps of a Book's Translation Target in one list, so that I know where it is in the process.
2. As a translator, I want each completed step to show a checkmark and collapse to one line, so that finished work does not clutter the page.
3. As a translator, I want one card that names the single next action for the Target, so that I know what to do without reading every button.
4. As a translator, I want the next-action card to change with the Target's Status, so that it always matches the current step.
5. As a translator, I want to see progress for a running step inside that step, so that I can follow it without a separate panel.
6. As a translator, I want to stop a running Dictionary Building from the step that is running, so that I can cancel it where I am looking.
7. As a translator, I want to stop a running translation from the step that is running, so that I can cancel it where I am looking.
8. As a translator, I want to re-run a completed step from its menu, so that I can redo work without it cluttering the main path.
9. As a translator, I want every re-run to ask me to confirm first, so that I do not restart work by accident.
10. As a translator, I want the confirmation to say what the re-run will overwrite, so that I know the cost before I agree.
11. As a translator, I want Chunk Materialization to be available from its step's menu, so that I can regenerate the Chunks when I need to.
12. As a translator, I want Chunk Materialization to be unavailable while the Target is Analyzing or Translating, so that the Chunks cannot be rewritten under a running step.
13. As a translator, I want Replace Source File to be in the Source File step's menu with a confirm, so that the most destructive upload is not the first thing I see.
14. As a translator, I want Replace Source File to confirm what is discarded, so that I know my existing work will be lost.
15. As a translator, I want to edit the Book Dictionary from the Dictionary Building step, so that reviewing Terms stays on the main path.
16. As a translator, I want Promote to Series Dictionary and Unpromote in the Dictionary Building step's menu, so that they are available but not in my way, because they change the whole Series.
17. As a translator, I want Promote and Unpromote to confirm before they run, so that a Series-wide change is never a single click.
18. As a translator, I want Start Translation to appear as the next action once the Target is Dictionary Ready, so that I can move on without searching.
19. As a translator, I want Book Composition to appear as the next action only when the Target is Translated, so that I do not produce a partial Book by accident.
20. As a translator, I want Book Composition to be refused when the Target is not Translated, so that a partial Book is never written by mistake.
21. As a translator, I want to open the output folder from a link next to Book Composition once the Target is Translated, so that I can find the finished file.
22. As a translator, I want a Failed Target to say why it failed, so that I know which recovery applies.
23. As a translator, I want a Failure caused by Dictionary Building to offer Restart Dictionary Building, so that I can recover without deleting my Target.
24. As a translator, I want a Failure caused by Chunks to offer Retry failed Chunks as the next action, so that I can recover the failed Chunks directly.
25. As a translator, I want a stopped translation to offer Resume translation, so that I know the action continues my work.
26. As a translator, I want Resume translation to continue only the incomplete Chunks and keep the completed ones, so that stopping does not cost me finished work.
27. As a translator, I want Validate and Repair hidden while the Target is Translating, so that I cannot overwrite a running translation.
28. As a translator, I want the server to refuse Validate and Repair while the Target is Translating, so that the guard holds even if the page is stale.
29. As a translator, I want Validate and Repair refused on a Target that has no Book Dictionary or Series Dictionary, so that a Chunk is never translated without its Dictionary.
30. As a translator, I want existing Failed Targets from before this change to keep working, so that I do not lose access to recovery.
31. As a translator, I want Delete Translation Target to list what will be lost (the Book Dictionary, Chunk Translations and translated Book), so that I know before I delete.
32. As a translator, I want Delete to require me to type the Language Tag of the Target when translated output exists, so that a misclick cannot delete finished work.
33. As a translator, I want Delete to need only a plain confirm when no output exists, so that empty Targets are quick to remove.
34. As a translator, I want Delete refused while the Target is Analyzing or Translating, so that I cannot delete a Target that is being written.
35. As a translator, I want Agent, Model and Effort in one collapsed Settings section on the Book page, so that the page stays short and the settings are still in reach.
36. As a translator, I want Settings to save as I change them, as it does now, so that I do not have to remember a Save button.
37. As a translator, I want metadata in a collapsed Details section, so that it does not sit in the main path.
38. As a translator, I want the Library to show each Book's Status and one next-action button, so that I can see which Books need me without opening them.
39. As a translator, I want each Library row to open its Book, so that I can take a fuller action there.
40. As a translator, I want the Workspace settings to stay in the header-gear modal, so that Workspace-wide options remain in one place.
41. As a translator, I want Add Book to stay the primary action in the Library header, so that adding Books is still obvious.
42. As a translator, I want the Welcome screen to keep its single primary action, so that the first screen is unchanged and easy to use.
43. As a translator, I want completed steps to remain reachable from their menus, so that a finished Book can still be re-checked without a separate screen.
44. As a translator, I want every action the old page offered to remain reachable somewhere, so that the change removes clutter and no capability.
45. As a translator, I want the redesigned page to use the app's existing components and styling, so that the app still looks like itself.
46. As a translator, I want notices of completed actions to keep appearing as toasts, so that feedback works the same as before.
47. As a maintainer, I want every UI change verified in the headless UI session, so that the page is checked in a browser and not only by tests.

## Implementation Decisions

**Modules**

- **Book progress model (new, deep).** One module decides, from a Target's Status, its Failure cause, whether the Target has a Source File, a Book Dictionary or Series Dictionary, prepared Chunks, and translated output, the state of each step, the single primary action, and the actions for each step's menu. It has no I/O. The Book page and the Library rows both read from it, so the two cannot disagree.
- **Status guard (extended).** One check per Target reports whether the Target is Analyzing or Translating. Every action that writes to a Target's files (Chunk Materialization, Validate and Repair, Replace Source File, Delete, Book Composition, Restart Dictionary Building) asks this check before it runs. The check is keyed on Status, not on in-memory state, so it also holds after a restart.
- **Book page (rewritten view).** Renders the step list, the next-action card, step menus, the Settings section and the Details section, all from the progress model. Existing per-action endpoints are reused. No new endpoint is needed.
- **Library rows (extended).** Each row shows the Status badge and the next action from the progress model.
- **Dictionary Building step (reshaped).** Edit stays visible. Promote and Unpromote move into the step's menu behind a confirm.
- **Confirmations.** One reusable confirm pattern for re-runs and destructive actions. The Delete variant also takes the Language Tag of the Target, typed, when translated output exists.

**Schema change**

- A Translation Target's stored Status gains an optional Failure cause: Dictionary Building, Chunks, or stopped by the user. It is written wherever the Target becomes `Failed`. A `Failed` Target with no cause (from before this change) gets the full set of recovery actions in its menus and no single primary recovery, so nothing is lost. Its next run writes a cause.

**Behaviour**

- Restart Dictionary Building resets the Target to `New` with its cause cleared, then runs Dictionary Building. It is available only for the Dictionary Building cause.
- Resume translation is Validate and Repair under a clearer label, shown for the stopped cause. Validate and Repair keeps its existing behaviour: it re-requests only incomplete Chunks and keeps completed ones.
- Retry failed Chunks is the existing action, shown as the next action for the Chunks cause.
- Book Composition is refused by the server unless the Target is `Translated`. It is visible only when `Translated`.
- Validate and Repair is refused by the server while the Target is `Translating`, and when the Target has no Dictionary.
- Chunk Materialization, Replace Source File and Delete are refused by the server while the Target is `Analyzing` or `Translating`.
- Stop buttons stay visible on the step that is running, and nowhere else.
- Toasts for completed actions keep their current behaviour.

**Step names and action placement**

- Steps, in order: Source File, Chunk Materialization, Dictionary Building, Translation, Book Composition. Chunk Materialization is the glossary term; the button label in the UI may read "Prepare text chunks".
- Primary on the card: the next action for the current Status only.
- In a step's menu: re-runs, Replace Source File, Chunk Materialization, Restart Dictionary Building, Promote to Series Dictionary, Unpromote, Validate and Repair, Delete Translation Target.
- Visible on the step: Edit Book Dictionary, Stop buttons while running, Open output folder once `Translated`, as a secondary link next to Book Composition.
- Collapsed sections: Settings, Details.
- Unchanged: Add Book, Add Series and the header-gear Workspace settings modal in the Library; the Welcome screen.

**Prerequisites**

- Dictionary Building reads the Source File directly and does not need Chunk Materialization. Translation does need Chunk Materialization. The step order is kept for clarity, as decided, and the progress model must not treat Chunk Materialization as a prerequisite for Dictionary Building.

## Testing Decisions

**What makes a good test here.** Tests describe what a translator sees and what the server accepts or refuses. They do not assert on markup structure or internal state beyond what the user can observe. A test is good when it would still pass after the page is restyled with the same components.

**Seam.** One seam for this work: the UI's HTTP handler tree, driven through an in-process test server. Every test in this spec goes through it. No new seam is needed. The translation service is already tested with a fake Agent, and that fake is enough to hold a translation open for the guard tests. If the fake cannot hold a translation open, extend it with a gate that the test releases. Do not add a new seam.

**What to test**

- For each Status and Failure cause, the Book page shows exactly one primary action, the expected step states, and the expected actions in each menu. Test with rendered page content, and use test identifiers only where the text is not enough.
- The Library row shows the same next action as the Book page for the same Translation Target.
- While the Target is Translating: Chunk Materialization, Validate and Repair, Replace Source File, Delete and Book Composition are each refused, and the Target's files are unchanged.
- While the Target is Analyzing: the same actions are refused.
- Validate and Repair is refused on a Target with no Dictionary.
- Book Composition is refused unless the Target is `Translated`.
- Delete without the typed Language Tag is refused when translated output exists, and succeeds with it. Delete with no output needs only the plain confirm.
- Restart Dictionary Building returns a Target with the Dictionary Building cause to `New` and runs Dictionary Building. The Chunks recovery is not offered for it.
- Resume translation after a stop continues only the incomplete Chunks, and completed Chunks are not requested again. (Prior art: the existing test that asserts only the incomplete Chunk is re-requested.)
- A `Failed` Target with no cause still offers the full recovery set and does not lose any action.
- Every re-run and destructive action asks for confirmation before running.
- Existing behaviour is preserved: the current tests for upload, metadata, Series and Target creation, pickers and translation still pass.

**Prior art.** The UI tests in `internal/ui` drive the handler tree with a test server and a form-posting helper, and name each test after the behaviour it checks. The translation service tests use the fake Agent to script responses and to read the recorded requests. Follow both patterns.

**Verification.** Each UI change is also checked in a headless session as described in `docs/agents/verification.md`, with screenshots of the main states: New, Analyzing, Dictionary Ready, Translating, Translated, each Failure cause, and stopped. The session is recorded in the ticket's Comments.

## Out of Scope

- Any visual redesign beyond the existing components and styling.
- New components not already in the app's styling library.
- Parallel translation or any change to how Chunks are translated (ADR-0003 keeps translation serial).
- Changes to the Series prompt or Series Dictionary editors, beyond what the Dictionary Building step's menu needs.
- Exporting the translated Book as a separate feature. Open output folder is the only output action in scope.
- Changes to the Chunk pipeline, the parser or Book Composition logic.
- Changing how Validate and Repair chooses which Chunks to re-request.
- Automatic repair on restart. ADR-0005 keeps repair explicit.

## Further Notes

- **Terms the glossary does not define yet.** Each needs a decision in `GLOSSARY.md` before implementation, not an invention in this spec:
  - *Validate and Repair* is defined in ADR-0005 and used in the UI, but not in the glossary.
  - *Failure cause* (Dictionary Building, Chunks, stopped by the user) is new.
  - *Prompt* (the Series and Book prompt files) is not defined, though it is a Series setting.
  - *Resume translation*, *Restart Dictionary Building* and *Retry failed Chunks* are action labels. They can stay UI copy, or the glossary can name them.
- **Terms that changed in this spec to match the glossary.** "Prepare text chunks" is now Chunk Materialization. "Compose translated book" is now Book Composition. "Chunk" and "Chunk Translation" replace "translated chunks". "Composed output" is the translated Book. "Run" is no longer used for the guard; the guard is keyed on Status.
- **Open items to confirm during implementation, not yet verified in the code:** whether Restart Dictionary Building should keep a partial Dictionary or discard it; whether Stop after the last Chunk completes can end as `Translated`; and whether the Series prompt and Series Dictionary editors are missing from the UI across the whole repository.
- **Typed confirmation.** The Delete confirmation asks for the Language Tag of the Target (for example `uk`). A Book has at most one Translation Target per Target Language, so the tag identifies the Target within its Book.
- **Stale references.** `docs/agents/domain.md` and `docs/adr/0006-guided-success-flow-ui.md` still refer to `CONTEXT.md` or to UI labels. `CONTEXT.md` is deleted in the working tree and `GLOSSARY.md` is the glossary.
- The spec keeps every action the old page offered, so no capability is removed. The only behaviour removed is the overlap during a running step, which the Status guard replaces.
