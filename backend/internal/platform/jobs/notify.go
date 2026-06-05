package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
)

// NotificationArgs is the payload for a notification dispatch (CONVENTIONS §16.2).
// Enqueued in the SAME tx as the originating write (transactional outbox) so it
// never fires for a rolled-back action and is never lost. The worker
// (NotificationWorker.Work) resolves these fields into a durable notifications
// row (E10 / SWP-NTF-*) — no second query needed: every field the row requires
// travels on the payload.
//
// NotifKind is the openapi NotificationKind value (e.g. "LEAVE_APPROVED"); Event
// is kept for back-compat with the pre-E10 enqueue shape (services may mirror
// NotifKind into Event). NOTE: the field is NotifKind, not Kind — Kind() is
// River's reserved JobArgs method (its job-type identifier). RecipientID is
// SWP-USR-… (system/HR targets) or SWP-EMP-… (submitter targets) — the
// GET /notifications scope filters on BOTH so either resolves to the logged-in
// principal.
type NotificationArgs struct {
	Event       string            `json:"event,omitempty"`       // legacy back-compat (mirrors NotifKind)
	NotifKind   string            `json:"kind"`                  // openapi NotificationKind
	RecipientID string            `json:"recipient_id"`          // SWP-EMP-… / SWP-USR-…
	Title       string            `json:"title"`                 // Bahasa headline
	Body        string            `json:"body"`                  // single-line body
	EntityType  string            `json:"entity_type,omitempty"` // legacy back-compat
	EntityID    string            `json:"entity_id,omitempty"`   // legacy back-compat

	// Deep link (flattened onto the row; the repo re-nests into DeepLink).
	DeepLinkEpic     string `json:"deep_link_epic,omitempty"`
	DeepLinkEntityID string `json:"deep_link_entity_id,omitempty"`
	DeepLinkPath     string `json:"deep_link_path,omitempty"`

	// Actor (who triggered the underlying event; empty ID = system actor).
	ActorID    string `json:"actor_id,omitempty"`
	ActorLabel string `json:"actor_label,omitempty"`

	IsCritical bool `json:"is_critical,omitempty"`

	Data map[string]string `json:"data,omitempty"` // legacy free-form payload
}

// Kind is River's stable job identifier.
func (NotificationArgs) Kind() string { return "notification.dispatch" }

// Dispatcher is the reusable transactional-outbox seam any service can call from
// inside its write tx to enqueue a notification. *Client satisfies it.
type Dispatcher interface {
	Dispatch(ctx context.Context, tx pgx.Tx, args NotificationArgs) error
}

var _ Dispatcher = (*Client)(nil)

// Dispatch enqueues a NotificationArgs job within the caller's tx (transactional
// outbox). Identical guarantee to EnqueueTx: the job is committed atomically with
// the originating write and rolled back with it, so a notification never fires
// for an action that did not happen. Call this from a service inside
// TxManager.InTx, after the state transition + audit, with a COMPLETE args (the
// worker writes the row verbatim).
func (c *Client) Dispatch(ctx context.Context, tx pgx.Tx, args NotificationArgs) error {
	return c.EnqueueTx(ctx, tx, args)
}

// Dispatch is the package-level convenience wrapper so call sites read
// `notify.Dispatch(ctx, dispatcher, tx, args)` without holding a *Client. The
// dispatcher seam keeps services unit-testable with a fake (no River/Postgres).
func Dispatch(ctx context.Context, d Dispatcher, tx pgx.Tx, args NotificationArgs) error {
	if d == nil {
		// Nil-safe (mirrors the pre-E10 nil jobs seam in unit tests): a service
		// wired without a dispatcher simply does not notify.
		return nil
	}
	return d.Dispatch(ctx, tx, args)
}

// NotificationWorker persists a queued notification as a durable notifications
// row (E10). It writes to the application DB from Work(), so — like
// PayslipExportWorker — it is constructed WITH the *db.Pool (registered in
// NewWorkerClient where the pool is in scope).
//
// External delivery (FCM/APNs) is out of scope (CONTEXT deferred): the in-app row
// IS the durable record (NT-6). A write error is returned so River backs off and
// retries; the originating action already committed, so a failed notification
// never blocks it.
type NotificationWorker struct {
	river.WorkerDefaults[NotificationArgs]
	pool *db.Pool
}

// NewNotificationWorker constructs the worker with the pool it writes through.
func NewNotificationWorker(pool *db.Pool) *NotificationWorker {
	return &NotificationWorker{pool: pool}
}

func (w *NotificationWorker) Work(ctx context.Context, job *river.Job[NotificationArgs]) error {
	a := job.Args

	kind := a.NotifKind
	if kind == "" {
		kind = a.Event // back-compat: older enqueues only set Event
	}

	q := sqlcgen.New(w.pool.Pool)
	row, err := q.InsertNotification(ctx, sqlcgen.InsertNotificationParams{
		RecipientID:      a.RecipientID,
		Kind:             kind,
		Title:            a.Title,
		Body:             a.Body,
		DeepLinkEpic:     strNilEmpty(a.DeepLinkEpic),
		DeepLinkEntityID: strNilEmpty(a.DeepLinkEntityID),
		DeepLinkPath:     strNilEmpty(a.DeepLinkPath),
		ActorID:          strNilEmpty(a.ActorID),
		ActorLabel:       strNilEmpty(a.ActorLabel),
		IsCritical:       a.IsCritical,
	})
	if err != nil {
		return fmt.Errorf("notification.dispatch: insert row (recipient=%s kind=%s): %w", a.RecipientID, kind, err)
	}

	slog.InfoContext(ctx, "notification.dispatch",
		"id", row.ID,
		"kind", kind,
		"recipient", a.RecipientID,
	)
	return nil
}

// strNilEmpty maps "" → nil so the COALESCE-defaulted columns (deep_link_path,
// actor_label) take their DB defaults and the nullable columns store NULL.
func strNilEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
