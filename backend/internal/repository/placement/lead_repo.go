// Package placement (repository) — LeadRepo backs the `lead` role's company-set
// resolution. A lead (service-line operational approver) covers MANY client
// companies; the auth middleware reads the active set here at request time to
// populate Principal.CompanyIDs (mirror of ShiftLeaderRepo's single-company
// GAP-3 derivation).
package placement

import (
	"context"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
)

// LeadRepo is the sqlc-backed reader for lead_assignments.
type LeadRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

// NewLeadRepo returns a LeadRepo backed by pool.
func NewLeadRepo(pool *db.Pool) *LeadRepo {
	return &LeadRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// GetActiveLeadCompaniesForEmployee returns the set of client companies the lead
// (employee) currently covers via active lead_assignments. Non-locking pool read
// used by the auth middleware to DERIVE a lead's company scope (Principal.CompanyIDs)
// at request time — so reassigning a lead takes effect on their next request rather
// than at next login. Returns an empty slice (nil error) when the lead covers none.
func (r *LeadRepo) GetActiveLeadCompaniesForEmployee(ctx context.Context, employeeID string) ([]string, error) {
	ids, err := r.q.ListActiveLeadCompaniesForEmployee(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	return ids, nil
}
