# Coding-agent CLI as the translation engine

Scriptorium translates by spawning the `claude` CLI as a subprocess rather than calling an LLM HTTP API. A novel is 150k+ words and needs two passes, so per-token billing would cost real money per book; driving a CLI that is already authenticated against a flat-rate subscription makes the marginal cost of a book effectively zero. This is the whole reason for the choice, and it is the reason to revisit it if the subscription ever goes away.

## Consequences

A coding agent is otherwise the wrong shape for bulk text transformation, so the invocation deliberately strips it back to a plain LLM call: `--system-prompt` (which *replaces* the agent persona rather than appending to it), `--tools` with no arguments to disable the tool loop, and `--output-format json` to read `.result` and `.total_cost_usd`. Verified: this yields `num_turns: 1` and bare text with no conversational framing.

Two costs follow. Process startup is ~1.5s on top of ~2s of API latency, which is why Chunks are sized in thousands of words rather than per paragraph — see ADR-0003. And `--model` is **silently overridden by the user's `~/.claude/settings.json`**; without passing `--settings` with inline JSON, Scriptorium would translate entire books on whatever model the user happens to have pinned globally, at that model's price. Passing `--settings` is not optional.

## Considered options

A direct LLM API call would be faster, natively parallel, and would give structured output and token accounting without any of the above workarounds. It was rejected on cost alone. The Agent is kept behind a narrow interface so an API adapter remains a small piece of work.

The Agent Client Protocol was considered as the way to drive Agents, and rejected. ACP standardises what Scriptorium deliberately suppresses — tool calls, permissions, file system and terminal access — and does not standardise the three things this ADR actually needs: replacing the system prompt, disabling the tool loop, and pinning the Model against the user's global configuration. Model selection exists only as an agent-declared session config option, which the spec states must not be required for correctness, so the per-Agent quirk handling above would remain either way. Decisively, neither target Agent speaks ACP natively: both `claude` and `codex` reach it through separate Node bridge processes, which would put a runtime dependency and a third-party package on the critical path of a 150k-word run inside a self-contained desktop app. Revisit if a target Agent ships native ACP, or if the Agent is ever asked to *do* something rather than return text — the one live candidate being an interactive Dictionary Building pass, where ACP's elicitation would let the Agent ask about an ambiguous Term mid-book instead of proposing the whole list for batch review.
