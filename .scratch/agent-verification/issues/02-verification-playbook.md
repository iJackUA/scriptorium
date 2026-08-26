# 02 — The verification playbook, and the eyeball ticket 01 asked for

**What to build:** `docs/agents/verification.md` — what a coding agent reads when it has finished a UI ticket and needs to check its work — and then a first session run against it, proving the thing works.

The document is short by design, and its most important content is the division of labour. Because the interface is htmx, every interaction is an HTTP request returning an HTML fragment, so `httptest` over the handler tree answers most questions with no infrastructure at all. A browser is for the residue HTTP cannot see: whether an `hx-target` actually matches an element, whether a stream's updates land in the right region, and whether it looks right. An agent that reaches for a browser to check a fragment's contents has misread this.

It also states the rule that keeps the harness from rotting: **a ticket that renders anything is not done until a verification session has run and its findings are recorded in the ticket's `## Comments`.**

The ticket closes by paying off the debt that started this: `scriptorium-v1` 01 shipped a library page whose window was never confirmed visually. Run a session against it and record what was found.

**Blocked by:** 01 — The headless twin and its Makefile target.

**Status:** ready-for-agent

- [ ] `docs/agents/verification.md` exists, points at `make headless`, and repeats no command lines the Makefile already holds
- [ ] It states when to use `httptest` or `curl` and when to use `agent-browser`, and why the browser is the exception
- [ ] It gives a worked three-command `agent-browser` session — open, snapshot, screenshot — as the shape to copy
- [ ] It states the legibility contract: semantics first, `data-testid` as escape hatch on Status badges and progress regions
- [ ] It states that UI tickets are not done until a verification session is recorded in their comments
- [ ] `CLAUDE.md` points at it, alongside the existing skill pointers
- [ ] `.verification/` is gitignored
- [ ] A session has been run against the shipped library page — including one Book click, to prove the `hx-target` actually swaps — with findings and a screenshot recorded in `scriptorium-v1` 01's comments

## Comments
