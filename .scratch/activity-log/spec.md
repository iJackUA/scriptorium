Status: ready-for-agent

# Activity log: an In progress list, per-Target Agent Transcripts, and guarded Activities

Source decision: `docs/adr/0007-per-target-agent-transcripts.md`. Vocabulary follows `GLOSSARY.md`, including Activity, Agent Transcript, Activity Log, Book Label and Failure Cause. Depends on the Status guard and Failure Cause work in `.scratch/success-flow-ui/`; see Further Notes.

## Problem Statement

A translator starts Dictionary Building or Translation and then loses sight of it. The work runs in the background, and the only sign of it is progress on one Book page. If the translator goes to another Book or another screen, nothing shows everything that is running. If they want to know what the Agent was asked, or what it returned, the only record is one shared file for the whole Workspace. That file mixes every Book's Agent calls and records no Book or Target, so the calls of one Target cannot be picked out. Deleting a Target leaves its calls behind in that file.

Two more problems come from the same gap. If the app closes during a run, the Target stays Analyzing or Translating with nothing running behind it, and the translator cannot tell that from a live run. And nothing stops two runs from starting on the same Target at once, so both can write the same state and Chunk files.

## Solution

A button labelled "In progress" sits next to the settings gear on every screen. It shows a count of running Activities. Opening it lists every running Activity across all Series and Targets, with a Stop button on each. Clicking an item opens its Activity Log: each Agent call the Activity has made, with its summary, updating live as calls start and return. The prompt and the replies are collapsed.

Each Translation Target keeps its own Agent Transcript, so the record of what the Agent was asked and returned belongs to the Target and goes with it when it is deleted. Agent calls that belong to no Target, such as fetching the model list, go to a single Workspace-level transcript.

Finished Activities are reached from a "View log" entry in the Target's step menu, which lists the Target's Activities and opens the Activity Log for any one of them.

The server refuses a second Activity on a Target that already has one running, and refuses to delete a Target while one of its Activities is running. On the next launch after a crash, a Target left Analyzing returns to New, and a Target left Translating becomes Failed with the cause Interrupted, with Resume translation as its next action.

## User Stories

1. As a translator, I want an "In progress" button next to the settings gear on every screen, so that I can check running work from wherever I am.
2. As a translator, I want the button to show a count of running Activities, so that I can see at a glance whether anything is running.
3. As a translator, I want the button to say "Nothing running" when nothing is running, so that an empty list is not mistaken for a broken one.
4. As a translator, I want the list to show every running Activity across all Series and Books, so that I do not have to open each Book to find out what is running.
5. As a translator, I want Dictionary Building to appear in the list, so that I can see when a dictionary is being built.
6. As a translator, I want Translation to appear in the list, so that I can see when a Book is being translated.
7. As a translator, I want Validate and Repair to appear in the list while it runs, so that a repair running in the background is visible.
8. As a translator, I want each item to name the Book by its Book Label and the target language, so that I can tell two Books apart at a glance.
9. As a translator, I want each item to name what it is doing in words, such as "Building dictionary", "Translating" or "Repairing", so that I know the kind of work without opening it.
10. As a translator, I want Chunk Materialization and Book Composition to stay out of the list, so that short operations do not clutter it.
11. As a translator, I want the list to show Activities in the order they started, so that the oldest running work is at the top.
12. As a translator, I want a Stop button on each item in the list, so that I can stop a runaway Activity without going to its Book.
13. As a translator, I want Stop in the list to take effect immediately, with no confirmation, so that stopping a run is never slowed by a dialog.
14. As a translator, I want Stop to keep the Chunks that are already finished, so that stopping never loses completed work.
15. As a translator, I want an item to disappear from the list once its Activity has ended, so that the list shows only what is running now.
16. As a translator, I want to click an item and open its Activity Log, so that I can see what the Agent was asked and what it returned.
17. As a translator, I want the Activity Log to update live while the Activity runs, so that I can follow it without refreshing.
18. As a translator, I want each Agent call in the Activity Log to show when it started, how long it took, its Model and its cost, so that I can judge a call without opening it.
19. As a translator, I want each Agent call to show any error it returned, so that I can see why a call failed.
20. As a translator, I want each call's prompt and replies collapsed by default, so that the log stays readable and does not show whole Chunks of Book prose at once.
21. As a translator, I want to expand a call to see its full prompt, raw reply and parsed reply, so that I can see exactly what was exchanged.
22. As a translator, I want a call that started but never returned, after a crash, to show as Interrupted, so that I can tell it did not finish.
23. As a translator, I want a "View log" entry in a Target's step menu, so that I can open the log of an Activity that has already finished.
24. As a translator, I want the "View log" entry to list every Activity the Target has run, so that I can pick the one I want.
25. As a translator, I want each Target's Agent Transcript to hold only that Target's Agent calls, so that I am not shown another Book's prompts.
26. As a translator, I want deleting a Target to remove its Agent Transcript, so that the Book prose in its prompts does not stay on disk after I delete the Target.
27. As a translator, I want the Delete confirmation to list the Agent Transcript among what will be lost, so that I know before I delete.
28. As a translator, I want Delete refused while the Target has an Activity running, so that a deleted Target is never written to by a run still in progress.
29. As a translator, I want the Delete button hidden while a Target has an Activity running, with the reason shown, so that I know why I cannot delete it yet.
30. As a translator, I want the server to refuse Delete while an Activity runs, even if the page was stale, so that the rule holds regardless of what I can see.
31. As a translator, I want the server to refuse a second Activity on a Target that already has one running, so that two runs never write the same files.
32. As a translator, I want a refused second start to say that the Target already has an Activity running, and to point me to it in the list, so that I know what to do.
33. As a translator, I want two Targets, of the same Book or of different Books, to run at the same time, so that I can work on several Books at once.
34. As a translator, I want Agent calls that belong to no Target, such as fetching the model list, to be recorded in the Workspace-level transcript, so that they are not lost.
35. As a translator, I want the existing Workspace transcript kept as it is, so that the Agent calls recorded before this change are still available.
36. As a translator, I want the app to start a Target that was Analyzing when it closed as New, so that I can start its dictionary again without deleting the Target.
37. As a translator, I want a Target that was Translating when the app closed to become Failed with the cause Interrupted, so that I know a run was lost rather than stopped by me.
38. As a translator, I want an Interrupted Target to offer Resume translation as its next action, so that I can continue from the Chunks that are already done.
39. As a translator, I want Resume translation after an interruption to request only the Chunks that are not finished, so that completed work is kept.
40. As a translator, I want the header "In progress" button and the Book page to agree on what is running, so that one never shows an Activity the other does not.
41. As a translator, I want the Activity Log to be reachable from the keyboard and to be readable at phone width, so that the app stays usable on a narrow window.
42. As a translator, I want existing Stop buttons on the Book page to keep working, so that stopping from the page still works as before.
43. As a translator, I want completed actions to keep showing toasts, so that feedback is the same as before.
44. As a maintainer, I want each Target's Agent Transcript kept in the Target's own folder, so that the transcript is removed or moved together with the Target's other files.
45. As a maintainer, I want the transcript files to stay private to the user, so that Book prose in prompts is not readable by other accounts on the machine.
46. As a maintainer, I want the Activity list to come from one registry rather than two, so that a new kind of Activity can be added in one place.
47. As a maintainer, I want the same guard to refuse a second Activity on a Target whether it is started from the Book page or from anywhere else, so that there is only one rule to keep correct.
48. As a maintainer, I want Stop with no running Activity to change nothing and say so, so that a stale Stop control cannot leave a Target in the wrong Status.
49. As a maintainer, I want the Interrupted cause written to the Target's state, so that the recovery is recorded and survives another restart.
50. As a maintainer, I want the feature verified in a headless browser session as well as by tests, so that the list, the modal and the live updates are checked in a real page.

## Implementation Decisions

**Modules**

- **Activity registry (new, deep).** One registry of running Activities across all Targets, replacing the separate registries for Dictionary Building and Translation. Each Activity records its kind (Dictionary Building, Translation or Validate and Repair), the Target it belongs to, the Book Label and target language used for its display label, when it started, and a way to stop it. Starting an Activity on a Target that already has one running is refused as one atomic check, so no two starts can both succeed. The registry has no I/O. Activity kinds are the only things that enter it: Chunk Materialization, Book Composition and model-list fetches do not.
- **Display labels.** Each kind has a fixed verb form: "Building dictionary", "Translating" and "Repairing". The item reads "<verb>: <Book Label> → <target language name>". The target language name is the Language name for the Target's Language Tag.
- **In progress list.** A header control next to the settings gear, on every screen. It shows the running count as a badge when the count is above zero, and "Nothing running" when empty. Its items are in start order, oldest first. Each item has a Stop button. Clicking an item opens the Activity Log for it.
- **Stop.** Stop from the list uses the same behaviour as the Stop buttons on the Book page: it cancels the run, keeps finished Chunks, and sets the Target's Status to the value that a stopped run leaves. No confirmation. Stop with no running Activity for the Target changes nothing and returns a message saying so. This replaces today's no-op that leaves the Target in Translating.
- **Activity Log.** A modal that shows the Activity's Agent calls in start order. Each call shows its start time, duration, Model, cost, and error if any. Each call's prompt and replies are collapsed by default and expand on demand. The modal updates live through the same event stream the progress regions already use, and a call appears when it starts and changes when it returns. A call that has an invoked record but no returned record, after a restart, shows as Interrupted.
- **Agent Transcript per Translation Target.** A JSON Lines file in the Target's folder, written with owner-only permissions. It keeps the existing invoked and returned records, and joins them by call in the Activity Log. Each record carries the id of the Activity that made the call, so the Activity Log and "View log" can group calls by Activity. The record gains no Series, Book or Target fields: the file's location identifies the Target.
- **Workspace-level transcript.** The existing Workspace file keeps its name and location. It receives only Agent calls that have no Target: fetching the model list and checking Agent availability. It has no screen in the app.
- **Delete Translation Target.** Deleting removes the Target's Agent Transcript along with the rest of its folder. The confirmation lists the Agent Transcript among what will be lost. Delete is refused while any Activity runs for the Target. The Delete button is hidden with the reason shown while an Activity runs, and the server refuses the request either way.
- **View log.** An entry in the Target's step menus that lists the Target's Activities, finished and running, and opens the Activity Log of the one chosen.
- **Startup recovery.** On launch, a Target whose Status is Analyzing becomes New. A Target whose Status is Translating becomes Failed with the Failure Cause Interrupted. Recovery runs once, at startup, before the first request is served. It writes the state back to the Target. Validate and Repair is not started automatically.
- **Failure Cause.** Interrupted is a new value of the optional Failure Cause that the success-flow work introduces: Dictionary Building, Chunks, Stopped by the user, and Interrupted. Its next action is Resume translation, which is Validate and Repair under its clearer label.

**Schema changes**

- The Target's stored state gains the Interrupted value for its Failure Cause. Existing Targets without a cause are unchanged.

**Behaviour**

- Two Activities on one Target are never allowed to run at once. The refusal names the running Activity and links to it in the list.
- Two Activities on different Targets, of the same Book or of different Books, may run at once.
- The list, the header badge and the Book page's Stop buttons all read from the same registry, so they cannot disagree.
- Delete is refused while the Target has a running Activity, whether it was started from the Book page or the list.

## Testing Decisions

**What makes a good test here.** Tests describe what a translator sees, what a maintainer can rely on in the files, and what the server accepts or refuses. They do not assert on markup structure or on internal fields beyond what a person or a file can observe. A test is good when it still passes after the page is restyled with the same components.

**Seam.** The existing one: the UI's HTTP handler tree, driven through an in-process test server. The Agent is the existing fake, which already records the requests it receives and can be given a gate that holds a call open until the test releases it. The Agent Logger is already an interface, so the transcript tests use the existing one rather than a new seam. No new seam is needed.

**What to test**

- The header control shows "Nothing running" when no Activity runs, and shows the count when Activities run.
- The list shows each running Activity with its verb, Book Label and target language, in start order.
- Two Activities of the same Target cannot start: the second start is refused and names the running one.
- Two Activities of different Targets can run at once.
- Stop from the list stops the run, keeps finished Chunks, and sets the Status the Book page would show after a stop.
- Stop with no running Activity changes nothing and says so.
- The Activity Log shows each Agent call with its start time, duration, Model, cost and error. Prompts and replies are collapsed in the rendered page and present in the response.
- The Activity Log updates while a call is held open, and shows the return when the gate is released.
- A call with an invoked record but no returned record shows as Interrupted after a restart.
- The "View log" entry lists finished and running Activities of one Target, and does not list another Target's.
- A Target's Agent Transcript holds only its own calls, and the model-list call goes to the Workspace-level transcript.
- Delete is refused while an Activity runs, and the page hides the button with the reason shown.
- Delete lists the Agent Transcript in its confirmation, and removes it when the Target is deleted.
- Startup recovery: a Target left Analyzing becomes New; a Target left Translating becomes Failed with the cause Interrupted, offers Resume translation, and resuming requests only the unfinished Chunks.
- The Workspace transcript keeps the records written before this change.
- Existing behaviour is preserved: the upload, metadata, Series and Target creation, pickers, toast and translation tests still pass.

**Prior art.** The UI tests in `internal/ui` drive the handler tree with a test server and name each test after the behaviour it checks. The translation service tests use the fake Agent to script responses and read back the recorded requests. The Agent package tests cover the transcript format. Follow all three patterns.

**Verification.** Each UI change is also checked in the headless session described in `docs/agents/verification.md`. Capture screenshots of: an empty header control, a running list with two Activities, the Activity Log while a call is open, a finished Activity opened from "View log", and an Interrupted Target. Record the session in the ticket's Comments.

## Out of Scope

- A cap or rotation on transcript size. ADR-0007 keeps transcripts uncapped; a cap can be added later without changing the file format.
- A screen for the Workspace-level transcript.
- Splitting the existing Workspace transcript by Target. Its records cannot be attributed.
- Queuing a second Activity on the same Target. The server refuses it.
- Parallel Chunk translation within one Target. ADR-0003 keeps translation serial.
- Chunk Materialization, Book Composition and model-list fetches in the In progress list.
- Automatic Validate and Repair on startup. ADR-0005 keeps repair explicit.
- Cancelling a single Chunk. Stop cancels the whole Activity.
- Desktop notifications outside the app.
- Any visual redesign beyond the existing components and styling.

## Further Notes

- **Shared work with `success-flow-ui`.** The Status guard (issue 02), Failure Cause (issue 03), re-run confirmations (issue 04) and Delete confirmation (issue 09) in `.scratch/success-flow-ui/` cover the same code. Build those first, or build this spec's registry, guard and Failure Cause together with them. Do not build the same guard twice.
- **Existing defects this spec removes.** Stop Translation does nothing when no run exists, so a stopped or stale Target stays Translating. Delete is allowed during a run, which lets a run write into a deleted folder. Two starts on one Target are not refused, and the check and the write are not under one lock.
- **Glossary.** Activity, Agent Transcript, Activity Log, Book Label and Failure Cause are defined in `GLOSSARY.md`. "Validate and Repair" and the action labels in the success-flow spec are still undefined in the glossary; that is a separate item and is not needed for this spec.
- **Decision record.** `docs/adr/0007-per-target-agent-transcripts.md` records the transcript layout and the In progress list.
- **Order.** The spec's registry comes first, because the guards and the list both depend on it. Startup recovery and Failure Cause come next, then the list and the Activity Log, then the Delete changes.
