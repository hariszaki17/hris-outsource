# PRD · F9.5 — Cutover, Validation & Rollback

> **Epic:** E9 Data Migration · **Feature:** F9.5 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

The big-bang switch: at cutover we **freeze** ims-system, run the **final** migration, run **validation gates**, and only on a clean **go/no-go** do we switch users to hris-outsource — with a documented **rollback** if validation fails. This is the highest-risk moment; it must be a rehearsed runbook, not an improvisation.

## 2. Goals & non-goals

**Goals**
- A repeatable cutover runbook: freeze → final run → validate → go/no-go → switch → monitor.
- Validation gates that must pass before switching.
- A rollback path that keeps SWP on ims-system if validation fails.

**Non-goals**
- The load mechanics (F9.4). Ongoing post-launch ops.

## 3. Actors

Migration engineer/ops (runbook), HR/Super Admin (validation sign-off, review gate), Leadership (go/no-go), System (validation checks).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Runbook + tooling** | Engineer / ops | Freeze, final run, switch, rollback. |
| **Web console** | HR / Super Admin | Validation reports + sign-off; review-queue gate (F9.3). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| CV-1 | **Freeze:** ims-system is set read-only/frozen for the final run (big-bang — no dual writes). |
| CV-2 | The **final run** is a full F9.1–F9.4 pass on the frozen snapshot; all **blocking** review items (F9.3) must be Resolved (go-live gate). |
| CV-3 | **Validation gates** must pass: per-entity **record counts** (source vs loaded), **leave-balance** reconciliation, **payslip totals**, and **sample spot-checks** of key agents/placements. |
| CV-4 | A **go/no-go** decision is recorded; switching proceeds only on GO. |
| CV-5 | **Rollback:** on NO-GO, users stay on ims-system (unfreeze), issues are fixed, and cutover is retried — the partially-loaded target is discarded/reset. |
| CV-6 | After GO, traffic switches to hris-outsource and **post-cutover monitoring** runs (errors, key flows). |
| CV-7 | Legacy `lumen_swp` is retained **read-only** for reference post-cutover (period per §10). |
| CV-8 | The entire cutover is audited; the final `MigrationRun` + validation results are archived. |

## 6. Data model

`MigrationRun` (final), validation results, go/no-go record. No new domain entities.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Cutover, validation & rollback

  Scenario: Final run requires a clean review gate
    Given open blocking review items exist
    When cutover is attempted
    Then it is blocked until they are resolved

  Scenario: Validation gates pass → GO
    Given the final run completed
    When validation runs
    And record counts, leave balances, and payslip totals reconcile and spot-checks pass
    Then a GO decision is recorded and users switch to hris-outsource

  Scenario: Validation fails → rollback
    Given a validation gate fails (e.g., counts mismatch)
    Then a NO-GO is recorded
    And SWP remains on ims-system (unfrozen) while the issue is fixed

  Scenario: Freeze prevents legacy writes during final run
    Given the final run is in progress
    Then ims-system is read-only and no new legacy writes occur

  Scenario: Post-cutover monitoring
    Given the switch is done
    Then key flows are monitored and the final run is archived
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | New legacy writes slip in during freeze | Prevented by read-only freeze; any caught delta re-migrated. |
| C-2 | Validation passes but a problem appears post-switch | Documented hotfix path; rollback window defined (§10). |
| C-3 | Maintenance window overruns | Pre-rehearsed timings; dry-runs size the window (F9.4). |
| C-4 | Partial target from a failed attempt | Reset/discard; crosswalk-keyed re-run is clean. |

## 9. Dependencies

F9.1–F9.4 (the pipeline), F9.3 (go-live gate), E1 (audit), leadership/HR sign-off.

## 10. Decisions & open questions

- ✅ Big-bang: freeze → final run → validation gates → go/no-go → switch → monitor; rollback to ims-system on failure.
- **Open:** exact **maintenance-window** length + rehearsal schedule (sized by dry-runs).
- **Open:** the precise **validation gate thresholds** and required sign-offs.
- **Open:** how long `lumen_swp` is kept read-only post-cutover, and any post-switch **rollback window**.
