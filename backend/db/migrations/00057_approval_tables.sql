-- +goose Up
-- E11 Approvals engine — core schema (FEATURE §4). One configurable approval
-- template per client company (INV-1): an ordered chain of 2–3 lines, each line a
-- multi-user OR-set; lines clear in sequence. Instances route a domain request
-- (leave/OT) through the live template; actions are the append-only decision trail
-- (INV-9). Enums are text + CHECK backstops (app owns the authoritative enum,
-- pinned to openapi). IDs use the per-prefix column-DEFAULT allocator (decision
-- [05-01], same swp_next_id() mechanism as leave_requests/overtime): INSERTs omit
-- id to let the DEFAULT fire, OR supply an explicit id (seed/test).

-- approval_templates (SWP-APT-*): 0..1 per company (INV-1, UNIQUE company_id).
CREATE TABLE approval_templates (
    id          text PRIMARY KEY DEFAULT ('SWP-APT-' || swp_next_id('APT')),
    company_id  text NOT NULL UNIQUE REFERENCES client_companies(id),   -- INV-1: one template per company
    version     integer NOT NULL DEFAULT 1,                              -- bumped on every edit (INV-6)
    created_by  text,                                                    -- SWP-USR-*
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- approval_lines (SWP-APL-*): ordered 2..3 lines per template; line_no 1..3 unique
-- within a template (INV-2: AND across lines).
CREATE TABLE approval_lines (
    id          text PRIMARY KEY DEFAULT ('SWP-APL-' || swp_next_id('APL')),
    template_id text NOT NULL REFERENCES approval_templates(id) ON DELETE CASCADE,
    line_no     integer NOT NULL CHECK (line_no >= 1 AND line_no <= 3),
    UNIQUE (template_id, line_no)
);

-- approval_line_members: OR-set of users on a line (INV-2). Composite PK; a user
-- appears at most once per line.
CREATE TABLE approval_line_members (
    line_id text NOT NULL REFERENCES approval_lines(id) ON DELETE CASCADE,
    user_id text NOT NULL REFERENCES users(id),
    PRIMARY KEY (line_id, user_id)
);

-- approval_instances (SWP-APV-*): one live run of the engine for one domain request.
-- template_id null = super-admin fallback (INV-7). current_line is 1-based; status
-- PENDING→APPROVED|REJECTED. One instance per (request_type, request_id).
CREATE TABLE approval_instances (
    id               text PRIMARY KEY DEFAULT ('SWP-APV-' || swp_next_id('APV')),
    request_type     text NOT NULL CHECK (request_type IN ('LEAVE','OVERTIME')),
    request_id       text NOT NULL,                                      -- FK into the domain table (loose, cross-epic)
    company_id       text,                                               -- SWP-CMP-* (denormalized for the inbox queue)
    template_id      text REFERENCES approval_templates(id),             -- NULL = fallback (INV-7)
    template_version integer,                                            -- chain version this instance is on
    current_line     integer NOT NULL DEFAULT 1,                         -- 1-based line being decided
    line_count       integer NOT NULL,                                   -- configured line count (1 for fallback)
    status           text NOT NULL DEFAULT 'PENDING'
                         CHECK (status IN ('PENDING','APPROVED','REJECTED')),
    requester_id     text,                                               -- SWP-EMP-* (no self-approval, INV-3)
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (request_type, request_id)                                    -- one instance per request
);

-- Inbox / queue load: per-company by status, newest first via keyset (created_at,id).
CREATE INDEX approval_instances_company_status_idx
    ON approval_instances (company_id, status);

-- approval_actions (SWP-APA-*): append-only decision trail (INV-9). One row per
-- decision; stamped with the template_version in force at action time.
CREATE TABLE approval_actions (
    id               text PRIMARY KEY DEFAULT ('SWP-APA-' || swp_next_id('APA')),
    instance_id      text NOT NULL REFERENCES approval_instances(id) ON DELETE CASCADE,
    line_no          integer NOT NULL,                                   -- line acted on
    template_version integer,                                            -- version in force at action time
    actor_user_id    text,                                               -- SWP-USR-*
    action           text NOT NULL CHECK (action IN ('APPROVE','REJECT','BYPASS')),
    reason           text,                                               -- required (app) for REJECT + BYPASS
    created_at       timestamptz NOT NULL DEFAULT now()
);

-- Timeline read: all actions for an instance in chronological order.
CREATE INDEX approval_actions_instance_idx
    ON approval_actions (instance_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS approval_actions;
DROP TABLE IF EXISTS approval_instances;
DROP TABLE IF EXISTS approval_line_members;
DROP TABLE IF EXISTS approval_lines;
DROP TABLE IF EXISTS approval_templates;
