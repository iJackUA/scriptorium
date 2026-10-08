Status: ready-for-agent
Blocked by: 04-refuse-delete-during-activity.md, 06-stop-from-list.md, 08-live-activity-log.md, 09-view-log-step-menu.md, 10-startup-recovery.md

# Headless verification of the Activity log

Parent: `.scratch/activity-log/spec.md`

## What to build

Check the Activity log in a real browser, as `docs/agents/verification.md` requires. Run the headless twin, and capture screenshots of: the empty In progress control; the list with two running Activities; an open Activity Log with a call in progress; a finished Activity opened from "View log"; and an Interrupted Target. Record the session in this ticket's Comments.

## Acceptance criteria

- Screenshots of the five states above are captured and attached to the Comments (story 50).
- The live update, the Stop from the list, and the View log entry each work in the browser.
- Any defect found is either fixed or recorded as its own ticket.

## Tests

- This ticket adds no automated tests. It records the browser session.
