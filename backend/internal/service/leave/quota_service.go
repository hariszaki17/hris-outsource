// Package leave — QuotaService: the per-type balance read (F6.5 "Saldo per jenis").
// The HR per-type entitlement adjust (LQ-6) lives on LeaveService (it meters through
// the QuotaMeter); this service owns the cap_basis-resolved balance list.
package leave

import (
	"context"
	"fmt"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
)

// QuotaService implements the per-type quota balance read.
type QuotaService struct {
	repo QuotaRepository
	txm  TxRunner
	now  Clock
}

// NewQuotaService wires the quota service.
func NewQuotaService(repo QuotaRepository, txm TxRunner) *QuotaService {
	return &QuotaService{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *QuotaService) SetClock(c Clock) { s.now = c }

// EmployeeTypeBalances returns the per-type balance for an employee (F6.5) — every
// active leave type with its current-window quota (resolved by cap_basis).
func (s *QuotaService) EmployeeTypeBalances(ctx context.Context, employeeID string) ([]dom.TypeBalance, error) {
	now := s.now()
	curYear := fmt.Sprintf("%04d", now.Year())
	curMonth := fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	return s.repo.ListEmployeeTypeBalances(ctx, employeeID, curYear, curMonth)
}
