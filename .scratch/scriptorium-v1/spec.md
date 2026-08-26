# Scriptorium v1

Status: ready-for-agent

## Problem Statement

I read books that have never been translated into the language I want to read them in, and machine translation services render them unreadably — they translate sentence by sentence with no memory, so a character's name is spelled three different ways across a novel, the register drifts between formal and casual at random, and the result reads like a manual rather than a book.

Paying a human translator is not an option for personal reading. Doing it myself with a chat window means copying thousands of paragraphs back and forth by hand, losing the book's structure in the process, and having no way to resume when I stop halfway. And when I read a *series*, the problem compounds: even if I get one book right, the next book has no memory of the choices I made in the last one, so the same character acquires a new name in volume two.

I also don't want a per-book bill. I already pay for an AI subscription and want the translation to draw on that rather than metered API charges.

## Solution

A desktop application that translates whole ebooks with an AI Agent, preserving the original file's structure exactly and holding terminology consistent across an entire Series.

I organise my library into Series. Each Series declares its source language and owns a Dictionary — a small, hand-curated set of Terms (character names, places, coined words) that must always translate the same way. Books inherit that Dictionary, so volume three of a series calls the protagonist what volume one called them.

For each Book I upload the original file once. Before translating, I run Dictionary Building: the Agent reads the whole Book and proposes the Terms that recur often enough to matter, with translations chosen as a consistent set. I review that list in a plain text editor, fix what's wrong, and only then start the translation.

Translation runs Chunk by Chunk, each Chunk seeing the previous one's translation so tone and phrasing carry forward. I watch progress, and when it finishes I get a file in the same format and structure as the original, with the original untouched, and a button to open the folder it's in. If something fails partway, I can close the app and resume later without losing work.

## User Stories

### Library and organisation

1. As a translator, I want to create a Series with a name and a source language, so that all the books by one author share settings and terminology.
2. As a translator, I want to add a Book to a Series, so that it inherits that Series' Dictionary and source language.
3. As a translator, I want to add a standalone Book without first thinking about Series, so that translating a one-off novel doesn't require ceremony.
4. As a translator, I want a standalone Book to still live in a real Series behind the scenes, so that I can add a sequel to it later without migrating anything.
5. As a translator, I want a Series containing exactly one Book to display without a group header, so that my library list isn't cluttered with meaningless groupings.
6. As a translator, I want to assign each Book a short Book Code by hand, so that I control the folder name it is stored under and can find it on disk.
7. As a translator, I want to be told when a Book Code is already used in that Series or contains characters that aren't allowed, so that I fix it before anything is written to disk.
8. As a translator, I want to see all my Books in a list with their Status, so that I can tell at a glance what needs my attention.
9. As a translator, I want to open a Book to see its details, so that I can review its metadata and act on it.
10. As a translator, I want my whole library to live in one workspace folder of plain files, so that I can back it up, sync it, inspect it, or edit it by hand without the app.
11. As a translator, I want to choose where the workspace folder lives on first launch, so that I can put it on the drive I want.

### Source files and metadata

12. As a translator, I want to upload the original ebook file to a Book, so that there is something to translate.
13. As a translator, I want `.txt` and `.fb2` files to be accepted, so that I can work with the formats I actually have.
14. As a translator, I want the Source File stored unmodified, so that the original is never at risk no matter what the translation does.
15. As a translator, I want title, author, and language read directly out of an fb2 file, so that metadata is exact and instant rather than guessed.
16. As a translator, I want the Agent to infer title and author from the opening pages of a `.txt` file, so that plain-text books still get a useful details page.
17. As a translator, I want to correct any metadata by hand, so that a bad inference or a missing field doesn't stay wrong.
18. As a translator, I want to be warned that replacing a Source File discards all existing translation work for that Book, so that I don't destroy hours of work by accident.

### Languages and translation targets

19. As a translator, I want to pick a target language for a Book from a list, so that I can start a translation.
20. As a translator, I want to translate one Book into several languages independently, so that I can produce a Ukrainian version and a German version from a single upload.
21. As a translator, I want each Translation Target to carry its own Status and progress, so that a finished Ukrainian translation isn't confused with an unstarted German one.
22. As a translator, I want the source language set once on the Series, so that I'm not re-picking it for every book by the same author.
23. As a translator, I want to delete a Translation Target, so that I can abandon a language without touching the Book or its other translations.

### Dictionary building

24. As a translator, I want to start Dictionary Building for a Translation Target, so that terminology is decided before any prose is translated.
25. As a translator, I want to watch Dictionary Building progress through the Book, so that I know it's working and roughly how long is left.
26. As a translator, I want only recurring Terms proposed, so that the Dictionary stays small enough to be useful rather than listing every proper noun in the book.
27. As a translator, I want proposed translations chosen with the whole Term list in view, so that related names are rendered consistently with each other instead of one Chunk at a time.
28. As a translator, I want to review the proposed Dictionary as a plain text file, so that I can edit it in whatever editor I like without fighting a form.
29. As a translator, I want to add, edit, and remove Terms freely, so that I have the final say over how names are rendered.
30. As a translator, I want a Book Dictionary that overrides the Series Dictionary, so that a term unique to one volume doesn't pollute the terminology of the whole series.
31. As a translator, I want to promote a Term from a Book Dictionary to the Series Dictionary, so that once I decide a name is series-wide, later books inherit it.
32. As a translator, I want to be told when the Dictionary has grown large enough to bloat every request, so that I know to prune it.
33. As a translator, I want to re-run Dictionary Building without losing my hand edits, so that I can extend a Dictionary after changing the source.

### Translation

34. As a translator, I want to start the translation once the Dictionary is ready, so that the terminology I approved is actually applied.
35. As a translator, I want the Dictionary included in every request, so that Terms are respected throughout, not just where they were first found.
36. As a translator, I want each Chunk translated with the previous Chunk's translation in view, so that register, tense, and forms of address stay consistent across boundaries.
37. As a translator, I want Chunks kept within Chapter boundaries, so that unrelated scenes aren't blended into one request.
38. As a translator, I want live progress showing Chunks completed out of the total, so that I can judge whether to wait or come back later.
39. As a translator, I want to see the accumulated cost of a Book's translation, so that I can judge whether the Model I chose is worth it.
40. As a translator, I want to pick the Agent and Model per Book, so that I can spend more on a book I care about and less on one I don't.
41. As a translator, I want a default Agent and Model in settings, so that I'm not choosing every time.
42. As a translator, I want to stop a running translation, so that I can free my machine without corrupting the work done so far.

### Failure, resume, and output

43. As a translator, I want a Chunk that comes back malformed retried automatically, so that a one-off glitch doesn't stall the book.
44. As a translator, I want a persistently malformed Chunk translated one Text Node at a time, so that the book completes even when a batch request keeps failing.
45. As a translator, I want a Chunk that fails every attempt to keep its original text and be reported, so that I get a readable book with visible gaps rather than a silently scrambled one.
46. As a translator, I want to see how many Chunks failed, so that I can decide whether to retry them or accept the result.
47. As a translator, I want to retry only the failed Chunks, so that I don't pay to re-translate the book.
48. As a translator, I want to close the app mid-translation and resume where I left off, so that a long book doesn't require one uninterrupted sitting.
49. As a translator, I want editing the Dictionary to invalidate only the work it affects, so that a small terminology fix doesn't force a full re-translation.
50. As a translator, I want the output file in the same format and structure as the original, so that it opens in my e-reader exactly as the original did.
51. As a translator, I want to open the folder containing the finished translation, so that I can copy it to my reading device.
52. As a translator, I want the translated file named after the Book Code and target language, so that multiple translations of one Book don't collide.

### Prompts

53. As a translator, I want a sensible built-in translation prompt, so that the app works well without configuration.
54. As a translator, I want to override the prompt per Series, so that I can tell the Agent that a given author reads formal and archaic.
55. As a translator, I want a broken prompt override to fail loudly, so that I don't discover the Dictionary was silently dropped after translating a whole book.

## Implementation Decisions

### Domain and vocabulary

All terms carry the meanings defined in `CONTEXT.md`. In particular: **Status and progress belong to a Translation Target**, not to a Book, because one Book may be translated into several languages at different times. Every Book belongs to exactly one Series; a standalone Book is a Series of one, and "hidden" is a rendering choice only — there is no second concept in the model.

### Architectural constraints already decided

Three ADRs govern this work and must be respected rather than re-litigated:

- **ADR-0001** — translation runs by spawning the `claude` CLI as a subprocess, not by calling an HTTP API. The justification is subscription economics.
- **ADR-0002** — the Agent is never shown markup. Text is extracted, translated, and spliced back into the original document tree.
- **ADR-0003** — Chunks are translated serially, because each depends on the previous Chunk's accepted translation.

### Workspace schema

State lives in plain files under a single workspace root chosen on first launch. Book identity is per-Book; translation artefacts are per target language:

```
<workspace>/
  workspace.toml                      default Agent and Model, language list
  <series-code>/
    series.toml                       name, source language, defaults
    prompt.md                         optional prompt override
    dictionaries/<pair>.tsv           Series Dictionary, per language pair
    books/<book-code>/
      book.toml                       metadata, Agent and Model override
      source.<ext>                    Source File, never modified
      translations/<pair>/
        dictionary.tsv                Book Dictionary
        state.json                    progress, resumable
        out/<book-code>.<target>.<ext>
```

Two format families, chosen deliberately: **TOML** for anything a human edits, because comments survive; **JSON** for machine-written progress, which is rewritten constantly and not meant for human eyes. Dictionaries are **TSV** (`original`, `translation`, `note`) because they are hand-edited *and* machine-generated, and a line-oriented format is far more robust to model output than YAML's indentation rules.

`state.json` is the only file that must be crash-safe: write to a temporary file and rename, so an interrupted write can never leave a book unresumable.

### Agent invocation

The Agent is driven as a plain LLM call, not as a coding agent. The invocation must:

- **Replace** the default system prompt rather than appending to it, and disable all tools, so there is no tool loop and no conversational framing around the result.
- Request JSON output and read the result text, the error flag, and the reported cost.
- **Pass an explicit settings override.** The user's global CLI configuration silently overrides the model flag; without this, Scriptorium translates entire books on whatever model the user has pinned globally, at that model's price. This is not optional and must be covered by a test.

Agent and Model are configurable per Book, defaulting from workspace settings. In v1 the Agent field has exactly one legal value — the field exists so the file format stays stable when a second Agent is added, not because it does anything yet.

Two Models are used by task: a cheap one for mechanical work (metadata inference, Term extraction) and a stronger one for translation, where quality is the entire point.

### Format handling

A format handler parses a Source File into an ordered list of Text Nodes plus whatever structure is needed to reassemble it, and accepts translated Text Nodes back to produce an output file. This boundary is what makes later formats cheap: adding epub or docx means writing a handler, not touching the pipeline.

- **fb2** — Text Nodes are paragraph-level text content. Chapters come from the file's own section structure. Metadata is parsed directly from the file's description block; no Agent involvement.
- **txt** — Text Nodes are blank-line-separated paragraphs. Chapters are detected with a built-in set of regex heuristics, overridable per Book. **Detection failure is not an error**: with no chapters found the whole file is one Chapter and the pipeline runs unchanged, losing only continuity hygiene. Metadata is inferred by the Agent from the opening pages.

### Chunking and continuity

Chunks are built to a word budget of roughly 2,000 words, never splitting a Text Node and never crossing a Chapter. This is driven by the ~3.5s fixed cost per Agent invocation: per-paragraph requests would mean hours of process startup for a novel, while whole-chapter requests produce outputs long enough for models to drift and summarise.

Each request carries a **Continuity Window**: the tail of the previous Chunk's source text and its accepted translation, explicitly marked as reference material not to be translated. Continuity resets at Chapter boundaries.

### The numbered-node protocol

This is the load-bearing safety mechanism of the whole design. Because translations are spliced back **by position**, a Chunk that returns fewer Text Nodes than it was sent shifts every later translation into the wrong slot — producing a structurally valid file that is quietly scrambled from that point onward, which a reader would not notice until reading it.

Therefore each Text Node is sent with an explicit index marker, and a response is **rejected** unless the markers come back with the same count, the same indices, none missing and none invented. The validator additionally rejects: known conversational prefixes (stripped before validation), output identical to the input (indicating a refusal or pass-through), and truncated trailing nodes.

On rejection, a fixed ladder:

1. Retry once with a stricter instruction appended.
2. Fall back to one request per Text Node, where misalignment is impossible.
3. Mark the Chunk failed, **leave the source text in that slot**, and surface the failure count in the UI. A gap is never shipped silently.

### Dictionary building

Two passes, split by concern:

1. **Extraction** — per Chunk, cheap Model, candidate Terms only and no translations. Results are merged across Chunks and **counted**; a Term appearing throughout the book is a major character, one appearing once is a walk-on. Only Terms clearing an occurrence threshold survive.
2. **Translation of Terms** — a single request with the entire surviving Term list visible at once, so that names are chosen as a coherent set.

Translating Terms during extraction is explicitly rejected: it produces several spellings of the same name, which is precisely the inconsistency the Dictionary exists to prevent.

The full Dictionary is injected into every translation request without filtering, which is sound only while it stays small. Budget roughly 100 Terms; the UI warns past that.

### Resume

`state.json` records per Chunk: index, status, a hash of the source text, and cost. On restart only `pending` and `failed` Chunks are re-requested. Hashing means a Dictionary edit or a source change invalidates the affected Chunks rather than all of them, and re-running is always safe.

### Prompts

The translation prompt is a template with documented slots for source and target language, the Dictionary, the Continuity Window, and the numbered-node instructions. A built-in default ships with the app; an optional per-Series override replaces it. Overrides are **validated on load** — a template missing a required slot fails loudly, because the alternative is silently translating a book without its Dictionary.

### UI

Wails v2 wrapping an embedded Go HTTP server serving `html/template` plus htmx, with Tailwind and daisyUI for components. Server-sent events carry progress for Dictionary Building and translation — this is the reason for choosing an embedded server over native bindings, since the application is fundamentally a progress dashboard over long-running jobs.

The server binds to loopback on a random port and checks request origin. Handlers stay thin adapters over the service layer.

Opening the output folder goes through a one-function platform shim, so cross-platform support later is a matter of filling in that function.

## Testing Decisions

### What makes a good test here

Tests assert **external behaviour**: the file produced, the state recorded, and the requests the Agent received. They do not assert on internal function calls, struct shapes, or the wording of prompts beyond the presence of the things that must be there (Dictionary Terms, the Continuity Window, node markers).

There is **one substitution seam: the Agent interface.** It is the only slow, costly, nondeterministic dependency, and faking it is what makes the whole suite fast and repeatable. The fake is both stub and spy — scripted responses going in, a recorded transcript of requests coming out. That dual role is what keeps the seam count at one: chunking, continuity, and Dictionary injection are invisible in the output file but plainly visible in the recorded requests.

Tests are driven through the **service layer** over a temporary workspace directory, and observe exactly three things: the output file, `state.json`, and the recorded Agent requests. The HTTP handlers are deliberately not tested — they are thin adapters, and driving server-sent events through a test server is high cost for low value. Keeping them thin is a design obligation that follows from this decision.

### What gets tested through the seam

- **Splice fidelity** — a real fb2 in, a translated fb2 out, structure asserted identical. This is ADR-0002's central claim.
- **Chunking** — recorded requests respect the word budget and never cross a Chapter.
- **Continuity** — request *N* contains the translation returned for request *N-1*.
- **Serial execution** — recorded requests are strictly ordered and never overlap, as required by ADR-0003.
- **Dictionary injection** — every translation request contains the merged Dictionary, with Book Dictionary entries overriding Series ones.
- **The failure ladder** — script the fake to drop a node index and assert the retry, then the per-node fallback, then the Chunk marked failed with source text left in place. Each rung gets its own test.
- **Validator rejections** — conversational prefixes, unchanged output, truncated responses.
- **Resume** — script a mid-book failure, re-run, assert only unfinished Chunks are re-requested.
- **Hash invalidation** — edit the Dictionary, re-run, assert the right Chunks are re-requested and no others.
- **Dictionary building** — assert extraction requests carry no translation instruction, that merging counts occurrences and drops one-off Terms, and that Term translation happens in a single request with the whole list present.
- **Model pinning** — assert the settings override is present in every invocation, since its absence is silent and expensive.
- **Chapter detection failure** — a `.txt` with no detectable chapters still translates end to end.

### What gets tested directly, with no seam

The format handlers are pure functions and need no fake. The important one is a **round-trip property**: parse a Source File to Text Nodes, splice back with an identity translation, and assert the output is equivalent to the input. ADR-0002 claims structure is preserved *by construction*; this is the test that proves it, and it should run over a corpus of real fb2 files including awkward ones — nested markup, footnotes, empty paragraphs, poetry blocks.

The chunker and the validator are likewise pure and cheap to test directly for their edge cases, though their integration is covered through the seam.

### Prior art

None — this is a greenfield repository. These tests establish the prior art. Later work should follow the same shape: drive the service layer, fake only the Agent, assert on files and recorded requests.

## Out of Scope

- **epub and docx.** Deferred by design. The format handler boundary exists so they can be added without touching the pipeline.
- **Any Agent other than `claude`.** The configuration field exists and accepts one value; a second Agent is future work.
- **Direct LLM API calls.** Rejected in ADR-0001 on cost grounds. The Agent interface keeps an adapter cheap if the reasoning ever changes.
- **Parallel translation.** Rejected in ADR-0003. The upgrade path — parallel across Chapters, serial within — is available because the chunker already emits Chapter boundaries.
- **Reassembling chapters with an Agent.** Explicitly rejected: translations fill slots in a document that was never taken apart, so there is nothing to join.
- **Filtering the Dictionary per Chunk.** Rejected while Dictionaries stay small. Revisit if they grow past a few hundred Terms.
- **A relational database.** Plain files throughout.
- **Editing translated text in the app.** Output is a file; edit it in a real editor.
- **Multi-user, sync, sharing, or accounts.** Single user, single machine.
- **Windows and Linux builds.** macOS is the target; the platform shim is the only place that should need attention later.
- **Polished visual design and animation.** daisyUI defaults are sufficient.

## Further Notes

Prerequisite: the Wails CLI is not installed on the development machine. Go 1.24.2, `claude`, and `codex` are present.

**Build the format handler round-trip first.** If parse-and-splice cannot reproduce an fb2 file byte-for-byte under an identity translation, ADR-0002 does not hold and several downstream decisions move. It is the cheapest way to invalidate the riskiest assumption, and it needs no Agent at all.

Two behaviours are worth calling out as easy to get subtly wrong, both verified during design:

- The CLI's system-prompt flag must **replace**, not append. Appending leaves the coding-agent persona in place and the tool loop with it.
- The model flag is **silently overridden** by the user's global configuration. This fails invisibly and expensively — the book translates fine, on the wrong model, at the wrong price.

The occurrence threshold for Dictionary Terms is a guess and should be tunable while the first few real books go through. Too low and the Dictionary bloats past the size that makes unfiltered injection sound; too high and the recurring names that matter get dropped.
