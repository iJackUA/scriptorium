# 17 — Agent and Model configuration, and prompt overrides

**What to build:** Two kinds of configuration, both defaulting sensibly so the application works with no setup.

**Agent and Model.** A default Agent lives in workspace settings, so the user is not choosing every time, and it can be overridden per Book — spending more on a book they care about and less on one they don't. Two Models are used by task: a cheap one for mechanical work (metadata inference, Term extraction) and a stronger one for translation, where quality is the entire point.

Model names belong to an Agent's own vocabulary, so the pair of Models is held **per Agent** rather than globally. This matters at the override: a Book that overrides the Agent but inherits Models from another Agent's namespace is an invalid pairing, and it must be rejected on load rather than discovered as a failed request halfway through a book.

**Prompts.** A sensible built-in translation prompt ships with the application. A per-Series override in `prompt.md` **replaces** it, so the user can tell the Agent that a given author reads formal and archaic. Overrides are validated on load: a template missing a required slot fails loudly, because the alternative is silently translating a whole book without its Dictionary and only discovering it afterwards.

**Blocked by:** 03 — Series and Books on disk; 08 — Agent interface, `claude` adapter, and the fake; 12 — Translation tracer bullet: happy path.

**Status:** ready-for-agent

- [ ] Workspace settings hold a default Agent, and a mechanical Model and a translation Model for each known Agent
- [ ] `book.toml` can override Agent and Model, and the override wins
- [ ] The Agent field accepts any known Agent and rejects unknown values
- [ ] A Book that overrides the Agent without overriding the Models is rejected on load, naming the Agent and the Model that don't belong together
- [ ] Mechanical work uses the cheap Model and translation uses the strong one, asserted against the recorded requests
- [ ] A `<series-code>/prompt.md` override replaces the built-in prompt rather than appending to it
- [ ] An override missing a required slot fails loudly on load, before any request is made
- [ ] The error names which slot is missing
- [ ] With no override present, the built-in default is used and nothing warns
