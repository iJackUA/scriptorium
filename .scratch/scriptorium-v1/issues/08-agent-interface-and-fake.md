# 08 — Agent interface, `claude` adapter, and the fake

**What to build:** One narrow Agent interface, a real adapter that drives the `claude` CLI as a subprocess, and the fake that stands in for it in every test.

Per ADR-0001 the Agent is driven as a plain LLM call, not as a coding agent. The invocation must replace the default system prompt rather than appending to it (appending leaves the coding-agent persona and the tool loop in place), disable all tools, request JSON output, and read the result text, the error flag, and the reported cost.

It must also pass an explicit settings override. The user's global CLI configuration silently overrides the model flag; without the override, Scriptorium translates entire books on whatever model the user has pinned globally, at that model's price. This fails invisibly and expensively, so it is covered by its own test.

The fake is both stub and spy — scripted responses going in, a recorded transcript of requests coming out. It is the suite's single substitution seam, and that dual role is what keeps the seam count at one: chunking, continuity, and Dictionary injection are invisible in the output file but plainly visible in the recorded requests.

The Agent field names which Agent to drive. `claude` is the first and `codex` follows shortly, so the field takes a known set of values and rejects anything outside it. Building the second adapter is also the test of this interface: if the seam is as narrow as ADR-0001 claims, `codex` fits behind it unchanged.

**Blocked by:** None — can start immediately.

**Status:** done

- [x] The interface exposes a single request/response call carrying prompt, Model, and returning result text, error flag, and cost
- [x] The adapter replaces the system prompt rather than appending to it
- [x] The adapter disables all tools, so there is no tool loop and no conversational framing
- [x] The adapter requests JSON output and reads result, error flag, and reported cost
- [x] Every invocation carries the explicit settings override that pins the Model, asserted by a test
- [x] A non-zero exit or an error flag surfaces as an Agent error, not as an empty translation
- [x] The fake replays scripted responses in order and records every request it received
- [x] Recorded requests are inspectable enough to assert on Dictionary Terms, the Continuity Window, and node markers

## Comments

Implemented in `internal/agent`. The `Agent` interface has one `Call` method with
`Request` and `Response` types. The Claude adapter invokes the CLI in print mode,
replaces its system prompt, disables built-in and MCP tools, requests JSON, and
pins the requested Model both through `--model` and an inline `--settings`
override. Non-zero exits, malformed output, and Claude's error flag return an
`ErrCall`; parsed result and cost remain available for an error-flag response.

`Fake` replays responses in order and records a thread-safe request transcript;
the transcript retains complete prompts so later service tests can inspect
Dictionary Terms, Continuity Windows, and node markers. Workspace loading now
rejects unknown configured Agent names before the workspace is used.

Tests use a temporary helper executable to assert the exact subprocess argument
vector and cover response decoding, model pinning, tool disabling, error
propagation, fake replay, transcript capture, and unknown Agents. `make check`
passes (format check, vet, and the full Go test suite).
