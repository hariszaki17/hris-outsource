package jobs

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"
)

// NotificationArgs is the payload for a fire-and-forget notification dispatch
// (CONVENTIONS §16.2). Enqueued in the same tx as the originating write; the
// worker delivers via FCM/APNs and records the SWP-NTF row (E10).
type NotificationArgs struct {
	Event       string            `json:"event"`        // e.g. "LEAVE_APPROVED"
	RecipientID string            `json:"recipient_id"` // SWP-EMP-… / SWP-USR-…
	EntityType  string            `json:"entity_type"`
	EntityID    string            `json:"entity_id"`
	Data        map[string]string `json:"data,omitempty"`
}

// Kind is River's stable job identifier.
func (NotificationArgs) Kind() string { return "notification.dispatch" }

// NotificationWorker delivers a queued notification. Stubbed: the real
// implementation lands with E10 (notification center + FCM/APNs transport).
type NotificationWorker struct {
	river.WorkerDefaults[NotificationArgs]
}

func (w *NotificationWorker) Work(ctx context.Context, job *river.Job[NotificationArgs]) error {
	slog.InfoContext(ctx, "notification.dispatch",
		"event", job.Args.Event,
		"recipient", job.Args.RecipientID,
		"entity", job.Args.EntityType+"/"+job.Args.EntityID,
	)
	// TODO(E10): persist SWP-NTF row + deliver via FCM/APNs. Failures here retry
	// per River's backoff; they never block the originating API action.
	return nil
}
