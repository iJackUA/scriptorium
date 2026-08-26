# 08 — Agent interface, `claude` adapter, and the fake

**What to build:** One narrow Agent interface, a real adapter that drives the `claude` CLI as a subprocess, and the fake that stands in for it in every test.

Per ADR-0001 the Agent is driven as a plain LLM call, not as a coding agent. The invocation must replace the default system prompt rather than appending to it (appending leaves the coding-agent persona and the tool loop in place), disable all tools, request JSON output, and read the result text, the error flag, and the reported cost.

It must also pass an explicit settings override. The user's global CLI configuration silently overrides the model flag; without the override, Scriptorium translates entire books on whatever model the user has pinned globally, at that model's price. This fails invisibly and expensively, so it is covered by its own test.

The fake is both stub and spy — scripted responses going in, a recorded transcript of requests coming out. It is the suite's single substitution seam, and that dual role is what keeps the seam count at one: chunking, continuity, and Dictionary injection are invisible in the output file but plainly visible in the recorded requests.

In v1 the Agent field has exactly one legal value. The field exists so the file format stays stable when a second Agent is added.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] The interface exposes a single request/response call carrying prompt, Model, and returning result text, error flag, and cost
- [ ] The adapter replaces the system prompt rather than appending to it
- [ ] The adapter disables all tools, so there is no tool loop and no conversational framing
- [ ] The adapter requests JSON output and reads result, error flag, and reported cost
- [ ] Every invocation carries the explicit settings override that pins the Model, asserted by a test
- [ ] A non-zero exit or an error flag surfaces as an Agent error, not as an empty translation
- [ ] The fake replays scripted responses in order and records every request it received
- [ ] Recorded requests are inspectable enough to assert on Dictionary Terms, the Continuity Window, and node markers
