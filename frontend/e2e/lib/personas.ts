/**
 * lib/personas.ts
 *
 * Deterministic test personas seeded by backend/cmd/seed.
 * Passwords MUST match the constants in backend/cmd/seed/seed.go.
 * Update here if the seed passwords ever change.
 *
 * Reference: e2e-harness-spec.md §Seeding, 01-CONTEXT.md §Seed data
 */

export interface Persona {
  /** Seeded email address (login credential). */
  email: string;
  /** Known plaintext password (matches argon2id hash in the DB). */
  password: string;
  /** RBAC role assigned to this persona. */
  role: 'super_admin' | 'hr_admin' | 'shift_leader' | 'agent';
  /** Client company name (shift_leader only). */
  companyName?: string;
}

/**
 * PERSONAS — the four deterministic test personas.
 *
 * Passwords match backend/cmd/seed/seed.go constants:
 *   PasswordHRAdmin     = "Pass1ng-Garuda!"
 *   PasswordShiftLeader = "Lead3r-Senayan!"
 *   PasswordSuperAdmin  = "Sup3r-Admin-2026!"
 *   PasswordAgent       = "Ag3nt-Budi-2026!"
 */
export const PERSONAS = {
  hrAdmin: {
    email: 'sari.hadi@swp.test',
    password: 'Pass1ng-Garuda!',
    role: 'hr_admin',
  },
  shiftLeader: {
    email: 'rudi.wijaya@swp.test',
    password: 'Lead3r-Senayan!',
    role: 'shift_leader',
    companyName: 'Plaza Senayan',
  },
  superAdmin: {
    email: 'super.admin@swp.test',
    password: 'Sup3r-Admin-2026!',
    role: 'super_admin',
  },
  agent: {
    email: 'agent.budi@swp.test',
    password: 'Ag3nt-Budi-2026!',
    role: 'agent',
  },
} as const satisfies Record<string, Persona>;

export type PersonaKey = keyof typeof PERSONAS;
