# Deferred items — Phase 06 (E4 Schedule & Shifts)

## Out-of-scope discoveries (NOT fixed — pre-existing, unrelated to 06-01 changes)

- **`gofmt -l` flags pre-existing files `internal/domain/identity.go` and
  `internal/domain/people.go`.** These predate Phase 6 and are untouched by plan
  06-01 (only `scheduling.go` was added, and it is gofmt-clean). Per the executor
  scope-boundary rule, pre-existing formatting drift in unrelated files is not
  fixed here. Surface to a maintenance pass or whoever next edits those files.
  - Discovered: 2026-06-04, during 06-01 final verification.
  - Not touched by: 06-01 (migrations + sqlc + domain/scheduling.go only).
