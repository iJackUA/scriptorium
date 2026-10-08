Status: ready-for-agent
Blocked by: 05-in-progress-list.md

# Stop from the In progress list

Parent: `.scratch/activity-log/spec.md`

## What to build

Each item in the In progress list has a Stop button. Stop acts immediately, with no confirmation, and does the same as the Stop buttons on the Book page: it cancels the Activity, keeps the Chunks already finished, and leaves the Target in the Status the Book page's Stop produces today.

Stop with no running Activity for the Target changes nothing and returns a message saying so. This replaces today's behaviour, where Stop Translation with no run leaves the Target in Translating.

## Acceptance criteria

- Each item has a Stop button, and pressing it stops that Activity without a confirmation (stories 12, 13).
- Stop keeps every Chunk already finished (story 14).
- The Target's Status after Stop matches the Book page's Stop (story 42).
- Stop with no running Activity changes nothing and says so (story 48).
- Stopping one Activity leaves other Activities on other Targets running.

## Tests

- Stop from the list cancels a run held open by the fake Agent, and the Chunks finished before it are kept.
- Stop with no run for the Target leaves its Status and files unchanged, and returns the message.
- Stop from the Book page still works (existing test).
