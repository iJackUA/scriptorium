Status: ready-for-agent
Blocked by: .scratch/success-flow-ui/issues/03-failure-cause-recovery.md

# Startup recovery after a crash

Parent: `.scratch/activity-log/spec.md`

## What to build

On launch, before the first request is served, a Translation Target still marked Analyzing becomes New, and a Target still marked Translating becomes Failed with the cause Interrupted. Both are written to the Target's state. The Interrupted Target offers Resume translation, which is Validate and Repair under its clearer label, and it re-requests only the Chunks that are not finished.

Validate and Repair is not started automatically.

This ticket uses the Failure Cause field built in the `success-flow-ui` work, and adds Interrupted as its value.

## Acceptance criteria

- A Target marked Analyzing when the app starts becomes New (story 36).
- A Target marked Translating when the app starts becomes Failed with the cause Interrupted (story 37).
- The Interrupted Target offers Resume translation as its next action (story 38).
- Resume translation requests only the unfinished Chunks (story 39).
- Recovery writes to the Target's state, so it survives another restart (story 49).
- Nothing is started automatically (ADR-0005).

## Tests

- A state file marked Analyzing is New after startup.
- A state file marked Translating is Failed with cause Interrupted after startup, and the page offers Resume translation.
- Resuming an Interrupted Target requests only the Chunks that are not finished.
