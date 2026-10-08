Status: ready-for-agent
Blocked by: 01-unify-activity-registry.md

# Refuse a second Activity on the same Target

Parent: `.scratch/activity-log/spec.md`

## What to build

A Translation Target may have at most one running Activity. Checking that the Target has none and registering the new Activity is one atomic step, so two requests cannot both start. The refusal says which Activity is running for that Target, by kind and display label, and points to the In progress list.

Two Targets, of the same Book or of different Books, may still run at once.

## Acceptance criteria

- A second Dictionary Building, Translation or Validate and Repair request on a Target that has an Activity running is refused, and the Target's files are not changed (stories 31, 32).
- The refusal names the running Activity (story 32).
- Two Targets of the same Book can run at once (story 33).
- Two Targets of different Books can run at once (story 33).
- Two simultaneous start requests on one Target result in one running Activity and one refusal.

## Tests

- A start on a Target with a running Activity is refused, and the response names it.
- A start on a second Target of the same Book succeeds while the first runs.
- Concurrent starts on one Target: exactly one succeeds.
