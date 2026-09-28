# 17 — Agent and Model configuration, and prompt overrides

**What to build:** Two kinds of configuration, both defaulting sensibly so the application works with no setup.

**Agent and Model.** A default Agent lives in workspace settings, so the user is not choosing every time, and it can be overridden per Book — spending more on a book they care about and less on one they don't. Two Models are used by task: a cheap one for mechanical work (metadata inference, Term extraction) and a stronger one for translation, where quality is the entire point.

Model names belong to an Agent's own vocabulary, so the pair of Models is held **per Agent** rather than globally. This matters at the override: a Book that overrides the Agent but inherits Models from another Agent's namespace is an invalid pairing, and it must be rejected on load rather than discovered as a failed request halfway through a book.

**Prompts.** A sensible built-in translation prompt ships with the application. A per-Series override in `prompt.md` **replaces** it, so the user can tell the Agent that a given author reads formal and archaic. Overrides are validated on load: a template missing a required slot fails loudly, because the alternative is silently translating a whole book without its Dictionary and only discovering it afterwards.

**Blocked by:** 03 — Series and Books on disk; 08 — Agent interface, `claude` adapter, and the fake; 12 — Translation tracer bullet: happy path.

**Status:** done

- [x] Workspace settings hold a default Agent, and a mechanical Model and a translation Model for each known Agent
- [x] `book.toml` can override Agent and Model, and the override wins
- [x] The Agent field accepts any known Agent and rejects unknown values
- [x] A Book that overrides the Agent without overriding the Models is rejected on load, naming the Agent and the Model that don't belong together
- [x] Mechanical work uses the cheap Model and translation uses the strong one, asserted against the recorded requests
- [x] A `<series-code>/prompt.md` override replaces the built-in prompt rather than appending to it
- [x] Source and target language prompt slots receive both the English language name and canonical Language Tag
- [x] An override missing a required slot fails loudly on load, before any request is made
- [x] The error names which slot is missing
- [x] With no override present, the built-in default is used and nothing warns

## Comments

Models are held per Agent in `workspace.toml` under `[agents.<name>]` (the
default file writes `[agents.claude]`). A legacy top-level `[models]` table is
still read as the default Agent's, because the file is never rewritten. Any
Model left unset falls back to that Agent's own built-in default, so a trimmed
config still works. `book.toml` may set `agent` and `[models]`, and each field
wins over the workspace. An unknown Agent is rejected when `book.toml` or
`[agents.*]` is read.

Resolution lives in `workspace.Config.Settings`, reached through
`library.Book.AgentSettings`. A Book naming another Agent inherits only that
Agent's Models. If that Agent has none, the Book is rejected, and the error
names the Agent and the default Agent's Model it would otherwise have borrowed.
Every known Agent has built-in Models, so in v1, with `claude` the only Agent,
this rejection can't be reached. It is tested directly on `Config.Settings`
with a hypothetical second Agent. It runs when an action starts (metadata
inference, Dictionary Building, translation, repair), before any request is
made, not while the library list is read.

`<series-code>/prompt.md` replaces `DefaultPromptTemplate` outright.
`translation.LoadPromptTemplate` rejects an override missing any of the six
slots and names each missing slot. The Translator loads it before any request,
and the translate and repair actions check it first so the error shows on the
Book page. With no override, the built-in prompt is used silently.

Tests assert the Models on the fake's recorded requests: cheap for Dictionary
Building and metadata, the Book's override for translation. They also cover
override replacement, language labels, missing-slot failure with zero
requests, and the metadata writer keeping `[models]` intact.
