# hris-outsource backend — developer tasks
#
# Codegen scope: sqlc only. The OpenAPI specs in docs/api/ remain the contract
# (the frontend's Orval generates the web client from them), but the Go server
# is hand-written against that contract — no server-side codegen. (oapi-codegen
# can't parse the 3.1 specs; rather than fight it, handlers are authored by hand
# and kept faithful by contract tests.)
#   - sqlc : db/migrations + db/queries -> internal/repository/sqlc
# CI runs `make gen` and fails if `git diff` is dirty (query drift).

SHELL := /bin/bash
.DEFAULT_GOAL := help

DB_URL ?= postgres://hris:hris@localhost:5432/hris?sslmode=disable

## tools: install codegen + lint toolchain (pinned)
.PHONY: tools
tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/riverqueue/river/cmd/river@latest
	go install mvdan.cc/gofumpt@latest
	@echo "Install golangci-lint via your package manager or the official script."

## gen: regenerate codegen output (sqlc). Run before `go build`.
.PHONY: gen
gen: gen-sqlc

## gen-sqlc: type-safe query code from db/migrations + db/queries
.PHONY: gen-sqlc
gen-sqlc:
	sqlc generate

## migrate-up / migrate-down: apply/rollback schema (goose, embedded in cmd/migrate)
.PHONY: migrate-up
migrate-up:
	go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down:
	go run ./cmd/migrate down

## river-migrate: create River's job-queue tables
.PHONY: river-migrate
river-migrate:
	river migrate-up --database-url "$(DB_URL)"

## build / run / worker
.PHONY: build
build:
	go build ./...

.PHONY: run
run:
	go run ./cmd/api

.PHONY: worker
worker:
	go run ./cmd/worker

## test: unit + integration (integration uses testcontainers; needs Docker)
.PHONY: test
test:
	go test ./...

## lint / fmt
.PHONY: lint
lint:
	golangci-lint run

.PHONY: fmt
fmt:
	gofumpt -w .

## verify: the CI gate — sqlc must be clean, then lint + test
.PHONY: verify
verify: gen
	@git diff --exit-code -- internal/repository/sqlc \
		|| (echo "ERROR: generated sqlc code is stale. Run 'make gen' and commit." && exit 1)
	$(MAKE) lint test

## help: list targets
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -E 's/## //'
