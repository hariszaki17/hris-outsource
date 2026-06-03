module github.com/hariszaki17/hris-outsource/backend

go 1.23.0

// Direct dependencies. Run `go mod tidy` after first checkout to populate
// go.sum and indirect requires. Versions are recent-stable as of 2026-06;
// tidy will reconcile them.
require (
	github.com/caarlos0/env/v11 v11.2.2
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-chi/cors v1.2.1
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.1
	github.com/joho/godotenv v1.5.1
	github.com/oklog/ulid/v2 v2.1.0
	github.com/pressly/goose/v3 v3.22.1
	github.com/prometheus/client_golang v1.20.5
	github.com/riverqueue/river v0.14.2
	github.com/riverqueue/river/riverdriver/riverpgxv5 v0.14.2
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.56.0
	go.opentelemetry.io/otel v1.31.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.31.0
	go.opentelemetry.io/otel/exporters/prometheus v0.53.0
	go.opentelemetry.io/otel/metric v1.31.0 // indirect
	go.opentelemetry.io/otel/sdk v1.31.0
	go.opentelemetry.io/otel/sdk/metric v1.31.0
	go.opentelemetry.io/otel/trace v1.31.0 // indirect
	golang.org/x/crypto v0.31.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.60.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/riverqueue/river/riverdriver v0.14.2 // indirect
	github.com/riverqueue/river/rivershared v0.14.2 // indirect
	github.com/riverqueue/river/rivertype v0.14.2 // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.31.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/goleak v1.3.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241007155032-5fefd90f89a9 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241007155032-5fefd90f89a9 // indirect
	google.golang.org/grpc v1.67.1 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
