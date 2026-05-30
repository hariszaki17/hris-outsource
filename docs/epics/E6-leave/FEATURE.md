# E6 — Leave Management · Feature Document

> **Epic:** E6 Leave Management · **Status:** Draft v1 · **Parent:** [EPICS.md](../../EPICS.md)
> Annual leave quotas, agent leave requests with documents, two-level (leader → HR) approval, and integration with scheduling/attendance.

---

## 1. Goal & outcome

Let agents request leave from mobile, track their **annual quota** (cuti tahunan), route requests through **shift-leader then HR** approval, and ensure approved leave **cancels scheduled shifts and never reads as "absent"** in attendance. Annual entitlement is a **lump grant per period that expires at period end**; over-balance annual requests are **blocked**.

## 2. Actors & roles

| Actor | Involvement |
|---|---|
| **Agent** | Requests leave (mobile), uploads documents, names a delegate, views balance/history. |
| **Shift Leader** | First-level approver for their company's agents. |
| **HR / Super Admin** | Second-level approver; manages quotas/grants; handles no-leader escalation. |
| **System** | Enforces balance, runs the two-level flow, cancels shifts, suppresses absent, audits, notifies. |

## 3. Scope

**In scope:** leave quotas/balances, leave request (+ documents, delegate), two-level approval, leave↔schedule/attendance integration, leave calendar & balances.
**Out of scope:** leave-type definitions (E2 master). Payroll effect of unpaid leave (E8 context). Overtime (E7).

## 4. Domain entities

```mermaid
erDiagram
    EMPLOYEE ||--o{ LEAVE_QUOTA : "has"
    EMPLOYEE ||--o{ LEAVE_REQUEST : "files"
    LEAVE_TYPE ||--o{ LEAVE_REQUEST : "categorizes"
    LEAVE_TYPE ||--o{ LEAVE_QUOTA : "annual types"
    LEAVE_REQUEST ||--o{ LEAVE_APPROVAL : "decided via"

    LEAVE_QUOTA {
        bigint id PK
        bigint employee_id FK
        bigint leave_type_id FK
        date period_start
        date period_end
        int total
        int used
        int remaining
    }
    LEAVE_REQUEST {
        bigint id PK
        bigint employee_id FK
        bigint leave_type_id FK
        bigint delegate_id FK "nullable"
        date start_date
        date end_date
        int duration_days
        string status "Pending|LeaderApproved|Approved|Rejected|Cancelled"
        text notes
        text admin_notes
        string document_url "if required"
        datetime issued_at
    }
    LEAVE_APPROVAL {
        bigint id PK
        bigint leave_request_id FK
        int level "1=leader, 2=HR"
        bigint approver_id FK
        string decision "Approved|Rejected"
        text reason
        datetime decided_at
    }
```

**Invariants:**
- **INV-1:** annual (`is_tahunan`) requests deduct from the active `LeaveQuota`; a request **cannot exceed `remaining`** (blocked).
- **INV-2:** **two-level approval** — `Pending → LeaderApproved → Approved`; a reject at either level ends it (`Rejected`).
- **INV-3:** an **Approved** leave **cancels overlapping scheduled shifts** (E4) and **suppresses "Absent"** in attendance (E5) for those days.
- **INV-4:** annual quota is a **lump grant per period**, **expires at `period_end`** (no carryover).
- **INV-5:** leave types flagged `is_document_required` (E2) require a document upload before submission.

## 5. Features

| ID | Feature | PRD |
|----|---------|-----|
| **F6.1** | Leave Quota & Balances | [leave-quota-balances.md](prds/leave-quota-balances.md) |
| **F6.2** | Leave Request (documents, delegate) | [leave-request.md](prds/leave-request.md) |
| **F6.3** | Two-Level Approval Workflow | [leave-approval.md](prds/leave-approval.md) |
| **F6.4** | Leave–Schedule/Attendance Integration | [leave-schedule-integration.md](prds/leave-schedule-integration.md) |
| **F6.5** | Leave Calendar & Balance Views | [leave-calendar-views.md](prds/leave-calendar-views.md) |

## 6. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | Request leave, upload docs, pick delegate, view balance & status. |
| **Web / mobile** | Shift Leader | First-level approve/reject for their company. |
| **Web console** | HR / Super Admin | Second-level approval, quota grants/adjustments, reporting. |

---

### F6.1 — Leave Quota & Balances

Grant each agent their annual entitlement as a **lump sum per period** (anniversary or calendar year), track `used`/`remaining`, and **expire** unused days at period end.

```mermaid
flowchart TD
    subgraph SYS[System / HR]
        A1([Grant job at period start]) --> A2[Create LeaveQuota total=entitlement, used=0]
        A3([Leave approved]) --> A4[used += duration, remaining -= duration]
        A5([Period end job]) --> A6[Expire remaining, close quota]
        A7[HR manual adjustment] --> A8[Adjust total/remaining + reason]
    end
```

**Entities:** `LeaveQuota`. **Depends on:** E2 (annual leave type, entitlement source).

---

### F6.2 — Leave Request (documents, delegate)

Agent submits a request: type, date range, computed duration, optional **delegate** (who covers), and a **document** when the type requires it. Annual requests pre-check balance.

```mermaid
flowchart TD
    subgraph AG[Agent - mobile]
        B1([New leave request]) --> B2[Type + date range + reason]
        B2 --> B3{Type requires document?}
        B3 -- Yes --> B4[Upload document]
        B3 -- No --> B5[Pick delegate optional]
        B4 --> B5
        B5 --> B6[Submit]
    end
    subgraph SYS[System]
        B6 --> C1[Compute duration_days]
        C1 --> C2{Annual & duration > remaining?}
        C2 -- Yes --> C3[Block: insufficient balance]
        C2 -- No --> C4[Create LeaveRequest = Pending]
        C4 --> C5[(Persist + audit)]
        C5 --> C6[Notify shift leader]
    end
```

**Entities:** `LeaveRequest`. **Depends on:** E2 (leave types), F6.1 (balance).

---

### F6.3 — Two-Level Approval Workflow

Shift leader approves first; then HR confirms. Reject at either level ends the request. On final approval, the quota is deducted and downstream integration (F6.4) fires.

```mermaid
flowchart TD
    subgraph SL[Shift Leader]
        L1([Pending request]) --> L2{Approve?}
        L2 -- Reject --> R1[Rejected + reason]
        L2 -- Approve --> L3[LeaderApproved]
    end
    subgraph HR[HR Admin]
        L3 --> H1{Approve?}
        H1 -- Reject --> R1
        H1 -- Approve --> H2[Approved]
    end
    subgraph SYS[System]
        H2 --> S1[Deduct quota F6.1]
        S1 --> S2[Cancel shifts + suppress absent F6.4]
        S2 --> S3[(Persist approvals + audit)]
        S3 --> S4[Notify agent]
        R1 --> S4
    end
```

**Entities:** `LeaveApproval`, `LeaveRequest`. **Depends on:** F3.4 (leader scope / HR escalation).

---

### F6.4 — Leave–Schedule/Attendance Integration

On approval, overlapping **scheduled shifts (E4) are cancelled/marked leave**, and attendance (E5) **does not mark those days Absent**. Cancelling/shortening an approved leave restores the schedule state.

```mermaid
flowchart LR
    subgraph SYS[System]
        I1([Leave Approved]) --> I2[Find overlapping schedules E4]
        I2 --> I3[Cancel/mark them as Leave]
        I3 --> I4[Tag those dates so E5 skips Absent]
        I5([Leave cancelled/shortened]) --> I6[Restore affected schedule days]
    end
```

**Entities:** updates `Schedule` (E4), informs `Attendance` (E5). **Depends on:** E4, E5.

---

### F6.5 — Leave Calendar & Balance Views

Agent sees their balance + request history (mobile); leader/HR see a team leave calendar (who's off when) for planning coverage.

```mermaid
flowchart LR
    subgraph AG[Agent - mobile]
        V1([My leave]) --> V2[Balance + history + status]
    end
    subgraph LH[Leader / HR - web]
        V3([Team leave calendar]) --> V4[Who's on leave by date + pending requests]
    end
```

**Entities:** reads `LeaveQuota`, `LeaveRequest`. **Depends on:** F6.1–F6.3.

---

## 7. Decisions & open questions

**Resolved (2026-05-29):**
- ✅ **Annual lump grant** per period; tracked total/used/remaining (matches legacy `employee_leave_quotas`).
- ✅ **Expire at period end** (no carryover).
- ✅ **Two-level approval**: shift leader → HR (escalate to HR if no leader).
- ✅ **Block** annual requests beyond remaining balance.

**Resolved — open-items review (2026-05-29), see [EPICS.md §8](../../EPICS.md):**
- ✅ **Duration** = working days **excluding public holidays**.
- ✅ **Period basis** = **calendar year**.
- ✅ **Probation** = **pro-rated** annual leave (also pro-rate mid-year joiners).
- ✅ **Non-annual types** (sick/maternity/unpaid) = **per-type quotas** (`LeaveQuota` generalized to one per employee/leave_type/period).
- ✅ **Half-day leave** = not in v1 (full days only).
- ✅ **Delegate** = informational/notified (no enforced coverage).

**Still open (confirm with SWP):**
1. Exact "working day" definition for 24/7 shift workers (rostered days vs standard business days) used in duration counting.
