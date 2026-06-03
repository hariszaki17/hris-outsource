// Package audit writes the immutable audit-log rows required on EVERY write
// (CONVENTIONS §16.1): actor, action, entity, before/after state, request_id.
// It runs inside the caller's transaction (pgx.Tx) so the audit row commits
// atomically with the data change — no write is ever unaudited. Bulk operations
// call Record once per affected entity.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

// Action is the verb recorded (CREATE/UPDATE/DELETE or a domain action verb).
type Action string

const (
	ActionCreate Action = "CREATE"
	ActionUpdate Action = "UPDATE"
	ActionDelete Action = "DELETE"
)

// Entry is one audit record. Before/After are arbitrary structs serialized to
// JSONB; pass nil where not applicable (Before on create, After on delete).
type Entry struct {
	Action     Action
	EntityType string // e.g. "placement", "leave_request"
	EntityID   string // SWP-… id of the affected resource
	Before     any
	After      any
}

// Record inserts an audit row using the actor + request_id from context. The
// SWP-AL id is allocated from the same swp_next_id sequence inside this tx.
func Record(ctx context.Context, tx pgx.Tx, e Entry) error {
	p, _ := auth.PrincipalFrom(ctx)
	reqID := httpx.RequestID(ctx)

	before, err := marshalNullable(e.Before)
	if err != nil {
		return fmt.Errorf("audit before: %w", err)
	}
	after, err := marshalNullable(e.After)
	if err != nil {
		return fmt.Errorf("audit after: %w", err)
	}

	const q = `
		INSERT INTO audit_log
			(id, actor_user_id, actor_role, action, entity_type, entity_id,
			 before_state, after_state, request_id, created_at)
		VALUES
			('SWP-AL-' || swp_next_id('AL'), $1, $2, $3, $4, $5, $6, $7, $8, now())`
	_, err = tx.Exec(ctx, q,
		nullStr(p.UserID), nullStr(string(p.Role)),
		string(e.Action), e.EntityType, e.EntityID,
		before, after, nullStr(reqID),
	)
	if err != nil {
		return fmt.Errorf("insert audit_log: %w", err)
	}
	return nil
}

func marshalNullable(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
