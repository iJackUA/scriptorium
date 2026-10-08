Status: ready-for-agent
Blocked by: 01-unify-activity-registry.md

# In progress button and list

Parent: `.scratch/activity-log/spec.md`

## What to build

A header control labelled "In progress", placed next to the settings gear on every screen. It shows a badge with the number of running Activities when that number is above zero, and shows "Nothing running" when it is zero. Opening it lists every running Activity across all Series and Books, oldest first.

Each item reads "<verb>: <Book Label> → <target language name>", where the verb is "Building dictionary" for Dictionary Building, "Translating" for Translation, and "Repairing" for Validate and Repair. Book Label is the Book's title, or its Book Code when no title is set. Chunk Materialization, Book Composition and model-list fetches are not listed.

This ticket is read-only. Stop and the Activity Log come in later tickets.

## Acceptance criteria

- The header control appears on the Library and the Book page (story 1).
- The badge shows the running count, and is hidden when nothing runs (story 2).
- With nothing running, the list says "Nothing running" (story 3).
- Every running Activity across Series and Books appears (story 4).
- Dictionary Building, Translation and Validate and Repair each appear with their verb (stories 5, 6, 7, 9).
- Items show the Book Label and target language (story 8).
- Chunk Materialization and Book Composition do not appear (story 10).
- Items are in start order (story 11).
- The list and the Book page agree on what is running, because both read the registry (story 40).

## Tests

- With no Activity running, the header shows "Nothing running" and no badge.
- With two Activities of different Targets, the badge shows 2 and the list shows both labels in start order.
- A Book without a title shows its Book Code as its label.
