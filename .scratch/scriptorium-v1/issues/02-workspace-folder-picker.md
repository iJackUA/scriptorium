# 02 — Workspace folder picker

**What to build:** On first launch the application asks where the workspace folder should live and lets the user pick any directory on any drive. The choice is remembered, so a relaunch opens straight into that workspace. The workspace root gets a `workspace.toml` holding the default Agent and Model and the list of target languages.

The whole library is plain files under this one root, so it can be backed up, synced, inspected, or hand-edited without the application.

**Blocked by:** 01 — Wails shell with a dummy library page.

**Status:** ready-for-agent

- [ ] First launch presents a native folder picker and accepts any writable directory
- [ ] The chosen path persists across restarts; a second launch goes straight to the library
- [ ] `workspace.toml` is created at the root with default Agent, default Models, and the target language list
- [ ] An existing workspace is opened rather than overwritten
- [ ] A workspace whose folder has since disappeared is reported clearly and the picker is offered again
- [ ] `workspace.toml` is TOML so hand-written comments survive a rewrite
