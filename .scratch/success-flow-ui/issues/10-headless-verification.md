Status: ready-for-agent
Blocked by: 01-book-progress-wizard.md, 02-status-guard.md, 03-failure-cause-recovery.md, 04-rerun-confirmations.md, 05-actions-into-step-menus.md, 06-book-composition-gating.md, 07-settings-and-details-sections.md, 08-library-next-action-rows.md, 09-delete-target-confirmation.md

# Verify the redesigned UI in the headless session

Parent: `.scratch/success-flow-ui/spec.md`

## What to build

Check the finished UI in a headless session, as `docs/agents/verification.md` describes. Run `make headless`, open the printed URL in a browser, and capture screenshots of each main state: New, Analyzing, Dictionary Ready, Translating, Translated, each Failure cause, and stopped. Check the htmx swaps and the SSE regions, and that the step list reads as a wizard at a glance.

Record the session and its findings in this ticket's Comments.

## Acceptance criteria

- Screenshots of New, Analyzing, Dictionary Ready, Translating, Translated, each Failure cause, and stopped are saved in `.verification/`, which is not versioned.
- The Book page and the Library rows match in every state (story 38).
- A stale page cannot start a refused action (story 28).
- Findings and the session are recorded in this ticket's Comments.
- `make check` passes before the change is committed (AGENTS.md).

## Notes

- This ticket does not change code. If a finding needs a fix, open a new ticket for it.
