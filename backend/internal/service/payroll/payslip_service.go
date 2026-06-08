// Package payroll — PayslipService: read payslips (list/detail) decrypting money
// AT THE BOUNDARY, surfacing a row whose ciphertext fails to open as a 200 OK
// payslip with status DECRYPT_FAIL (money nulled, breakdown empty) — NEVER a 4xx;
// plus append-only audit notes (list + create, audited in-tx).
//
// Mirrors the Phase-2 foundations slice (read + list + simple write + audit). RBAC
// is route-enforced (hr/super = global), so there is no agent-scope branch here.
package payroll

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/crypto"
)

// PayslipService implements the payslip read + audit-note business logic.
type PayslipService struct {
	repo   PayslipRepository
	txm    TxRunner
	cipher *crypto.Cipher
	jobs   Jobs // notify-stub enqueue (TODO Phase-11); may be nil in unit tests
}

// NewPayslipService wires the payslip service. cipher decrypts the *_enc money at
// the read boundary (the DECRYPT_FAIL source). jobs is the River seam for the
// note notify stub (nil-safe).
func NewPayslipService(repo PayslipRepository, txm TxRunner, cipher *crypto.Cipher, jobs Jobs) *PayslipService {
	return &PayslipService{repo: repo, txm: txm, cipher: cipher, jobs: jobs}
}

// List returns one page of payslip summaries (paid_on DESC). For each row the
// three summary money ciphertexts are decrypted; if ANY present column fails to
// open the row is marked DECRYPT_FAIL (money + working_days nulled). DECRYPT_FAIL
// rows are NEVER filtered out and NEVER cause a non-200. The returned
// missingHistory flag is true on zero rows (→ meta.code MISSING_PAYROLL_HISTORY).
func (s *PayslipService) List(ctx context.Context, f PayslipFilter) (rows []dom.Payslip, next *string, hasMore, missingHistory bool, err error) {
	// Agent (mobile, scope:self / PAY-01): force employee_id to the caller; reject
	// an explicit employee_id that is not their own. Staff (hr/super) keep the
	// global archive (route RBAC already gates non-self roles out).
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleAgent {
		if f.EmployeeID != nil && *f.EmployeeID != p.EmployeeID {
			return nil, nil, false, false, apperr.OutOfScope()
		}
		eid := p.EmployeeID
		f.EmployeeID = &eid
	}

	limit := clampLimit(f.Limit)
	f.Limit = limit + 1

	raw, lerr := s.repo.ListPayslips(ctx, f)
	if lerr != nil {
		return nil, nil, false, false, apperr.Internal(lerr)
	}

	hasMore = len(raw) > limit
	if hasMore {
		raw = raw[:limit]
	}

	out := make([]dom.Payslip, 0, len(raw))
	for _, r := range raw {
		out = append(out, s.summaryFromRow(r))
	}

	if hasMore && len(raw) > 0 {
		last := raw[len(raw)-1]
		c, cerr := encodePayslipCursor(last.PaidOn, last.ID)
		if cerr != nil {
			return nil, nil, false, false, cerr
		}
		next = &c
	}

	missingHistory = len(out) == 0
	return out, next, hasMore, missingHistory, nil
}

// summaryFromRow decrypts the three summary money fields and resolves the row
// status. No breakdown arrays on the list shape.
func (s *PayslipService) summaryFromRow(r PayslipRow) dom.Payslip {
	p := dom.Payslip{
		ID:           r.ID,
		EmployeeID:   r.EmployeeID,
		EmployeeName: r.EmployeeName,
		PlacementID:  r.PlacementID,
		Year:         r.Year,
		Month:        r.Month,
		Period:       periodString(r.Year, r.Month),
		PaidOn:       r.PaidOn,
		WorkingDays:  r.WorkingDays,
		ReadOnly:     true,
		Status:       dom.PayslipStatusFinal,
		Source:       dom.SourceRef{System: r.SourceSystem, SourceID: r.SourceID},
		CreatedAt:    r.CreatedAt,
	}

	ge, f1 := decryptMoney(s.cipher, r.GrossEarningsEnc)
	gd, f2 := decryptMoney(s.cipher, r.GrossDeductionsEnc)
	th, f3 := decryptMoney(s.cipher, r.TakeHomePayEnc)

	if f1 || f2 || f3 {
		s.markDecryptFail(&p)
		return p
	}
	p.GrossEarnings, p.GrossDeductions, p.TakeHomePay = ge, gd, th
	return p
}

// Get loads one payslip + its full breakdown, decrypting at the boundary. If the
// summary OR any line fails to decrypt, the WHOLE payslip is DECRYPT_FAIL (money
// nulled, earnings/deductions/benefits = []) — partial decryption is never
// surfaced. NotFound → 404.
func (s *PayslipService) Get(ctx context.Context, id string) (dom.Payslip, error) {
	r, err := s.repo.GetPayslip(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.Payslip{}, apperr.NotFound()
	}
	if err != nil {
		return dom.Payslip{}, apperr.Internal(err)
	}

	// Agent (mobile, scope:self / PAY-01): may read only their own payslip; any
	// other employee's payslip is hidden as 404 (no existence leak). Staff
	// (hr/super) keep global read (route RBAC gates other roles out).
	if pr, ok := auth.PrincipalFrom(ctx); ok && pr.Role == auth.RoleAgent {
		if pr.EmployeeID == "" || pr.EmployeeID != r.EmployeeID {
			return dom.Payslip{}, apperr.NotFound()
		}
	}

	p := s.summaryFromRow(r)

	components, err := s.repo.ListComponents(ctx, id)
	if err != nil {
		return dom.Payslip{}, apperr.Internal(err)
	}
	benefits, err := s.repo.ListBenefits(ctx, id)
	if err != nil {
		return dom.Payslip{}, apperr.Internal(err)
	}

	// If the summary already failed, surface the canonical decrypt-fail shape
	// (empty breakdown) and stop — markDecryptFail in summaryFromRow set status.
	if p.Status == dom.PayslipStatusDecryptFail {
		p.Earnings, p.Deductions, p.Benefits = []dom.EarningLine{}, []dom.DeductionLine{}, []dom.BenefitLine{}
		return p, nil
	}

	earnings := make([]dom.EarningLine, 0)
	deductions := make([]dom.DeductionLine, 0)
	benefitLines := make([]dom.BenefitLine, 0)
	lineFail := false

	for _, c := range components {
		v, failed := decryptMoney(s.cipher, c.ValueEnc)
		if failed {
			lineFail = true
			break
		}
		switch c.Kind {
		case "EARNING":
			earnings = append(earnings, dom.EarningLine{Name: c.Name, Value: v, ForBPJS: c.ForBPJS})
		case "DEDUCTION":
			deductions = append(deductions, dom.DeductionLine{Name: c.Name, Value: v, ForBPJS: c.ForBPJS})
		}
	}
	if !lineFail {
		for _, b := range benefits {
			v, failed := decryptMoney(s.cipher, b.ValueEnc)
			if failed {
				lineFail = true
				break
			}
			benefitLines = append(benefitLines, dom.BenefitLine{Name: b.Name, Value: v})
		}
	}

	if lineFail {
		s.markDecryptFail(&p)
		p.Earnings, p.Deductions, p.Benefits = []dom.EarningLine{}, []dom.DeductionLine{}, []dom.BenefitLine{}
		return p, nil
	}

	p.Earnings = earnings
	p.Deductions = deductions
	p.Benefits = benefitLines
	return p, nil
}

// markDecryptFail applies the canonical DECRYPT_FAIL nulling (per the openapi
// field table): status DECRYPT_FAIL, decrypt_fail true, locked_reason set, all
// money + working_days nulled.
func (s *PayslipService) markDecryptFail(p *dom.Payslip) {
	p.Status = dom.PayslipStatusDecryptFail
	p.DecryptFail = true
	reason := dom.LockedReasonDecryptFail
	p.LockedReason = &reason
	p.GrossEarnings = nil
	p.GrossDeductions = nil
	p.TakeHomePay = nil
	p.WorkingDays = nil
}

// --- audit notes ---

// ListAuditNotes returns one chronological page of notes (created_at ASC). 404 if
// the payslip does not exist. Notes are returned even on DECRYPT_FAIL payslips.
func (s *PayslipService) ListAuditNotes(ctx context.Context, payslipID, cursor string) ([]dom.PayslipAuditNote, *string, bool, error) {
	exists, err := s.repo.PayslipExists(ctx, payslipID)
	if err != nil {
		return nil, nil, false, apperr.Internal(err)
	}
	if !exists {
		return nil, nil, false, apperr.NotFound()
	}

	cursorCreatedAt, cursorSeq, derr := DecodeAuditNoteCursor(cursor)
	if derr != nil {
		return nil, nil, false, derr
	}

	limit := clampLimit(0) // notes use the default page size (no client limit on the FE)
	rows, err := s.repo.ListAuditNotes(ctx, payslipID, cursorSeq, cursorCreatedAt, limit+1)
	if err != nil {
		return nil, nil, false, apperr.Internal(err)
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c, cerr := encodeAuditNoteCursor(last.CreatedAt, seqFromID(last.ID))
		if cerr != nil {
			return nil, nil, false, cerr
		}
		next = &c
	}
	return rows, next, hasMore, nil
}

// CreateAuditNote appends an immutable note (append-only). 404 if the payslip does
// not exist; 400 if the text is empty-after-trim or > 4000 chars. In-tx: assigns
// seq = count+1, the composite id "{payslip_id}-NOTE-{seq}", inserts, and audits.
func (s *PayslipService) CreateAuditNote(ctx context.Context, payslipID, text string) (dom.PayslipAuditNote, error) {
	exists, err := s.repo.PayslipExists(ctx, payslipID)
	if err != nil {
		return dom.PayslipAuditNote{}, apperr.Internal(err)
	}
	if !exists {
		return dom.PayslipAuditNote{}, apperr.NotFound()
	}

	trimmed := strings.TrimSpace(text)
	if n := len([]rune(trimmed)); n < 1 || n > 4000 {
		return dom.PayslipAuditNote{}, apperr.Invalid(map[string]string{"text": "Catatan wajib 1–4000 karakter."})
	}

	author := actorEmployeeID(ctx)
	authorID := deref(author)

	var out dom.PayslipAuditNote
	err = s.txm.InTx(ctx, func(tx pgx.Tx) error {
		count, cerr := s.repo.CountAuditNotes(ctx, payslipID)
		if cerr != nil {
			return cerr
		}
		seq := count + 1
		id := fmt.Sprintf("%s-NOTE-%d", payslipID, seq)

		note, ierr := s.repo.InsertAuditNote(ctx, tx, AuditNoteRow{
			ID:         id,
			PayslipID:  payslipID,
			Seq:        seq,
			Text:       trimmed,
			AuthorID:   authorID,
			AuthorName: nil, // denormalized name resolution deferred (E2 lookup); FE falls back to author_id
		})
		if ierr != nil {
			return ierr
		}
		out = note

		if aerr := audit.Record(ctx, tx, audit.Entry{
			Action:     "CREATE",
			EntityType: "payslip_audit_note",
			EntityID:   payslipID,
			After:      map[string]any{"note_id": id, "text": trimmed},
		}); aerr != nil {
			return aerr
		}

		// Notify stub (TODO Phase-11): a real NotificationArgs job would announce
		// PAYSLIP_NOTE_ADDED. Enqueued in-tx (transactional outbox) when wired.
		return nil
	})
	if err != nil {
		return dom.PayslipAuditNote{}, asAppErr(err)
	}
	return out, nil
}

// seqFromID extracts the trailing {seq} from a composite "{payslip_id}-NOTE-{seq}"
// id for the audit-note cursor. Returns 0 on any parse miss (cursor still valid by
// created_at).
func seqFromID(id string) int {
	idx := strings.LastIndex(id, "-NOTE-")
	if idx < 0 {
		return 0
	}
	tail := id[idx+len("-NOTE-"):]
	n := 0
	for _, c := range tail {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
