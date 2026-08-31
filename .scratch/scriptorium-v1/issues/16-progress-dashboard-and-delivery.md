# 16 — Progress dashboard and delivery

**What to build:** The application is fundamentally a progress dashboard over long-running jobs — this is the reason the UI is an embedded server rather than native bindings. This ticket makes that real.

While Dictionary Building or a translation runs, the user watches live progress: Chunks completed out of the total, carried by server-sent events, so they can judge whether to wait or come back later. The accumulated cost of a Book's translation is visible, so they can judge whether the Model they chose is worth it. A running translation can be stopped, freeing the machine without corrupting the persisted work done so far. The number of failed or incomplete Chunks is shown with an action to **Validate and Repair** them.

When every Chunk Translation is valid, Book Composition runs automatically and produces the finished translation. A separate **Compose translated book** action always remains available as a diagnostic gateway: it uses the currently persisted parts, falls back to original Text Nodes, never repairs or changes state, and writes a marked partial output when necessary. A button opens the folder containing the finished or partial translation, so it can be copied to a reading device. That goes through a one-function platform shim, so supporting another platform later is a matter of filling in that function.

**Blocked by:** 10 — Dictionary Building; 13 — The failure ladder.

**Status:** ready-for-agent

- [ ] Server-sent events carry progress for both Dictionary Building and translation
- [ ] Translation progress shows Chunks completed out of the total
- [ ] Accumulated cost for a Translation Target is displayed and updates as the run proceeds
- [ ] Stopping a running translation leaves persisted Chunk artifacts and `state.json` consistent and the book resumable
- [ ] **Validate and Repair** checks the persisted state and translated Chunk files, then re-requests only missing or malformed Chunks
- [ ] **Compose translated book** produces a best-effort output without calling the Agent or changing `state.json`, and warns when it is partial
- [ ] Automatic Book Composition runs only when every Chunk Translation is valid
- [ ] An open-folder button reveals the output directory, routed through a single platform shim function
- [ ] Handlers stay thin adapters over the service layer; they carry no pipeline logic
- [ ] A dropped SSE connection reconnects and resyncs rather than freezing the display
