Status: ready-for-agent
Blocked by: 07-activity-log-modal.md

# "View log" in the step menu

Parent: `.scratch/activity-log/spec.md`

## What to build

Each Translation Target's step menus gain a "View log" entry. It lists every Activity the Target has run, running and finished, and opens the Activity Log of the one the user chooses. The list is built from the Target's Agent Transcript, grouped by Activity id, so finished Activities remain reachable after the app restarts.

## Acceptance criteria

- "View log" appears in the Target's step menus (story 23).
- It lists the Target's finished and running Activities, with kind and time (stories 23, 24).
- It lists only the Target's own Activities (story 25).
- Choosing one opens its Activity Log (story 23).
- After a restart, a finished Activity still appears in the list (story 23).

## Tests

- A Target with two finished Activities lists both, and a second Target's Activities do not appear.
- Choosing an entry renders that Activity's calls.
- The list is built from the transcript, so a Target with no Activities shows an empty list.
