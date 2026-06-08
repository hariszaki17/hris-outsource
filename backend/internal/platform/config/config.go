// Package config loads all runtime configuration from the environment
// (12-factor). In dev a .env file is loaded first via godotenv; in prod the
// environment is authoritative. No secrets are ever read from anywhere else.
package config

import (
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Env         string `env:"ENV" envDefault:"development"`
	ServiceName string `env:"SERVICE_NAME" envDefault:"hris-api"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`

	HTTP   HTTP
	DB     DB
	Auth   Auth
	Crypto Crypto
	OTel   OTel
	Rate   Rate
	Cron   Cron
}

// Cron configures the in-process single-binary cron jobs (E5 absence-sweep). A
// later phase may graduate these to River; for now they run inside the API binary.
type Cron struct {
	// AbsenceSweepEnabled toggles the absence-sweep cron (default on).
	AbsenceSweepEnabled bool `env:"ABSENCE_SWEEP_ENABLED" envDefault:"true"`
	// AbsenceSweepInterval is the tick period for the sweep.
	AbsenceSweepInterval time.Duration `env:"ABSENCE_SWEEP_INTERVAL" envDefault:"15m"`
	// AbsenceGrace is how long after a shift's end a scheduled, un-clocked-in shift
	// must remain before it is marked ABSENT (the cutoff is now - grace).
	AbsenceGrace time.Duration `env:"ABSENCE_GRACE" envDefault:"30m"`

	// LeaveExpirySweepEnabled toggles the E6 leave-expiry sweep (default on). It
	// releases dangling pending on lapsed grant-lots (F6.1).
	LeaveExpirySweepEnabled bool `env:"LEAVE_EXPIRY_SWEEP_ENABLED" envDefault:"true"`
	// LeaveExpiryInterval is the tick period for the leave-expiry sweep.
	LeaveExpiryInterval time.Duration `env:"LEAVE_EXPIRY_INTERVAL" envDefault:"1h"`
}

type HTTP struct {
	Addr           string        `env:"HTTP_ADDR" envDefault:":8080"`
	ReadTimeout    time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout   time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"30s"`
	AllowedOrigins []string      `env:"CORS_ALLOWED_ORIGINS" envSeparator:","`
}

type DB struct {
	URL      string `env:"DATABASE_URL,required"`
	MaxConns int32  `env:"DATABASE_MAX_CONNS" envDefault:"10"`
}

type Auth struct {
	// Ed25519 keys, base64 (std encoding) of the raw key bytes.
	JWTPrivateKey string        `env:"AUTH_JWT_PRIVATE_KEY"`
	JWTPublicKey  string        `env:"AUTH_JWT_PUBLIC_KEY"`
	AccessTTL     time.Duration `env:"AUTH_ACCESS_TTL" envDefault:"30m"`
	RefreshTTL    time.Duration `env:"AUTH_REFRESH_TTL" envDefault:"720h"`
	CookieDomain  string        `env:"AUTH_COOKIE_DOMAIN"`
	CookieSecure  bool          `env:"AUTH_COOKIE_SECURE" envDefault:"true"`
}

type Crypto struct {
	// PayrollKey is the AES-256 key (base64 std-encoded 32 raw bytes) for payslip
	// monetary encryption-at-rest (INV-2). Milestone-scoped env constant — NOT a
	// KMS. For the E2E harness / dev a deterministic base64 32-byte key is
	// supplied via env (10-04 wires it into .env.e2e / the seed env so the seed
	// encrypts with the SAME key the API decrypts with). Do NOT hardcode a
	// production key.
	PayrollKey string `env:"PAYROLL_ENCRYPTION_KEY"`
}

type OTel struct {
	OTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

type Rate struct {
	PerMinute int `env:"RATELIMIT_PER_MINUTE" envDefault:"600"`
	Burst     int `env:"RATELIMIT_BURST" envDefault:"60"`
}

// Load reads .env (best-effort) then parses the environment into Config.
func Load() (Config, error) {
	_ = godotenv.Load() // dev convenience; ignore if absent
	var c Config
	if err := env.Parse(&c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) IsProd() bool { return c.Env == "production" }
