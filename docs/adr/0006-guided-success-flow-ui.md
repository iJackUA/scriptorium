# Guided stepper for the success flow

**Status:** proposed

Every action for every status is visible at once on the Book page, so the main success flow is hard to see. This decision turns it into a guided wizard and moves the actions outside it out of the default view. No code has been changed; every item below is for a later implementation round.

## Goal

- Make the main success flow look like a guided wizard: one visible list of steps, a clear status for each, and a single next-action card.
- Keep every manual action available, but move the ones outside the success flow into `⋯` menus or collapsed sections so they don't clutter the default view.
- Keep existing components and styling (no new visual language).

## Decision

| Topic | Decision |
|---|---|
| Wizard shape | Guided stepper: visible step list, checkmark on completed steps, one next-action card |
| Re-runs after completion | In each step's `⋯` menu; always confirm before running |
| Failed status | Split by cause, each with its own next action |
| Workspace-level settings | Stay in the header-gear modal |
| Book-level Agent, Model, Effort and prompts | One collapsed "Settings" section on the Book page |
| Destructive actions | In `⋯` with a confirm; Delete asks for the Target name when translated output exists |
| Review scope | Every screen, ranked by how often a user reaches it |
| Visual constraints | Existing components and styling only |

## Wizard steps

Five steps, in this order:

1. **Source file**: upload or replace the original ebook.
2. **Prepare text**: create the text chunks. Dictionary building reads the source document directly, so this step is only a prerequisite for translation.
3. **Dictionary**: build and review the Book Dictionary.
4. **Translation**: translate chunks.
5. **Compose**: produce the translated book.

Each step shows one of three states: done (checkmark, collapsed to one line), current (expanded, with its next action), or locked (greyed out, not yet reachable).

### Next-action card per status

| Status | Primary action on the card | Notes |
|---|---|---|
| New | Start dictionary building | |
| Analyzing | None; progress shown inline, Stop visible | |
| Dictionary Ready | Start translation | |
| Translating | None; progress shown inline, Stop visible | Validate and Repair hidden (see safety fixes) |
| Translated | Compose translated book | Open output folder shown as a secondary link |
| Failed: dictionary building | Restart dictionary building | Needs the cause recorded (see Failed below) |
| Failed: chunks | Retry failed chunks | |
| Failed: stopped by user | Resume translation (runs Validate and Repair) | Label the action; today it is unlabelled |

## Action placement

| Action | Today | Proposed | Confirm? |
|---|---|---|---|
| Upload source file | Source step | Source step, primary when no file | No |
| Replace source file | Visible, warning style, behind a modal | Source step `⋯` | Yes, names what is discarded |
| Prepare text chunks | Visible on every status | Prepare step `⋯`; hidden while a run is active | Yes |
| Start dictionary building | Visible when New | Next-action card | No (first run) |
| Restart dictionary building | Not available from the UI (only via the endpoint) | Dictionary step `⋯` after a failure | Yes |
| Stop dictionary building | Visible when Analyzing | Visible while running | No |
| Edit dictionary | Visible | Dictionary step, visible | No |
| Promote / Unpromote to Series | Visible | Dictionary step `⋯` | Yes |
| Start translation | Visible when Dictionary Ready | Next-action card | No |
| Stop translation | Visible when Translating | Visible while running | No |
| Validate and Repair | Visible on Translating, Failed and Translated | `⋯`; hidden while Translating | Yes |
| Retry failed chunks | Visible when Failed | Next-action card for chunk failures | No |
| Compose translated book | Visible on every status, including New | Next-action card when Translated only | No |
| Open output folder | Visible when not New | Secondary link next to Compose | No |
| Agent, Model, Effort, prompts | Collapsed sections | One collapsed "Settings" section | No (autosave) |
| Metadata | Visible form | Collapsed "Details" section | No |
| Delete Translation Target | Visible, danger style | Target `⋯`, typed-name confirm when output exists | Yes, lists what is lost |
| Workspace settings | Header-gear modal | Unchanged | No |

## Safety issues

These are behaviour problems, not only layout. They should be fixed whatever the layout decisions.

1. **Validate and Repair can run during a live translation.** `startRepair` rejects only New and Analyzing (`internal/ui/handlers.go:602`). It never checks the active translation run. Repair uses a background context, so Stop cannot cancel it. `translationRuns.start` overwrites an existing entry silently (`handlers.go:144-149`). Nothing locks `internal/translation`, so both runs write `state.json` and chunk files, and the last writer wins. **Fix:** hide the action while Status is Translating and show a reason.
2. **Prepare Text Chunks can run during a live translation.** The button is gated only on the source file (`internal/ui/templates/book-detail.html:79`). The handler has no run check (`handlers.go:454-471`). It rewrites the chunk files and manifest, although it leaves translated output alone. **Fix:** hide it while any run is active.
3. **A failed dictionary has no recovery path in the UI.** It ends in `Failed` (`handlers.go:901-911`). Start Dictionary Building needs New, so it is not offered. Validate and Repair and Retry failed chunks are offered, but they are meant for translation failures. Restart is only possible through `POST /dictionary/stop`, which sets New with no status check (`handlers.go:760-773`), or by deleting the Target. **Fix:** record the failure cause, and offer Restart dictionary building for that cause.
4. **Stop Translation leaves an unlabelled Failed state.** Resuming it is done through Validate and Repair, which has no label that says so. **Fix:** label it "Resume translation" under the stopped-by-user cause.
5. **Delete Translation Target does not say what is lost.** The dialog reads "Delete this Translation Target?" (`book-detail.html:91`). The folder holds the only organised copy of the Target's state, Dictionary, translated chunks and composed output (`store.go:824-838`). The raw Agent replies are also kept in `logs/agent-transcript.jsonl`, but they are not organised per Target. **Fix:** list what is lost in the dialog, and require the Target name when output exists.

## Screen-by-screen review (ranked by how often a user reaches each screen)

### 1. Book detail (most used)

- **Problem:** every action for every status is visible at once. Validate and Repair is styled as the primary button even on a Translated book, and Compose is offered before translation.
- **Proposal:** the step list, the next-action card, and a `⋯` menu on each step. Settings and Details are collapsed. Progress appears inline on the current step.

### 2. Library (second most used)

- **Problem:** the list shows a status badge per book, but a user has to open a book to see what to do next.
- **Proposal:** each row shows the status badge and one next-action button. Other actions stay on the book page.

### 3. Dictionary review modal

- **Problem:** the modal is a plain editor, and promotion to Series sits next to editing.
- **Proposal:** keep the editor and its save button visible. Move Promote and Unpromote into the Dictionary step's `⋯` menu, with a confirm, because they change the whole Series.

### 4. Workspace settings modal

- **Problem:** Target Languages, default Agent, Model and Effort, prompts, and the Refresh action are all in one modal.
- **Proposal:** no change. This modal is off the success path and already exists.

### 5. Welcome

- **Problem:** nothing to simplify. "Choose folder…" is the only primary action.
- **Proposal:** no change.

### 6. Modals for Replace and Delete

- **Problem:** Replace is a modal with a warning style. Delete's confirm does not say what is lost.
- **Proposal:** keep the modals, and move their triggers into the `⋯` menus (see Action placement). Delete gets the typed-name confirm.

## Consequences

The safety issues above are fixed whatever the layout. Open items that are not yet verified:


- **Validate and Repair after a dictionary failure.** It may try to translate with no dictionary. This is not verified. It matters for item 3 in the safety fixes.
- **Stop after the last chunk completes.** The run may compose and end as Translated. Not verified.
- **Compose when translation is incomplete.** Compose writes partial output in that case (from the code map). The proposal hides it until Translated, so this is not a blocker.
- **Series prompt and Series Dictionary editors.** The code map found no UI for them, but this was not checked across the whole repo. The Book page copy mentions a Series prompt file, so it may be a gap.
- **Metadata inference for `.txt` files.** The code path was read, but its behaviour on failure was not run.

## Implementation notes

- The codebase has no stepper, dropdown or tabs component of its own. daisyUI 5 already ships `steps`, `dropdown` and `collapse` classes, and the app uses daisyUI, so these count as existing components. This needs confirming before implementation.
- Once implementation starts, UI changes must be verified as described in `docs/agents/verification.md`: run the headless target, check the htmx swaps and SSE regions, and record the session in the ticket.
