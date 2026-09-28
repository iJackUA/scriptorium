# 18 — Editing prompts in Workspace settings and on the Book

**What to build:** The prompts sent for Dictionary Building and translation are built into the application. Let the user edit them in the interface at two levels, Workspace settings and a single Book, keeping the existing rule that an override **replaces** what it overrides rather than appending to it, and is validated on load.

No prompt file exists in a fresh Workspace. An editor shows the text of the prompt the level would otherwise use, and the file is written the first time the user saves an edit.

**Blocked by:** 17 — Agent and Model configuration, and prompt overrides.

**Status:** done

- [x] Three prompts can be edited: Dictionary Building Term extraction, Dictionary Building Term translation, and Translation
- [x] Workspace settings save to `prompts/<prompt>.md` at the Workspace root
- [x] A Book saves to `<series>/books/<book>/prompts/<prompt>.md`
- [x] The most specific file wins: Book, then the Series `prompt.md` (translation only, as before), then the Workspace, then the built-in prompt
- [x] Before a level has its own file, its editor shows the prompt it would inherit, which is the built-in text when nothing overrides it
- [x] Saving an inherited prompt unchanged writes nothing
- [x] A save missing a required slot is refused, names the slot, keeps the edit on screen, and writes nothing
- [x] Revert deletes the level's file after a confirmation, so the level inherits again
- [x] A broken Dictionary Building prompt stops Dictionary Building before the Status changes and before any request is made

## Comments

The Dictionary prompts are templates now. Term extraction requires `{{TEXT_NODES}}`. Term translation requires `{{SOURCE_LANGUAGE}}`, `{{TARGET_LANGUAGE}}` and `{{TERMS}}`, and still receives Language Tags, as it did before. The built-in templates produce exactly the prompts that were sent before this change.

The resolution order lives in `translation.PromptFiles` and `translation.LoadPrompt`. The editors in `internal/ui/prompts.go` read the same list, so what an editor shows as inherited is always what a run would use. The Series `prompt.md` still works, but it has no editor.

Verification: I ran `make headless` and drove it with `agent-browser`. In Workspace settings, the Prompts section showed all three editors badged Built-in, each holding the built-in text. I saved a Term extraction edit. The modal stayed open, the editor swapped to Customized and "Saved.", and `prompts/dictionary-extraction.md` was written. The Book page then showed that edit as inherited from `prompts/dictionary-extraction.md`. On the Book, a save without `{{TEXT_NODES}}` was refused, with the slot and file named. A valid save wrote the Book's file. Revert opened a confirmation dialog, then deleted the file and the empty `prompts/` folder, and the editor fell back to the Workspace prompt. Screenshots are in `.verification/`.
