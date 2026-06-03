// Package idempotency implements the Idempotency-Key contract (CONVENTIONS §13):
// for flagged create/action/bulk endpoints, the server caches the response by
// key for 24h. Re-submitting the same key + same body replays the stored
// response; the same key with a DIFFERENT body is a 409 IDEMPOTENCY_KEY_REUSED.
//
// Storage is Postgres (idempotency_keys table) — no Redis. The key is scoped to
// the authenticated user so keys can't collide across callers.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

type Middleware struct {
	pool *db.Pool
}

func New(pool *db.Pool) *Middleware { return &Middleware{pool: pool} }

// Handler wraps endpoints that REQUIRE idempotency. Endpoints that merely accept
// the header can use the same wrapper; absence of the header is allowed for the
// optional ones (here we treat a missing key as pass-through).
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next.ServeHTTP(w, r) // optional for some endpoints; required ones validate upstream
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			httpx.WriteError(w, r, apperr.Invalid(nil).WithCause(err))
			return
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))

		p, _ := auth.PrincipalFrom(r.Context())
		scopedKey := p.UserID + ":" + key
		reqHash := hashBody(body)

		// Replay or conflict against a previously stored response.
		status, stored, found, reuse, err := m.lookup(r.Context(), scopedKey, reqHash)
		if err != nil {
			httpx.WriteError(w, r, apperr.Internal(err))
			return
		}
		if reuse {
			httpx.WriteError(w, r, apperr.Conflict("IDEMPOTENCY_KEY_REUSED"))
			return
		}
		if found {
			replay(w, status, stored)
			return
		}

		// First time: capture the response, persist it on 2xx.
		rec := &capture{ResponseWriter: w, status: http.StatusOK, buf: &bytes.Buffer{}}
		next.ServeHTTP(rec, r)

		if rec.status >= 200 && rec.status < 300 {
			if err := m.store(r.Context(), scopedKey, reqHash, rec.status, rec.buf.Bytes()); err != nil {
				// Storing failed; the action already happened. Log via the error
				// path is overkill — best-effort cache is acceptable here.
				_ = err
			}
		}
	})
}

// lookup returns (status, body, found, reuse, err). reuse=true means the key
// exists with a different request body.
func (m *Middleware) lookup(ctx context.Context, key, reqHash string) (int, []byte, bool, bool, error) {
	const q = `SELECT request_hash, response_status, response_body
	           FROM idempotency_keys
	           WHERE key = $1 AND expires_at > now()`
	var storedHash string
	var status int
	var body []byte
	err := m.pool.QueryRow(ctx, q, key).Scan(&storedHash, &status, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, false, false, nil
	}
	if err != nil {
		return 0, nil, false, false, err
	}
	if storedHash != reqHash {
		return 0, nil, false, true, nil
	}
	return status, body, true, false, nil
}

func (m *Middleware) store(ctx context.Context, key, reqHash string, status int, body []byte) error {
	const q = `
		INSERT INTO idempotency_keys (key, request_hash, response_status, response_body, created_at, expires_at)
		VALUES ($1, $2, $3, $4, now(), now() + interval '24 hours')
		ON CONFLICT (key) DO NOTHING`
	_, err := m.pool.Exec(ctx, q, key, reqHash, status, body)
	return err
}

func replay(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Idempotent-Replayed", "true")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func hashBody(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// capture buffers the response so a successful one can be persisted for replay.
type capture struct {
	http.ResponseWriter
	status int
	buf    *bytes.Buffer
}

func (c *capture) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *capture) Write(b []byte) (int, error) {
	c.buf.Write(b)
	return c.ResponseWriter.Write(b)
}
