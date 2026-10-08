Status: ready-for-agent
Blocked by: 03-per-target-agent-transcript.md, 05-in-progress-list.md

# Activity Log modal

Parent: `.scratch/activity-log/spec.md`

## What to build

Clicking an item in the In progress list opens the Activity Log: a modal listing that Activity's Agent calls in start order. The calls are read from the Target's Agent Transcript, grouped by Activity id.

Each call shows its start time, duration, Model, cost, and error if it has one. Each call's prompt and replies are collapsed by default and expand on demand. A call with an invoked record but no returned record, after a restart, shows as Interrupted.

This ticket shows the log as it stands. Live updates come in the next ticket.

## Acceptance criteria

- Clicking an item opens the modal for that Activity, and shows only its calls (story 16).
- Each call shows start time, duration, Model, cost and error (stories 18, 19).
- Prompts and replies are collapsed by default and expand on demand (stories 20, 21).
- A call started but not returned, after a restart, shows as Interrupted (story 22).
- The modal is usable at phone width, and can be closed from the keyboard (story 41).

## Tests

- The modal lists the calls of one Activity and not another's on the same Target.
- The rendered page contains the summary of each call, and the prompt only inside its collapsed element.
- A transcript with an invoked record and no returned record renders that call as Interrupted.
