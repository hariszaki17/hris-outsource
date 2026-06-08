# E3 — Placement Management · Feature Document

> **Epic:** E3 Placement Management (the differentiator) · **Status:** Draft v1 · **Parent:** [EPICS.md](../../EPICS.md)
> Placing agents at client companies, in a service line, for a contract period — with history, lifecycle, and the per-company shift leader.

---

## 1. Goal & outcome

Make **placement a first-class entity** (in the legacy system it was just a string on `employee_contracts`). After this epic, SWP can place an agent at a client company in a service line for a defined period, track the full placement history of every agent, and know exactly which shift leader owns each company's on-site team. Every downstream module (shift scheduling, attendance, leave, overtime) hangs off the placement record.

## 2. Actors & roles

| Actor | Involvement in this epic |
|---|---|
| **HR / Placement Admin** | Primary driver — creates, activates, transfers, and ends placements; assigns shift leaders. |
| **Super Admin** | Same powers as HR admin + can override/correct any placement; manages master data (companies, service lines). |
| **Shift Leader** | Designated per company; consumes the roster (read) and is assigned/unassigned by HR admin. |
| **Agent** | Subject of a placement; views own active placement & history (read-only). |
| **System** | Validates rules, manages status transitions, emits notifications, writes audit log. |

## 3. Scope

**In scope:** placement creation (located at a **Site**, E2 F2.6), lifecycle/status, re-placement & transfer with history, shift-leader assignment (1 per **leadership unit** — company or site, per `leader_scope`), company roster view.
**Out of scope (other epics):** the shift master & rostering (E4), attendance (E5), leave (E6), overtime (E7), payroll figures (E8 read-only), and the migration of legacy placement data (E9).

## 4. Domain entities

```mermaid
erDiagram
    EMPLOYEE ||--o{ EMPLOYMENT_AGREEMENT : "employed under"
    EMPLOYMENT_AGREEMENT ||--o{ PLACEMENT : "designated via"
    CLIENT_COMPANY ||--o{ PLACEMENT : "hosts"
    CLIENT_COMPANY ||--|{ SITE : "has one or more"
    SITE ||--o{ PLACEMENT : "located at"
    SERVICE_LINE ||--o{ PLACEMENT : "categorizes"
    POSITION ||--o{ PLACEMENT : "role at site"
    CLIENT_COMPANY ||--o{ SHIFT_LEADER_ASSIGNMENT : "leadership unit (scope)"
    SITE ||--o{ SHIFT_LEADER_ASSIGNMENT : "leadership unit when scope=site"
    EMPLOYEE ||--o{ SHIFT_LEADER_ASSIGNMENT : "serves as"

    EMPLOYMENT_AGREEMENT {
        bigint id PK
        bigint employee_id FK
        string type "PKWT or PKWTT"
        string agreement_no
        date start_date
        date end_date "null for PKWTT"
        string status
    }
    PLACEMENT {
        bigint id PK
        bigint employee_id FK
        bigint employment_agreement_id FK
        bigint client_company_id FK
        bigint site_id FK "required; site.client_company_id = client_company_id"
        bigint service_line_id FK
        bigint position_id FK
        date start_date
        date end_date "null = open-ended"
        string status
        string ended_reason
        bigint predecessor_id FK "renewal or transfer chain"
        bigint created_by FK
    }
    SHIFT_LEADER_ASSIGNMENT {
        bigint id PK
        bigint client_company_id FK
        bigint site_id FK "null when leader_scope=company; set when =site"
        bigint employee_id FK
        datetime assigned_at
        datetime unassigned_at
    }
```

> **Employment vs placement (Indonesian labor law):** In outsourcing (alih daya), the employment relationship is between the agent and **SWP**, not the client. So the **EmploymentAgreement** (`PKWT` fixed-term / `PKWTT` indefinite) lives at the employee↔SWP level (modeled in E2), and a **Placement** is only a *work designation* to a client site. A placement references its employment agreement; for `PKWT` the placement period must fall within the agreement's validity, while `PKWTT` agreements allow **open-ended** placements.

**Invariants (confirmed 2026-05-29; INV-2/3/4 + INV-5 revised 2026-06-03 for client sites — see §7):**
- **INV-1:** an agent has **at most one *active* placement** at any moment (no split/multi-site agents). ✅
- **INV-5:** a placement is located at **exactly one `Site`** (`Placement.site_id` required, E2 F2.6), and that site belongs to the placement's client company. ✅
- A company's **leadership unit** depends on `ClientCompany.leader_scope` (E2): `company` → the company; `site` → each active site. INV-2/3/4 are stated per *leadership unit*:
  - **INV-2:** a leadership unit with active placements has **exactly one** shift leader.
  - **INV-3:** a shift leader leads **exactly one** leadership unit (one company **or** one site) — strict 1:1. ✅
  - **INV-4:** the designated shift leader must themselves be an agent **actively placed within that unit** (at the company / at that site).

## 5. Features

| ID | Feature | PRD |
|----|---------|-----|
| **F3.1** | Agent Placement (create & activate) | [agent-placement.md](prds/agent-placement.md) |
| **F3.2** | Placement Lifecycle & Status | [placement-lifecycle.md](prds/placement-lifecycle.md) |
| **F3.3** | Re-placement & Transfer (with history) | [replacement-transfer.md](prds/replacement-transfer.md) |
| **F3.4** | Shift-Leader Assignment | [shift-leader-assignment.md](prds/shift-leader-assignment.md) |
| **F3.5** | Company Placement Roster | [company-roster.md](prds/company-roster.md) |

---

### F3.1 — Agent Placement (create & activate)

HR admin places an agent at a client company, in a service line, for a contract period, referencing the agent's employment agreement (PKWT/PKWTT) and selecting the per-placement position. Compensation (base salary) and annual-leave entitlement are **employment-agreement (E2) terms, not placement terms** *(2026-06-07, EPICS §8)*. The placement starts as `Draft`, validates against the invariants, then activates on/after its start date.

```mermaid
flowchart TD
    subgraph HR[HR / Placement Admin]
        A1([Start: new placement]) --> A2[Select agent]
        A2 --> A3[Select client company + service line]
        A3 --> A4[Set period + position<br/>PKWT ref, dates]
        A4 --> A7[Submit placement]
        A6[Resolve existing:<br/>end or transfer] --> A4
    end
    subgraph SYS[System]
        A7 --> S1{Valid? dates, company active,<br/>no overlapping active placement}
        S1 -- Invalid: overlap --> S2[Block: agent already placed] --> A6
        S1 -- Invalid: data --> S3[Show field errors] --> A4
        S1 -- Valid --> S4[Create Placement = Draft]
        S4 --> S5{Start date <= today?}
        S5 -- Yes --> S6[Status = Active]
        S5 -- No --> S7[Status = Scheduled<br/>activate on start date]
        S6 --> S8[(Persist + audit log)]
        S7 --> S8
        S8 --> S9[Notify agent + company shift leader]
    end
    subgraph AG[Agent]
        S9 --> G1[Views active placement]
    end
    subgraph SL[Shift Leader]
        S9 --> L1[Agent appears in company roster]
    end
```

**Entities:** `Placement` (create), reads `Employee`, `ClientCompany`, `ServiceLine`. **Depends on:** E2 (master data).

---

### F3.2 — Placement Lifecycle & Status

Manages the placement state machine and the transitions HR admins trigger (renewal, termination, resignation) plus system-driven transitions (auto-activate on start date, flag **Expiring** at **30 days** before end — hardcoded). The system **never auto-ends** a placement: when an `Expiring` placement reaches its `end_date` with no HR decision it **stays `Expiring`** (grace) until HR explicitly acts via an **Inbox decision task** — **Continue** (renew) or **End**. **Renewal creates a linked successor placement** (a new record whose `predecessor_id` points to the old one); the prior placement is closed as `Superseded`. History is never edited in place. Placement-end here is a *work-designation* change only — it **never revokes the agent's login**; login revocation is employment-end only (E2 [F2.7](../E2-identity/prds/offboarding.md), INV-6 / OB-2).

```mermaid
stateDiagram-v2
    [*] --> Draft
    Draft --> Scheduled: start date in future
    Draft --> Active: start date today / immediate
    Scheduled --> Active: start date reached (system)
    Active --> Expiring: 30 days before end (system)
    Active --> Ended: HR End decision
    Active --> Terminated: HR ends early
    Active --> Resigned: agent resigns
    Active --> Superseded: renewed (successor created)
    Expiring --> Expiring: end_date passed, no HR decision (grace; login stays valid)
    Expiring --> Ended: HR End decision (no auto-end)
    Expiring --> Terminated: HR ends early
    Expiring --> Resigned: agent resigns
    Expiring --> Superseded: renewed (successor created)
    Ended --> [*]
    Terminated --> [*]
    Resigned --> [*]
    Superseded --> [*]
```

**Entities:** `Placement` (status, ended_reason, resign_at, `predecessor_id`). **Depends on:** F3.1.

---

### F3.3 — Re-placement & Transfer (with history)

Move an agent from one company/service line to another. Ends the current placement (reason = `Transferred`) and opens a new one, preserving the full chain so an agent's placement history is always queryable.

```mermaid
flowchart TD
    subgraph HR[HR / Placement Admin]
        T1([Start: transfer agent]) --> T2[Pick agent w/ active placement]
        T2 --> T3[Choose new company + service line + period]
        T3 --> T6[Confirm transfer]
    end
    subgraph SYS[System]
        T6 --> V1{New company has<br/>a shift leader?}
        V1 -- No --> V2[Warn: assign leader<br/>after transfer]
        V1 -- Yes --> V3[Proceed]
        V2 --> V3
        V3 --> V4[End current placement<br/>reason = Transferred]
        V4 --> V5[Create new placement = Active/Scheduled]
        V5 --> V6[(Persist both + link history + audit)]
        V6 --> V7[Notify agent, old leader, new leader]
    end
    subgraph SL[Shift Leaders]
        V7 --> L1[Old leader: agent leaves roster]
        V7 --> L2[New leader: agent joins roster]
    end
```

**Entities:** `Placement` (close + create, history link). **Depends on:** F3.1, F3.2.

---

### F3.4 — Shift-Leader Assignment

Designate exactly one shift leader per client company. The leader must be an agent actively placed at that company (INV-4). Reassignment ends the prior assignment.

```mermaid
flowchart TD
    subgraph HR[HR / Placement Admin]
        D1([Start: assign shift leader]) --> D2[Select client company]
        D2 --> D3[Pick agent placed at this company]
        D3 --> D6[Confirm assignment]
    end
    subgraph SYS[System]
        D6 --> C1{Candidate active at<br/>this company?}
        C1 -- No --> C2[Block: must be placed here] --> D3
        C1 -- Yes --> C3{Company already<br/>has a leader?}
        C3 -- Yes --> C4[End previous assignment]
        C3 -- No --> C5[Proceed]
        C4 --> C5
        C5 --> C6[Create ShiftLeaderAssignment]
        C6 --> C7[(Persist + audit log)]
        C7 --> C8[Grant shift-leader role scope = company]
        C8 --> C9[Notify new leader + agents]
    end
    subgraph SL[Shift Leader]
        C9 --> L1[Gains roster, approval & roster-mgmt access]
    end
```

**Entities:** `ShiftLeaderAssignment` (create/close), `Employee` role scope. **Depends on:** F3.1, E1 (RBAC).

---

### F3.5 — Company Placement Roster

A per-company view listing all agents placed there, their service line, status, period, and the company's shift leader — with filters and export. This is the HR admin's and shift leader's day-to-day view.

```mermaid
flowchart LR
    subgraph User[HR Admin / Shift Leader]
        R1([Open company]) --> R2[Apply filters:<br/>service line, status, period]
    end
    subgraph SYS[System]
        R2 --> Q1[Query active + historical placements]
        Q1 --> Q2[Resolve shift leader]
        Q2 --> Q3[Return roster + counts]
        Q3 --> R3[Render roster table]
        R3 --> R4{Export?}
        R4 -- Yes --> Q4[Generate Excel/PDF] --> R5[Download]
        R4 -- No --> R6[Done]
    end
```

**Entities:** reads `Placement`, `ShiftLeaderAssignment`. **Depends on:** F3.1, F3.4, E10 (export).

---

## 6. Cross-feature rules

- All state changes write an **audit log** entry (who, when, before/after) — see E1.
- **History is never destroyed:** ending/transferring a placement closes the record, never deletes it.
- Notifications (E10) fire on: placement activated, expiring soon, ended/terminated, transfer, shift-leader (re)assigned.

## 7. Decisions & open questions

**Resolved (2026-06-08 — shift-leader identity model shipped):**
- ✅ **Derived role/scope.** A shift leader is an Employee with an active `shift_leader_assignments` row (keyed by `employee_id`); the auth role + single-company scope are **derived at request time** by server middleware from that assignment, **not stored on `users`** — consistent with INV-2/3/4. Reassign/revoke is effective on the next request (no re-login). → [F3.4](prds/shift-leader-assignment.md) SL-10.
- ✅ **Single entry point** = the client-company detail **"Pemimpin Shift" tab** (E2 [F2.3](../E2-identity/prds/client-company-directory.md)); the placement-detail shift-leader card is **read-only** and links there, and the F3.5 roster "Ganti" action links to the same tab. → F3.4 SL-11.

**Resolved (2026-06-07 — comp/leave are E2 terms, EPICS §8):**
- ✅ **`annual_leave_entitlement` and `base_salary_ref` removed from `Placement`.** Under alih-daya law the employment relationship is SWP↔agent and a placement is only a work *designation*; base salary stays the single source on E2 `CompensationRecord`, and the annual leave entitlement moves to the **EmploymentAgreement** (`annual_leave_entitlement_days`, E2 [employment-agreement.md](../E2-identity/prds/employment-agreement.md)). E6 leave-quota already sources the entitlement from E2. **BR-9 (position is selected per placement) is unaffected.**

**Resolved (2026-06-06 — reconcile with E2 F2.7 offboarding):**
- ✅ **No auto-end of placement at expiry.** At `end_date` an `Expiring` placement **stays `Expiring`** (grace) until HR decides via an **Inbox task** — **Continue** (renew → successor / `Superseded`) or **End**. The state machine has no system-driven `Expiring/Active → Ended` transition. → F3.2.
- ✅ **Placement-end never revokes login.** Transfer / renewal / supersede / any placement close changes only the *work designation*; login revocation is **employment-end only** (E2 [F2.7](../E2-identity/prds/offboarding.md), INV-6 / OB-2).

**Resolved (2026-06-03 — client sites, EPICS §8):**
- ✅ **Placement targets a `Site`** (E2 F2.6), not just a company — `Placement.site_id` required (INV-5); every company has ≥1 site (auto "Main Site"). E5 geofence resolves from `placement.site`.
- ✅ **Shift-leader scope = configurable** via `ClientCompany.leader_scope` (`company` | `site`, default `company`). INV-2/3/4 now apply per **leadership unit** (company or site). `ShiftLeaderAssignment` gains a nullable `site_id`. → F3.4.

**Resolved (2026-05-29):**
- ✅ **INV-1** — one active placement per agent (no split/multi-site).
- ✅ **INV-3** — shift leader strictly 1:1 with a leadership unit *(2026-06-03: unit = company **or** site per `leader_scope`; was company-only)*.
- ✅ **Renewal** creates a **linked successor** placement (`predecessor_id`), never an in-place extension. → F3.2
- ✅ **Expiring threshold = 30 days**, hardcoded (no config yet).
- ✅ **Headcount targets** are **reporting only** (E10) — not modeled at placement level.
- ✅ **1-day buffer** between placements — no overlap, no same-day handover. → F3.1 / PRD BR-2
- ✅ **Position** comes from master data (E2) but is set **per placement**, so the same agent may hold a different position at a different company.
- ✅ **Backdating** allowed for **HR admin** (with reason + audit), not Super Admin only.
- ✅ **Employment agreement (PKWT/PKWTT)** is separate from placement and tied to SWP↔employee (E2); placement `end_date` may be open-ended (PKWTT) and, for PKWT, must sit within the agreement period.

**Resolved (round 2):**
- ✅ **Service line** → **manual classification** later, after SWP confirms (no inference for now). → [DATA-MAPPING.md](DATA-MAPPING.md) G-1.
- ✅ **Sub-companies** (`role=4`) → **not used by SWP; ignore**. ClientCompany = `companies.role=2` only. → G-6. *(Sites, added 2026-06-03 as E2 F2.6, are **net-new** — not migrated from `role=4`; see E9.)*
- ✅ **PKWT overrun** → **auto-cap** placement end to agreement end (PRD BR-1b).
- ✅ **Buffer** → next day after prior end is sufficient.

**Still open (data verification, deferred to E9):**
1. Confirm how legacy distinguishes PKWT vs PKWTT (likely `contract_status_id` / absence of `contract_end_at`). → DATA-MAPPING.md G-4.
