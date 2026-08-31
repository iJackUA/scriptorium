# Scriptorium

A desktop application for translating ebooks with an AI agent, one book at a time, with terminology held consistent across a whole series.

## Language

### Library structure

**Workspace**:
The single folder, chosen by the user on first launch, holding the whole library as plain files. Its root carries a `workspace.toml` of defaults and the Translation Target languages available throughout the Workspace.
_Avoid_: library folder, vault, database, storage, project folder

**Series**:
A named group of Books with one immutable Source Language, a Dictionary, and translation settings. Every Book belongs to exactly one Series; a standalone book is a Series containing one Book.
_Avoid_: collection, group, project, folder

**Book**:
A single work, with one Source File, belonging to exactly one Series.
_Avoid_: title, volume, ebook, document

**Book Code**:
A short identifier assigned by hand, unique within its Series, that names the Book everywhere it is stored or referenced.
_Avoid_: slug, id, shortname, key

**Translation Target**:
The pairing of a Book with one enabled Target Language distinct from its Series' Source Language. Progress, Dictionary, Status, and output all belong to a Translation Target rather than to the Book, so one Book can be translated into several languages independently.
_Avoid_: job, run, output, task

**Status**:
The stage a Translation Target has reached: `New`, `Analyzing`, `Dictionary Ready`, `Translating`, `Translated`, or `Failed`.
_Avoid_: state, phase, progress

**Language Tag**:
The canonical ISO 639-1 identifier for a Language, such as `en`, `uk`, or `de`. It is stored wherever a Source Language or Target Language is recorded; people see its human-readable Language name alongside it.
_Avoid_: free-text language, locale

**Source Language**:
The immutable Language Tag chosen when a Series is created. Every Book in that Series uses it as its original language.
_Avoid_: book language, input language

**Target Language**:
A Language Tag enabled in the Workspace for creating Translation Targets. It must differ from the Series' Source Language.
_Avoid_: output language, destination locale

**Language Pair**:
The ordered Source Language and Target Language of a Translation Target, such as `en` to `uk`.
_Avoid_: locale pair

### Source material

**Source File**:
The original ebook exactly as supplied. Never modified; replacing it invalidates all existing work for the Book.
_Avoid_: input, upload, original, master

**Text Node**:
One addressable unit of prose in the Source File — usually a paragraph — extracted for translation and spliced back into the position it came from.
_Avoid_: paragraph, segment, line, string

**Chapter**:
A structural division of the Book, read from the source's own markup where the format provides it and inferred from text patterns where it does not.
_Avoid_: section, part, division

**Chunk**:
A run of consecutive Text Nodes sent to the Agent as a single request, bounded by a word budget and never crossing a Chapter boundary. The unit of progress, failure, and resumption.
_Avoid_: batch, block, window, piece, segment

**Chunk Materialization**:
The Book-level, persisted representation of a Source File's ordered Text Nodes and Chapter boundaries, ready to be translated and reused for resumption.
_Avoid_: parse cache, intermediate output, source copy

**Chunk Translation**:
The accepted translation of one Chunk, retaining the global Text Node indices needed to place it back into the Source File.
_Avoid_: translated fragment, output piece

**Book Composition**:
The operation that places accepted Chunk Translations into the original document tree to produce a translated Book.
_Avoid_: concatenation, reassembly

**Continuity Window**:
The tail of the preceding Chunk's source text and its accepted translation, supplied as reference so register and terminology carry across Chunk boundaries. Never itself translated.
_Avoid_: context, history, overlap, memory

### Terminology control

**Dictionary**:
A curated set of Terms constraining how particular words are translated. Deliberately small — significant recurring vocabulary only, not general vocabulary.
_Avoid_: glossary, termbase, lexicon, wordlist

**Series Dictionary**:
The Dictionary shared by every Book in a Series for one target language. Curated by hand.
_Avoid_: global dictionary, shared dictionary

**Book Dictionary**:
A Dictionary holding Terms specific to one Translation Target. Overrides the Series Dictionary where they disagree. Entries are promoted to the Series Dictionary by hand when they prove series-wide.
_Avoid_: local dictionary, overrides

**Term**:
A single original-to-translation mapping — a character name, a place, a coined word — that must render identically everywhere it appears.
_Avoid_: entry, word, mapping, pair

**Dictionary Building**:
The stage that reads the whole Book to propose Terms for review, before any translation begins.
_Avoid_: analysis, scan, pre-pass, extraction

### Translation

**Agent**:
The external command-line AI tool invoked as a subprocess to perform translation and analysis.
_Avoid_: API, backend, provider, LLM

**Model**:
The specific model an Agent is asked to use for one kind of task, named in that Agent's own vocabulary. A Model is meaningful only paired with the Agent that accepts it; there is no namespace shared across Agents. Chosen separately for mechanical work and for translation.
_Avoid_: engine, AI
