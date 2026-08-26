## Running this project

`Makefile` is the single source of truth for every repeatable command — setup,
build, run, assets, checks. Run `make help` to see them. Never document or run a
repeated multi-part command from anywhere else: add a target and point at it, so
there is one place to change when the command changes. `make check` is what a
change must pass before it is committed.

## Agent skills

### Issue tracker

Issues live as markdown files under `.scratch/<feature>/` in this repo. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage roles, each label string equal to its name. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context — `CONTEXT.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
