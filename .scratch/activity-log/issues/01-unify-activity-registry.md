Status: ready-for-agent
Blocked by: none

# Unify the Activity registry

Parent: `.scratch/activity-log/spec.md`

## What to build

Replace the separate in-memory registries for Dictionary Building and Translation with one registry of running Activities, keyed by Translation Target. Each Activity records its kind (Dictionary Building, Translation, or Validate and Repair), its Target, the Book Label and target language used for its display, when it started, and how to stop it. Validate and Repair becomes a registered kind.

This is a preparatory change. Nothing the user sees changes, and the registry is the single place later tickets read from.

## Acceptance criteria

- The progress regions, the Stop buttons on the Book page and the Dictionary and Translation progress streams behave as they did before (story 42).
- Validate and Repair is registered as an Activity kind (story 46).
- Every existing test in `internal/ui` and `internal/translation` passes without changes to its assertions.
- The registry has no I/O and can be exercised without a server.

## Tests

- Existing UI tests cover the progress and Stop behaviour unchanged.
- A registry test starts, lists and stops Activities of two kinds on two Targets.
