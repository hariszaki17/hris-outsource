/**
 * STATEFUL E11 Approvals mock layer (hand-authored — NOT Orval output).
 *
 * The generated `*.msw.ts` handlers return random faker payloads with no memory between
 * requests, so they cannot exercise a real approve → advance → approve → APPROVED flow,
 * self-approval blocking, super-admin bypass, or live-template pending reset (INV-6). This
 * module replaces them with an in-memory store + handlers that mutate it, so Playwright e2e
 * (and the local dev experience) can drive the full approval lifecycle deterministically.
 *
 * WIRING (see mocks.ts): these handlers are placed BEFORE the generated ones (MSW = first
 * match wins) and the superseded generated arrays (getApprovalInstancesMock,
 * getApprovalTemplatesMock, the auth-login + auth-me handlers, and the listEmployees +
 * getLeaveRequest handlers) are excluded for the overridden routes only.
 *
 * The store is seeded at module load via `resetE11Store()`, so a page reload gives a fresh,
 * known state. `window.__resetE11` is exposed by apps/web for test cleanup between specs.
 *
 * NOTE: this module is hand-authored and intentionally NOT covered by `@ts-nocheck`; it is
 * typed against the generated E11/E1/E2 model types so drift from the spec fails tsc.
 */
import { HttpResponse, http } from 'msw';
import type { Employee } from './gen/e2/model/employee.ts';
import type { ListEmployees200 } from './gen/e2/model/listEmployees200.ts';
import type { LoginResponse } from './gen/e1/model/loginResponse.ts';
import type { MeResponse } from './gen/e1/model/meResponse.ts';
import type { Role } from './gen/e1/model/role.ts';
import type { LeaveRequest } from './gen/e6/model/leaveRequest.ts';
import type { ApprovalAction } from './gen/e11/model/approvalAction.ts';
import type { ApprovalInstance } from './gen/e11/model/approvalInstance.ts';
import type { ApprovalInstanceDetail } from './gen/e11/model/approvalInstanceDetail.ts';
import type { ApprovalLine } from './gen/e11/model/approvalLine.ts';
import type { ApprovalTemplate } from './gen/e11/model/approvalTemplate.ts';
import type { ApprovalTemplateUpsert } from './gen/e11/model/approvalTemplateUpsert.ts';
import type { LineMember } from './gen/e11/model/lineMember.ts';
import type { ListApprovalInstances200 } from './gen/e11/model/listApprovalInstances200.ts';
import { RequestType } from './gen/e11/model/requestType.ts';

// ---------------------------------------------------------------------------
// User directory — email → role + identity. Drives login + /me + the employee picker.
// ---------------------------------------------------------------------------

interface MockUser {
  id: string; // SWP-USR-…
  employeeId: string; // SWP-EMP-…
  email: string;
  fullName: string;
  role: Role;
  /** Free-text position; surfaced by the picker `current_position`. */
  position: string;
  nip: string;
  nik: string;
}

/**
 * The four canonical login users (email → role) plus extra searchable staff so a tester can
 * add MULTIPLE distinct members per template line. Any email NOT in this table logs in as
 * `hr_admin` (default) to keep pre-existing specs working — see `userForLogin`.
 */
const USERS: MockUser[] = [
  {
    id: 'SWP-USR-SUPER',
    employeeId: 'SWP-EMP-SUPER',
    email: 'superadmin@swp.test',
    fullName: 'Super Admin',
    role: 'super_admin',
    position: 'Super Admin',
    nip: '9001',
    nik: '3175010101800001',
  },
  {
    id: 'SWP-USR-HR',
    employeeId: 'SWP-EMP-HR',
    email: 'hradmin@swp.test',
    fullName: 'Sari Hadi',
    role: 'hr_admin',
    position: 'HR Admin',
    nip: '9002',
    nik: '3175010101850002',
  },
  {
    id: 'SWP-USR-LEADER',
    employeeId: 'SWP-EMP-LEADER',
    email: 'leader@swp.test',
    fullName: 'Rudi Wijaya',
    role: 'shift_leader',
    position: 'Koordinator Lokasi',
    nip: '9003',
    nik: '3175010101880003',
  },
  {
    id: 'SWP-USR-AGENT',
    employeeId: 'SWP-EMP-AGENT',
    email: 'agent@swp.test',
    fullName: 'Budi Santoso',
    role: 'agent',
    position: 'Petugas Parkir',
    nip: '9004',
    nik: '3175010101900004',
  },
  // Extra staff so the picker has more than the four canonical users to search/add. These
  // never log in via the seeded emails, so their `role` only seeds the directory; the E1
  // generated Role union has no `lead`, so this row is typed hr_admin (Dewi is still a
  // distinct, searchable approver candidate).
  {
    id: 'SWP-USR-LEAD',
    employeeId: 'SWP-EMP-LEAD',
    email: 'lead@swp.test',
    fullName: 'Dewi Lestari',
    role: 'hr_admin',
    position: 'Lead Operasional',
    nip: '9005',
    nik: '3175010101870005',
  },
  {
    id: 'SWP-USR-HR2',
    employeeId: 'SWP-EMP-HR2',
    email: 'hr2@swp.test',
    fullName: 'Citra Putri',
    role: 'hr_admin',
    position: 'HR Generalist',
    nip: '9006',
    nik: '3175010101890006',
  },
];

const USER_BY_ID = new Map(USERS.map((u) => [u.id, u]));
const USER_BY_EMAIL = new Map(USERS.map((u) => [u.email.toLowerCase(), u]));

/** Login resolution: known email → that user; anything else → hr_admin default. */
function userForLogin(identifier: string): MockUser {
  const byEmail = USER_BY_EMAIL.get(identifier.trim().toLowerCase());
  if (byEmail) return byEmail;
  // Default keeps existing specs (which use arbitrary identifiers) working as HR.
  return USER_BY_ID.get('SWP-USR-HR') as MockUser;
}

function displayName(userId: string): string {
  return USER_BY_ID.get(userId)?.fullName ?? userId;
}

function memberOf(userId: string): LineMember {
  return { user_id: userId, display_name: displayName(userId), active: true };
}

// ---------------------------------------------------------------------------
// In-memory store
// ---------------------------------------------------------------------------

interface StoredInstance {
  id: string;
  request_type: RequestType;
  request_id: string;
  company_id: string;
  template_id: string | null;
  template_version: number;
  current_line: number; // 1-based
  status: 'PENDING' | 'APPROVED' | 'REJECTED';
  requester_id: string;
  summary: string;
  /** Resolved chain (mirrors the template at current version) for the timeline. */
  lines: ApprovalLine[];
  /** Append-only decision trail (INV-9). */
  actions: ApprovalAction[];
  created_at: string;
  updated_at: string;
}

interface StoredTemplate {
  id: string;
  company_id: string;
  version: number;
  /** Ordered lines; line i = lineMemberIds[i] (OR-set of user ids). */
  lineMemberIds: string[][];
  created_by: string;
  created_at: string;
  updated_at: string;
}

interface E11Store {
  currentUser: { id: string; role: Role } | null;
  templates: Map<string, StoredTemplate>; // keyed by companyId
  instances: Map<string, StoredInstance>; // keyed by instance id
}

let store: E11Store = emptyStore();

function emptyStore(): E11Store {
  return { currentUser: null, templates: new Map(), instances: new Map() };
}

const NOW = '2026-06-14T03:00:00Z';
let actionSeq = 0;
function nextActionId(): string {
  actionSeq += 1;
  return `SWP-APA-${String(actionSeq).padStart(4, '0')}`;
}

function linesFromMemberIds(memberIds: string[][]): ApprovalLine[] {
  return memberIds.map((ids, i) => ({
    id: `LINE-${i + 1}`,
    line_no: i + 1,
    members: ids.map(memberOf),
  }));
}

// ---------------------------------------------------------------------------
// Seed — covers every e2e scenario. Called at module load + by resetE11Store().
// ---------------------------------------------------------------------------

export function resetE11Store(): void {
  store = emptyStore();
  actionSeq = 0;

  // --- Templates -----------------------------------------------------------
  // C1: 2-line template. line1 = leader OR hr; line2 = hr.
  store.templates.set('SWP-CMP-001', {
    id: 'SWP-APT-C1',
    company_id: 'SWP-CMP-001',
    version: 1,
    lineMemberIds: [
      ['SWP-USR-LEADER', 'SWP-USR-HR'],
      ['SWP-USR-HR'],
    ],
    created_by: 'SWP-USR-SUPER',
    created_at: NOW,
    updated_at: NOW,
  });
  // C2: 3-line template. line1 = leader; line2 = lead OR hr; line3 = super.
  store.templates.set('SWP-CMP-002', {
    id: 'SWP-APT-C2',
    company_id: 'SWP-CMP-002',
    version: 1,
    lineMemberIds: [
      ['SWP-USR-LEADER'],
      ['SWP-USR-LEAD', 'SWP-USR-HR'],
      ['SWP-USR-SUPER'],
    ],
    created_by: 'SWP-USR-SUPER',
    created_at: NOW,
    updated_at: NOW,
  });
  // SWP-CMP-999: intentionally has NO template → GET returns 404 (super-admin fallback).

  // --- Instances -----------------------------------------------------------
  const c1Lines = linesFromMemberIds(
    store.templates.get('SWP-CMP-001')?.lineMemberIds as string[][],
  );
  const c2Lines = linesFromMemberIds(
    store.templates.get('SWP-CMP-002')?.lineMemberIds as string[][],
  );

  // PEND1: full-approve path. leader approves line1 → advances to line2; hr approves line2 → APPROVED.
  store.instances.set('SWP-APV-PEND1', {
    id: 'SWP-APV-PEND1',
    request_type: RequestType.LEAVE,
    request_id: 'SWP-LR-PEND1',
    company_id: 'SWP-CMP-001',
    template_id: 'SWP-APT-C1',
    template_version: 1,
    current_line: 1,
    status: 'PENDING',
    requester_id: 'SWP-USR-AGENT',
    summary: 'Cuti Tahunan · Budi Santoso · 3 hari',
    lines: c1Lines,
    actions: [],
    created_at: NOW,
    updated_at: NOW,
  });

  // MID2: 3-line C2 with line1 already cleared (seeded APPROVE) → current_line 2 (mixed timeline).
  store.instances.set('SWP-APV-MID2', {
    id: 'SWP-APV-MID2',
    request_type: RequestType.OVERTIME,
    request_id: 'SWP-OT-MID2',
    company_id: 'SWP-CMP-002',
    template_id: 'SWP-APT-C2',
    template_version: 1,
    current_line: 2,
    status: 'PENDING',
    requester_id: 'SWP-USR-AGENT',
    summary: 'Lembur · Budi Santoso · 4 jam',
    lines: c2Lines,
    actions: [
      {
        id: nextActionId(),
        line_no: 1,
        template_version: 1,
        actor_user_id: 'SWP-USR-LEADER',
        actor_name: displayName('SWP-USR-LEADER'),
        action: 'APPROVE',
        reason: null,
        created_at: NOW,
      },
    ],
    created_at: NOW,
    updated_at: NOW,
  });

  // SELF: requester is leader, who is ALSO the sole line1 member → self-approval-forbidden path.
  store.instances.set('SWP-APV-SELF', {
    id: 'SWP-APV-SELF',
    request_type: RequestType.LEAVE,
    request_id: 'SWP-LR-SELF',
    company_id: 'SWP-CMP-002', // C2 line1 = [leader]
    template_id: 'SWP-APT-C2',
    template_version: 1,
    current_line: 1,
    status: 'PENDING',
    requester_id: 'SWP-USR-LEADER',
    summary: 'Cuti · Rudi Wijaya · 1 hari',
    lines: c2Lines,
    actions: [],
    created_at: NOW,
    updated_at: NOW,
  });

  // REJ: pending on C1 line1 for the reject path (leader or hr can reject).
  store.instances.set('SWP-APV-REJ', {
    id: 'SWP-APV-REJ',
    request_type: RequestType.LEAVE,
    request_id: 'SWP-LR-REJ',
    company_id: 'SWP-CMP-001',
    template_id: 'SWP-APT-C1',
    template_version: 1,
    current_line: 1,
    status: 'PENDING',
    requester_id: 'SWP-USR-AGENT',
    summary: 'Cuti Sakit · Budi Santoso · 2 hari',
    lines: c1Lines,
    actions: [],
    created_at: NOW,
    updated_at: NOW,
  });

  // BYP: pending for the super-admin bypass path.
  store.instances.set('SWP-APV-BYP', {
    id: 'SWP-APV-BYP',
    request_type: RequestType.OVERTIME,
    request_id: 'SWP-OT-BYP',
    company_id: 'SWP-CMP-001',
    template_id: 'SWP-APT-C1',
    template_version: 1,
    current_line: 1,
    status: 'PENDING',
    requester_id: 'SWP-USR-AGENT',
    summary: 'Lembur · Budi Santoso · 6 jam',
    lines: c1Lines,
    actions: [],
    created_at: NOW,
    updated_at: NOW,
  });

  // DONE: terminal APPROVED render.
  store.instances.set('SWP-APV-DONE', {
    id: 'SWP-APV-DONE',
    request_type: RequestType.LEAVE,
    request_id: 'SWP-LR-DONE',
    company_id: 'SWP-CMP-001',
    template_id: 'SWP-APT-C1',
    template_version: 1,
    current_line: 2,
    status: 'APPROVED',
    requester_id: 'SWP-USR-AGENT',
    summary: 'Cuti Tahunan · Budi Santoso · 1 hari',
    lines: c1Lines,
    actions: [
      {
        id: nextActionId(),
        line_no: 1,
        template_version: 1,
        actor_user_id: 'SWP-USR-LEADER',
        actor_name: displayName('SWP-USR-LEADER'),
        action: 'APPROVE',
        reason: null,
        created_at: NOW,
      },
      {
        id: nextActionId(),
        line_no: 2,
        template_version: 1,
        actor_user_id: 'SWP-USR-HR',
        actor_name: displayName('SWP-USR-HR'),
        action: 'APPROVE',
        reason: null,
        created_at: NOW,
      },
    ],
    created_at: NOW,
    updated_at: NOW,
  });

  // REJD: terminal REJECTED render.
  store.instances.set('SWP-APV-REJD', {
    id: 'SWP-APV-REJD',
    request_type: RequestType.LEAVE,
    request_id: 'SWP-LR-REJD',
    company_id: 'SWP-CMP-001',
    template_id: 'SWP-APT-C1',
    template_version: 1,
    current_line: 1,
    status: 'REJECTED',
    requester_id: 'SWP-USR-AGENT',
    summary: 'Cuti · Budi Santoso · 5 hari',
    lines: c1Lines,
    actions: [
      {
        id: nextActionId(),
        line_no: 1,
        template_version: 1,
        actor_user_id: 'SWP-USR-HR',
        actor_name: displayName('SWP-USR-HR'),
        action: 'REJECT',
        reason: 'Bentrok dengan jadwal puncak.',
        created_at: NOW,
      },
    ],
    created_at: NOW,
    updated_at: NOW,
  });
}

// Seed immediately so a fresh page load is deterministic.
resetE11Store();

/** Test/inspection seed accessor — returns the canonical instance ids and their purpose. */
export const E11_SEED = {
  companies: {
    C1: 'SWP-CMP-001', // 2-line template
    C2: 'SWP-CMP-002', // 3-line template
    NO_TEMPLATE: 'SWP-CMP-999', // 404 → super-admin fallback
  },
  instances: {
    PEND1: 'SWP-APV-PEND1',
    MID2: 'SWP-APV-MID2',
    SELF: 'SWP-APV-SELF',
    REJ: 'SWP-APV-REJ',
    BYP: 'SWP-APV-BYP',
    DONE: 'SWP-APV-DONE',
    REJD: 'SWP-APV-REJD',
  },
  users: {
    SUPER: 'superadmin@swp.test',
    HR: 'hradmin@swp.test',
    LEADER: 'leader@swp.test',
    AGENT: 'agent@swp.test',
  },
} as const;

// ---------------------------------------------------------------------------
// Serialization
// ---------------------------------------------------------------------------

function toApprovalInstance(i: StoredInstance): ApprovalInstance {
  return {
    id: i.id,
    request_type: i.request_type,
    request_id: i.request_id,
    company_id: i.company_id,
    template_id: i.template_id,
    template_version: i.template_version,
    current_line: i.current_line,
    line_count: i.lines.length,
    status: i.status,
    requester_id: i.requester_id,
    summary: i.summary,
    created_at: i.created_at,
    updated_at: i.updated_at,
  };
}

function toDetail(i: StoredInstance): ApprovalInstanceDetail {
  return { ...toApprovalInstance(i), lines: i.lines, actions: i.actions };
}

function toTemplate(t: StoredTemplate): ApprovalTemplate {
  return {
    id: t.id,
    company_id: t.company_id,
    version: t.version,
    lines: linesFromMemberIds(t.lineMemberIds),
    created_by: t.created_by,
    created_at: t.created_at,
    updated_at: t.updated_at,
  };
}

function currentLineMemberIds(i: StoredInstance): string[] {
  return i.lines[i.current_line - 1]?.members.map((m) => m.user_id) ?? [];
}

// ---------------------------------------------------------------------------
// Error helpers (CONVENTIONS §11 envelope: { error: { code, message, fields? } }).
// ---------------------------------------------------------------------------

function errorBody(code: string, message: string, fields?: Record<string, string>) {
  return { error: { code, message, ...(fields ? { fields } : {}) } };
}
function jsonError(
  status: number,
  code: string,
  message: string,
  fields?: Record<string, string>,
) {
  return HttpResponse.json(errorBody(code, message, fields), { status });
}

// ---------------------------------------------------------------------------
// Auth handlers (override) — login + /me, role keyed by email.
// ---------------------------------------------------------------------------

function meResponseFor(u: MockUser): MeResponse {
  // Shift leaders are company-scoped (to C1 here); everyone else global.
  const scope =
    u.role === 'shift_leader'
      ? { type: 'company' as const, company_id: 'SWP-CMP-001' }
      : { type: 'global' as const, company_id: null };
  return {
    id: u.id,
    phone: '+628110000000',
    email: u.email,
    role: u.role,
    status: 'ACTIVE',
    employee_id: u.employeeId,
    full_name: u.fullName,
    last_login_at: '2026-06-13T07:00:00Z',
    scope,
  };
}

const authHandlers = [
  http.post('*/auth/login', async ({ request }) => {
    let identifier = '';
    try {
      const body = (await request.json()) as { identifier?: string } | null;
      identifier = body?.identifier ?? '';
    } catch {
      // empty / non-JSON body → default user
    }
    const u = userForLogin(identifier);
    store.currentUser = { id: u.id, role: u.role };
    const res: LoginResponse = {
      access_token: `mock-token-${u.id}`,
      refresh_token: `mock-refresh-${u.id}`,
      token_type: 'Bearer',
      expires_in: 3600,
      user: meResponseFor(u),
    };
    return HttpResponse.json(res, { status: 200 });
  }),

  // /me — keep role consistent with the last login (so reload-restore matches the session).
  http.get('*/auth/me', () => {
    const cur = store.currentUser ? USER_BY_ID.get(store.currentUser.id) : undefined;
    const u = cur ?? (USER_BY_ID.get('SWP-USR-HR') as MockUser);
    return HttpResponse.json(meResponseFor(u), { status: 200 });
  }),
];

// ---------------------------------------------------------------------------
// Employees search (override) — line-member picker source.
// ---------------------------------------------------------------------------

function toEmployee(u: MockUser): Employee {
  return {
    id: u.employeeId,
    full_name: u.fullName,
    nip: u.nip,
    nik: u.nik,
    status: 'ACTIVE',
    join_at: '2024-01-01',
    phone: '+628110000000',
    // current_position is free-text (string | null) on the real Employee contract.
    current_position: u.position,
    // user_id links the Employee row to its login — the approval picker resolves approver ids
    // via this (members are SWP-USR-… ids, employees are SWP-EMP-… ids).
    user_id: u.id,
  } as Employee;
}

const employeesHandlers = [
  http.get('*/employees', ({ request }) => {
    const url = new URL(request.url);
    const q = (url.searchParams.get('q') ?? '').trim().toLowerCase();
    const matched = USERS.filter((u) => {
      if (!q) return true;
      return (
        u.fullName.toLowerCase().includes(q) ||
        u.nip.includes(q) ||
        u.nik.includes(q) ||
        u.position.toLowerCase().includes(q) ||
        u.id.toLowerCase().includes(q)
      );
    });
    const body: ListEmployees200 = {
      data: matched.map(toEmployee),
      next_cursor: null,
      has_more: false,
    };
    return HttpResponse.json(body, { status: 200 });
  }),
];

// ---------------------------------------------------------------------------
// Template handlers (override) — per-company, stateful.
// ---------------------------------------------------------------------------

function companyIdFromTemplateUrl(url: string): string {
  // …/client-companies/{companyId}/approval-template
  const m = url.match(/\/client-companies\/([^/]+)\/approval-template/);
  return m?.[1] ? decodeURIComponent(m[1]) : '';
}

/** Reset all non-terminal instances of a company to line 1 + drop their actions (INV-6). */
function resetPendingInstancesForCompany(companyId: string, newVersion: number): void {
  for (const inst of store.instances.values()) {
    if (inst.company_id !== companyId) continue;
    if (inst.status !== 'PENDING') continue;
    inst.current_line = 1;
    inst.actions = [];
    inst.template_version = newVersion;
    inst.lines = linesFromMemberIds(
      store.templates.get(companyId)?.lineMemberIds as string[][],
    );
    inst.updated_at = NOW;
  }
}

const templateHandlers = [
  http.get('*/client-companies/:companyId/approval-template', ({ request }) => {
    const companyId = companyIdFromTemplateUrl(request.url);
    const t = store.templates.get(companyId);
    if (!t) {
      // No template → super-admin fallback (INV-7). Surfaced to the FE as 404 NO_TEMPLATE.
      return jsonError(404, 'NO_TEMPLATE', 'No approval template for this company.');
    }
    return HttpResponse.json(toTemplate(t), { status: 200 });
  }),

  http.put('*/client-companies/:companyId/approval-template', async ({ request }) => {
    const companyId = companyIdFromTemplateUrl(request.url);
    let payload: ApprovalTemplateUpsert | null = null;
    try {
      payload = (await request.json()) as ApprovalTemplateUpsert;
    } catch {
      payload = null;
    }
    const lines = payload?.lines ?? [];
    if (lines.length < 2 || lines.length > 3) {
      return jsonError(
        400,
        'APPROVAL_LINE_INVALID',
        'A template must have 2 or 3 lines.',
      );
    }
    // Every line needs ≥1 non-empty member.
    const fields: Record<string, string> = {};
    lines.forEach((line, idx) => {
      const members = (line.members ?? []).filter((m) => m && m.trim() !== '');
      if (members.length === 0) {
        fields[`lines.${idx}.members`] = 'Setiap baris memerlukan minimal satu anggota.';
      }
    });
    if (Object.keys(fields).length > 0) {
      return jsonError(
        422,
        'APPROVAL_LINE_INVALID',
        'Each approval line needs at least one member.',
        fields,
      );
    }

    const existing = store.templates.get(companyId);
    const newVersion = (existing?.version ?? 0) + 1;
    const stored: StoredTemplate = {
      id: existing?.id ?? `SWP-APT-${companyId}`,
      company_id: companyId,
      version: newVersion,
      lineMemberIds: lines.map((l) => l.members.filter((m) => m && m.trim() !== '')),
      created_by: existing?.created_by ?? store.currentUser?.id ?? 'SWP-USR-SUPER',
      created_at: existing?.created_at ?? NOW,
      updated_at: NOW,
    };
    store.templates.set(companyId, stored);
    // Live template + pending reset (INV-6).
    resetPendingInstancesForCompany(companyId, newVersion);
    return HttpResponse.json(toTemplate(stored), { status: 200 });
  }),

  http.delete('*/client-companies/:companyId/approval-template', ({ request }) => {
    const companyId = companyIdFromTemplateUrl(request.url);
    store.templates.delete(companyId);
    return new HttpResponse(null, { status: 204 });
  }),
];

// ---------------------------------------------------------------------------
// Instance handlers (override) — list / detail / approve / reject / bypass.
// ---------------------------------------------------------------------------

function instanceIdFromActionUrl(url: string, action: string): string {
  // …/approval-instances/{id}:approve  (id may itself contain ':' is not expected here)
  const m = url.match(new RegExp(`/approval-instances/([^/]+?):${action}(?:\\?|$)`));
  return m?.[1] ? decodeURIComponent(m[1]) : '';
}

async function readReason(request: Request): Promise<string> {
  try {
    const body = (await request.json()) as { reason?: string; note?: string } | null;
    return (body?.reason ?? body?.note ?? '').trim();
  } catch {
    return '';
  }
}

const instanceHandlers = [
  // LIST — honor mine / request_type / status / company_id.
  http.get('*/approval-instances', ({ request }) => {
    const url = new URL(request.url);
    const mine = url.searchParams.get('mine') === 'true';
    const requestType = url.searchParams.get('request_type');
    const statusParam = url.searchParams.get('status');
    const companyId = url.searchParams.get('company_id');
    const cur = store.currentUser;

    let list = [...store.instances.values()];
    if (requestType) list = list.filter((i) => i.request_type === requestType);
    if (statusParam) list = list.filter((i) => i.status === statusParam);
    if (companyId) list = list.filter((i) => i.company_id === companyId);
    if (mine) {
      list = list.filter((i) => {
        if (i.status !== 'PENDING') return false;
        if (!cur) return false;
        if (i.requester_id === cur.id) return false; // self-approval excluded from inbox
        if (cur.role === 'super_admin') return true; // super-admin sees all pending (bypass)
        return currentLineMemberIds(i).includes(cur.id);
      });
    }
    list.sort((a, b) => a.id.localeCompare(b.id));
    const body: ListApprovalInstances200 = {
      data: list.map(toApprovalInstance),
      next_cursor: null,
      has_more: false,
    };
    return HttpResponse.json(body, { status: 200 });
  }),

  // DETAIL — matches GET /approval-instances/:id but NOT the :action POSTs (those are POST).
  http.get('*/approval-instances/:id', ({ params }) => {
    const id = decodeURIComponent(String(params.id));
    const inst = store.instances.get(id);
    if (!inst) {
      return jsonError(404, 'NOT_FOUND', 'Approval instance not found.');
    }
    return HttpResponse.json(toDetail(inst), { status: 200 });
  }),

  // APPROVE
  http.post(/\/approval-instances\/[^/]+:approve(?:\?|$)/, async ({ request }) => {
    const id = instanceIdFromActionUrl(request.url, 'approve');
    const inst = store.instances.get(id);
    if (!inst) return jsonError(404, 'NOT_FOUND', 'Approval instance not found.');
    const cur = store.currentUser;
    if (!cur) return jsonError(401, 'UNAUTHENTICATED', 'Not signed in.');

    if (inst.status !== 'PENDING') {
      return jsonError(409, 'LINE_ALREADY_CLEARED', 'This request is already resolved.');
    }
    if (inst.requester_id === cur.id) {
      return jsonError(
        403,
        'SELF_APPROVAL_FORBIDDEN',
        'You cannot approve your own request.',
      );
    }
    const members = currentLineMemberIds(inst);
    if (cur.role !== 'super_admin' && !members.includes(cur.id)) {
      return jsonError(403, 'FORBIDDEN', 'You are not an approver on the current line.');
    }

    const note = await readReason(request);
    inst.actions.push({
      id: nextActionId(),
      line_no: inst.current_line,
      template_version: inst.template_version,
      actor_user_id: cur.id,
      actor_name: displayName(cur.id),
      action: 'APPROVE',
      reason: note || null,
      created_at: NOW,
    });
    const isLastLine = inst.current_line >= inst.lines.length;
    if (isLastLine) {
      inst.status = 'APPROVED';
    } else {
      inst.current_line += 1;
    }
    inst.updated_at = NOW;
    return HttpResponse.json(toApprovalInstance(inst), { status: 200 });
  }),

  // REJECT
  http.post(/\/approval-instances\/[^/]+:reject(?:\?|$)/, async ({ request }) => {
    const id = instanceIdFromActionUrl(request.url, 'reject');
    const inst = store.instances.get(id);
    if (!inst) return jsonError(404, 'NOT_FOUND', 'Approval instance not found.');
    const cur = store.currentUser;
    if (!cur) return jsonError(401, 'UNAUTHENTICATED', 'Not signed in.');
    if (inst.status !== 'PENDING') {
      return jsonError(409, 'LINE_ALREADY_CLEARED', 'This request is already resolved.');
    }
    const reason = await readReason(request);
    if (!reason) {
      return jsonError(400, 'VALIDATION_ERROR', 'A reason is required to reject.', {
        reason: 'Alasan penolakan wajib diisi.',
      });
    }
    if (inst.requester_id === cur.id) {
      return jsonError(403, 'SELF_APPROVAL_FORBIDDEN', 'You cannot act on your own request.');
    }
    const members = currentLineMemberIds(inst);
    if (cur.role !== 'super_admin' && !members.includes(cur.id)) {
      return jsonError(403, 'FORBIDDEN', 'You are not an approver on the current line.');
    }
    inst.actions.push({
      id: nextActionId(),
      line_no: inst.current_line,
      template_version: inst.template_version,
      actor_user_id: cur.id,
      actor_name: displayName(cur.id),
      action: 'REJECT',
      reason,
      created_at: NOW,
    });
    inst.status = 'REJECTED'; // terminal
    inst.updated_at = NOW;
    return HttpResponse.json(toApprovalInstance(inst), { status: 200 });
  }),

  // BYPASS — super-admin only.
  http.post(/\/approval-instances\/[^/]+:bypass(?:\?|$)/, async ({ request }) => {
    const id = instanceIdFromActionUrl(request.url, 'bypass');
    const inst = store.instances.get(id);
    if (!inst) return jsonError(404, 'NOT_FOUND', 'Approval instance not found.');
    const cur = store.currentUser;
    if (!cur) return jsonError(401, 'UNAUTHENTICATED', 'Not signed in.');
    if (cur.role !== 'super_admin') {
      return jsonError(403, 'FORBIDDEN', 'Only a super admin can bypass approvals.');
    }
    if (inst.status !== 'PENDING') {
      return jsonError(409, 'LINE_ALREADY_CLEARED', 'This request is already resolved.');
    }
    const reason = await readReason(request);
    if (!reason) {
      return jsonError(400, 'VALIDATION_ERROR', 'A reason is required to bypass.', {
        reason: 'Alasan bypass wajib diisi.',
      });
    }
    inst.actions.push({
      id: nextActionId(),
      line_no: inst.current_line,
      template_version: inst.template_version,
      actor_user_id: cur.id,
      actor_name: displayName(cur.id),
      action: 'BYPASS',
      reason,
      created_at: NOW,
    });
    inst.status = 'APPROVED'; // force-approve
    inst.updated_at = NOW;
    return HttpResponse.json(toApprovalInstance(inst), { status: 200 });
  }),
];

// ---------------------------------------------------------------------------
// E6 linkage (override) — one seeded leave request carries approval_instance_id so the
// E6 → E11 wiring path is testable from the leave-request detail screen.
// ---------------------------------------------------------------------------

const leaveLinkHandlers = [
  http.get('*/leave-requests/:id', ({ params }) => {
    const id = decodeURIComponent(String(params.id));
    // Only intercept the seeded id; returning undefined passes through to the generated
    // handler (MSW v2 falls through to the next matching resolver).
    if (id !== 'SWP-LR-PEND1') {
      return;
    }
    const body: LeaveRequest = {
      id: 'SWP-LR-PEND1',
      employee_id: 'SWP-EMP-AGENT',
      employee_name: 'Budi Santoso',
      employee_company_id: 'SWP-CMP-001',
      employee_company_name: 'Plaza Senayan',
      leave_type_id: 'SWP-LT-001',
      leave_type_name: 'Cuti Tahunan',
      start_date: '2026-06-20',
      end_date: '2026-06-22',
      duration_days: 3,
      reason: 'Acara keluarga.',
      status: 'PENDING',
      approval_instance_id: 'SWP-APV-PEND1',
      backdated: false,
      clock_in_conflict: false,
      created_at: NOW,
      updated_at: NOW,
    } as unknown as LeaveRequest;
    return HttpResponse.json(body, { status: 200 });
  }),
];

// ---------------------------------------------------------------------------
// Public entry — ordered so the most specific/overriding handlers win first.
// ---------------------------------------------------------------------------

export function getE11StatefulHandlers() {
  return [
    ...authHandlers,
    ...employeesHandlers,
    ...templateHandlers,
    ...instanceHandlers,
    ...leaveLinkHandlers,
  ];
}
