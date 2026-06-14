package httpx

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
)

// rateLimited is the shared 429 error (CONVENTIONS §11 RATE_LIMITED).
var rateLimited = apperr.Error{Code: "RATE_LIMITED", HTTPStatus: http.StatusTooManyRequests}

// RateLimiter is a per-key token-bucket limiter (CONVENTIONS §19: 600/min
// sustained, 60/s burst).
//
// SCALE NOTE: this is IN-MEMORY and therefore PER-INSTANCE. Behind a load
// balancer the effective limit is (instances × limit) and is approximate. When
// we scale horizontally, swap this for a shared store (Postgres counter or
// Redis) behind the same interface. Single source of truth for the rule lives
// here so that swap is local.
type RateLimiter struct {
	perMinute int
	burst     int
	mu        sync.Mutex
	buckets   map[string]*bucket
	// keyFn extracts the limiter key (authenticated subject, else client IP).
	// Injected by the server so this package doesn't depend on auth (no cycle).
	keyFn func(*http.Request) string
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewRateLimiter(perMinute, burst int, keyFn func(*http.Request) string) *RateLimiter {
	return &RateLimiter{
		perMinute: perMinute,
		burst:     burst,
		buckets:   make(map[string]*bucket),
		keyFn:     keyFn,
	}
}

// Middleware enforces the limit and sets the X-RateLimit-* headers.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	refillPerSec := float64(rl.perMinute) / 60.0
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rl.keyFn(r)
		allowed, remaining := rl.take(key, refillPerSec)

		h := w.Header()
		h.Set("X-RateLimit-Limit", strconv.Itoa(rl.perMinute))
		h.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		h.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))

		if !allowed {
			w.Header().Set("Retry-After", "1")
			WriteError(w, r, &rateLimited)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) take(key string, refillPerSec float64) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(rl.burst), last: now}
		rl.buckets[key] = b
	}
	// Refill since last seen, capped at burst.
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * refillPerSec
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.last = now

	if b.tokens < 1 {
		return false, 0
	}
	b.tokens--
	return true, int(b.tokens)
}

// ClientIP returns a best-effort client IP for unauthenticated rate-limit keys.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := indexByte(xff, ','); i >= 0 {
			return xff[:i]
		}
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
