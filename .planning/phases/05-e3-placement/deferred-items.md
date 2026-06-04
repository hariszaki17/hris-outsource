# Deferred items — Phase 05 E3 Placement

## [05-02] golangci-lint config version mismatch (pre-existing, out-of-scope)
- `make verify` runs `golangci-lint run` which fails with:
  `can't load config: unsupported version of the configuration: ""`
- This is a tooling/version drift in the repo's golangci-lint config, NOT caused by
  the 05-02 placement changes. The plan's actual gates (`make gen`, `go build ./...`,
  `go vet ./...`, `gofmt -l`) all pass clean.
- Action: bump/repair `.golangci.yml` to the installed golangci-lint major version.
