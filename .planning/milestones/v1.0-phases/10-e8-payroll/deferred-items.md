# Deferred Items — Phase 10 (E8 Payroll)

Out-of-scope discoveries logged during execution. NOT fixed (per the execute-plan
scope boundary — only auto-fix issues directly caused by the current plan's changes).

## Pre-existing gofmt drift (discovered 10-01, NOT introduced here)

`gofmt -l internal/` reports these PRE-EXISTING files as unformatted (Phase 1-4
identity/people slices). None are touched by plan 10-01; the conventions say
`gofmt -l` must be clean for `make verify`, so a future cleanup pass (or whichever
plan next edits these files) should `gofmt -w` them:

- internal/domain/identity.go
- internal/domain/people.go
- internal/handler/identity/dto.go
- internal/handler/identity/handler_test.go
- internal/handler/people/agreements_dto.go
- internal/handler/people/agreements_handler_test.go
- internal/handler/people/change_requests_dto.go
- internal/handler/people/employees_dto.go
- internal/handler/people/employees_handler_test.go
- internal/service/identity/service_test.go
- internal/service/people/agreements_service.go
- internal/service/people/change_requests_service.go
- internal/service/people/employees_service.go

All files CREATED by 10-01 (crypto, config, domain/payroll, db/queries/payroll)
are gofmt-clean. `go build ./...` and `go vet ./...` exit 0.
