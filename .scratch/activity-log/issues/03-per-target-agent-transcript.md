Status: ready-for-agent
Blocked by: none

# Per-Target Agent Transcript

Parent: `.scratch/activity-log/spec.md`

## What to build

Agent calls made for Dictionary Building, Translation and Validate and Repair are recorded in the Translation Target's own folder, in the same JSON Lines format as today, with owner-only permissions. Each record carries the id of the Activity that made the call, which is the one new field. Records carry no Series, Book or Target fields: the location identifies the Target.

Agent calls that belong to no Target, such as fetching the model list and checking Agent availability, are recorded in the existing Workspace-level transcript. Records already in that file are kept as they are.

## Acceptance criteria

- A Target's folder holds only that Target's Agent calls (story 25).
- Each record carries its Activity's id (story 25 and Q1).
- The Workspace transcript keeps every record written before this change (story 35).
- Model-list fetches and availability checks are recorded in the Workspace transcript (story 34).
- The transcript files are readable only by the owning user (story 45).
- The transcript lives in the Target's folder, so moving or removing that folder moves or removes it (story 44).

## Tests

- A translation run writes its calls to the Target's folder and to no other transcript.
- A model-list fetch writes to the Workspace transcript and to no Target's folder.
- Each record has an Activity id, and no record has Series, Book or Target fields.
- Prior art: the existing transcript tests in `internal/agent`.
