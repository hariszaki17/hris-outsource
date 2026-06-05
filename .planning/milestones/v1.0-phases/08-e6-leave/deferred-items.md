# Phase 08 — Deferred Items

## [08-02] golangci-lint config version mismatch (pre-existing, out of scope)

`make verify` → `make lint` (`golangci-lint run`) fails with:

```
Error: can't load config: unsupported version of the configuration: ""
```

This is a pre-existing environment/config-version mismatch in `.golangci.yml`
(the installed golangci-lint binary requires a newer config schema). It is NOT
introduced by 08-02 and affects every package equally. The actual quality gate
for this plan was satisfied via `go build ./...`, `go vet ./...`, `gofmt -l`
(clean), the sqlc-not-stale check, and `go test ./...` (all green).

Action: a maintainer should migrate `.golangci.yml` to the supported schema
(see https://golangci-lint.run/docs/product/migration-guide) or pin the linter
version — tracked separately, not a code defect.
