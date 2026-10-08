# Per-Target Agent Transcripts and an In-progress list of Activities

**Status:** proposed

Every Agent call was appended to one Workspace file, `logs/agent-transcript.jsonl`. Its records carry no Series, Book, or Target, so a person cannot see what one Target did, and Delete leaves its calls behind. The app also has no place that shows work in progress. This decision moves each Translation Target's Agent calls into its own transcript, keeps one Workspace transcript for calls that belong to no Target, and shows running work in a global "In progress" list.

## Decision

- **Agent Transcript per Translation Target.** Each Target keeps its own transcript in its folder (`books/<book>/translations/<src>-to-<tgt>/`). Delete removes it with the Target, and the Delete confirmation lists it.
- **Workspace-level transcript.** `logs/agent-transcript.jsonl` stays where it is. It receives only Agent calls with no Target, such as fetching the model list. The existing records are kept and not split.
- **Activity Log.** The "In progress" list (a button next to the gear icon) shows every running Activity: Dictionary Building, Translation, and Validate and Repair. Each item has a Stop button and opens a modal that updates live, showing each Agent call with a summary. The prompt and replies are collapsed. Finished Activities are reached from a "View log" entry in the Target's step menu.
- **Guards.** The server refuses a second Activity on a Target that already has one running. It also refuses Delete while the Target has an Activity running, and hides the Delete button with the reason shown.
- **Crash recovery.** On the next launch, a Target left Analyzing returns to New. A Target left Translating becomes Failed with the cause Interrupted, and its next action is Resume translation.

## Considered options

- **One shared file, with Series, Book, and Target added to each record.** This keeps one file, but Delete cannot remove one Target's calls without rewriting the shared file, and two Targets write into it at once.
- **One file per Book.** Two Targets of the same Book would interleave their calls.

## Consequences

- Transcripts are not split retroactively. The Workspace transcript keeps the mixed history from before this change.
- The file location now determines which Target a call belongs to, so the Agent Transcript does not need Target fields.
- Transcripts are not capped. Full prompts include surrounding text, so a large Book's transcript can grow; a cap can be added later without changing the format.
