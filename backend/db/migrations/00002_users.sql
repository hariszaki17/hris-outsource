-- +goose Up
-- Login credentials + role. The identity split from the legacy system
-- (users.id vs employees.id) is preserved: a user MAY link to an employee
-- record (employee_id), and a shift_leader carries their single company scope
-- (company_id) used for `scope: company` RBAC enforcement.
CREATE TABLE users (
    id            text PRIMARY KEY,                 -- SWP-USR-<N>
    email         text NOT NULL,
    password_hash text NOT NULL,                    -- argon2id encoded string
    role          text NOT NULL CHECK (role IN ('super_admin','hr_admin','shift_leader','agent')),
    employee_id   text,                             -- SWP-EMP-<N>, nullable
    company_id    text,                             -- SWP-CMP-<N>, set for shift_leader
    status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz                        -- soft-delete (CONVENTIONS §6)
);

-- Email is unique among non-deleted users (case-insensitive).
CREATE UNIQUE INDEX users_email_active_uq
    ON users (lower(email))
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS users;
