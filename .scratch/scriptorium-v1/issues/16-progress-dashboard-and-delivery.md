# 16 — Progress dashboard and delivery

**What to build:** The application is fundamentally a progress dashboard over long-running jobs — this is the reason the UI is an embedded server rather than native bindings. This ticket makes that real.

While Dictionary Building or a translation runs, the user watches live progress: Chunks completed out of the total, carried by server-sent events, so they can judge whether to wait or come back later. The accumulated cost of a Book's translation is visible, so they can judge whether the Model they chose is worth it. A running translation can be stopped, freeing the machine without corrupting the work done so far. The number of failed Chunks is shown with an action to retry only those.

When it finishes, a button opens the folder containing the finished translation, so it can be copied to a reading device. That goes through a one-function platform shim, so supporting another platform later is a matter of filling in that function.

**Blocked by:** 10 — Dictionary Building; 13 — The failure ladder.

**Status:** ready-for-agent

- [ ] Server-sent events carry progress for both Dictionary Building and translation
- [ ] Translation progress shows Chunks completed out of the total
- [ ] Accumulated cost for a Translation Target is displayed and updates as the run proceeds
- [ ] Stopping a running translation leaves `state.json` consistent and the book resumable
- [ ] The failed-Chunk count is shown with a retry action that re-requests only those Chunks
- [ ] An open-folder button reveals the output directory, routed through a single platform shim function
- [ ] Handlers stay thin adapters over the service layer; they carry no pipeline logic
- [ ] A dropped SSE connection reconnects and resyncs rather than freezing the display
