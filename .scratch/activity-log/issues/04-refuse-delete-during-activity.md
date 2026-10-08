Status: ready-for-agent
Blocked by: 01-unify-activity-registry.md, 03-per-target-agent-transcript.md

# Refuse Delete while an Activity runs

Parent: `.scratch/activity-log/spec.md`

## What to build

Delete Translation Target is refused while the Target has an Activity running. The server refuses the request. The Delete button is hidden while an Activity runs, and the page shows the reason in its place. The Delete confirmation lists the Target's Agent Transcript among what will be lost.

## Acceptance criteria

- A Delete request during a run is refused, and the Target's folder is unchanged (stories 28, 30).
- The Delete button is hidden while an Activity runs, with the reason shown (story 29).
- The Delete confirmation lists the Agent Transcript (story 27).
- After a run ends, Delete works as before and removes the Target's folder, including its transcript (story 26).

## Tests

- Delete during a run is refused, and the folder still exists afterwards.
- Delete after a run succeeds, and the transcript file is gone with the folder.
- The confirmation names the Agent Transcript.
