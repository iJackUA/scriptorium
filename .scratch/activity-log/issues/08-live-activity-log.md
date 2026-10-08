Status: ready-for-agent
Blocked by: 07-activity-log-modal.md

# Live Activity Log

Parent: `.scratch/activity-log/spec.md`

## What to build

While an Activity runs, its Activity Log updates without a reload. A call appears when it starts and changes when it returns. The updates come through the same event stream the progress regions use. When the Activity ends, the stream stops and the modal stays readable with its final calls.

## Acceptance criteria

- A call appears in an open Activity Log as soon as it starts (story 17).
- The call's row changes when it returns, with its duration, cost and error (story 17).
- When the Activity ends, the stream stops and the log stays on screen, unchanged (story 17).
- Opening the log of a finished Activity shows its calls without opening a stream (story 24).

## Tests

- With a call held open by the fake Agent, the log shows the call before release, and the return after release.
- The stream closes when the Activity ends.
