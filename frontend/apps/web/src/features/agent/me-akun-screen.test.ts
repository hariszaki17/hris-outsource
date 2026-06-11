/**
 * Unit tests for the agent /me/akun "Ubah Profil" tiered field partitioning (Plan §E/§F).
 *
 * Scope: pure-logic assertions (no DOM render) — mirrors the e6 leave-quotas / e5 attendance
 * test pattern. The partitioning logic lives inline in `me-akun-screen.tsx`'s `onSave`; these
 * tests re-encode the SAME diff rules so the tier boundary can't silently drift:
 *
 *   - INSTANT tier (PATCH /me/profile, no approval): alamat, bahasa aplikasi, foto.
 *   - APPROVAL tier (POST change-request): telepon, kontak darurat, rekening bank.
 *
 * The two submits are independent: an agent may save only instant fields, only approval fields,
 * or both in one "Simpan". A field is only included when it actually changed from the current
 * employee record. See docs/eng/AGENT-WEB-ACCESS.md + E2 employee-profile.md.
 */

import type { BankAccount, EmergencyContact, SelfProfileUpdate } from '@swp/api-client/e2';
import { describe, expect, it } from 'vitest';

// ---------------------------------------------------------------------------
// Mirrors me-akun-screen.tsx onSave() — INSTANT diff (PATCH /me/profile)
// ---------------------------------------------------------------------------

interface EmpSnapshot {
  address?: string | null;
  app_language?: string | null;
  phone?: string | null;
  emergency_contact?: EmergencyContact | null;
  bank_account?: BankAccount | null;
}

interface FormState {
  address: string;
  language: string;
  photoObjectKey?: string; // present once a photo has been uploaded to the presigned PUT
  phone: string;
  ecName: string;
  ecPhone: string;
  bankName: string;
  bankNumber: string;
  bankHolder: string;
}

/** INSTANT tier payload — only changed instant fields (address / app_language / photo). */
function buildInstant(emp: EmpSnapshot, f: FormState): SelfProfileUpdate {
  const instant: SelfProfileUpdate = {};
  if (f.address !== (emp.address ?? '')) instant.address = f.address;
  if (f.language !== (emp.app_language ?? 'id')) instant.app_language = f.language as 'id' | 'en';
  if (f.photoObjectKey) instant.photo_object_key = f.photoObjectKey;
  return instant;
}

/** APPROVAL tier payload — only changed approval fields (phone / emergency_contact / bank). */
function buildChanges(
  emp: EmpSnapshot,
  f: FormState,
): { phone?: string; emergency_contact?: EmergencyContact; bank_account?: BankAccount } {
  const changes: {
    phone?: string;
    emergency_contact?: EmergencyContact;
    bank_account?: BankAccount;
  } = {};
  if (f.phone !== (emp.phone ?? '')) changes.phone = f.phone;
  if (
    f.ecName !== (emp.emergency_contact?.name ?? '') ||
    f.ecPhone !== (emp.emergency_contact?.phone ?? '')
  ) {
    changes.emergency_contact = { name: f.ecName, phone: f.ecPhone };
  }
  if (
    f.bankName !== (emp.bank_account?.bank_name ?? '') ||
    f.bankNumber !== (emp.bank_account?.account_number ?? '') ||
    f.bankHolder !== (emp.bank_account?.account_holder_name ?? '')
  ) {
    changes.bank_account = {
      bank_name: f.bankName,
      account_number: f.bankNumber,
      account_holder_name: f.bankHolder,
    };
  }
  return changes;
}

const EMP: EmpSnapshot = {
  address: 'Jl. Lama No. 1',
  app_language: 'id',
  phone: '081200000000',
  emergency_contact: { name: 'Siti', phone: '081211111111' },
  bank_account: { bank_name: 'BCA', account_number: '1234567890', account_holder_name: 'Budi' },
};

/** Form pre-filled to the current record (no edits) — the screen's initial modal state. */
function pristineForm(emp: EmpSnapshot): FormState {
  return {
    address: emp.address ?? '',
    language: emp.app_language ?? 'id',
    phone: emp.phone ?? '',
    ecName: emp.emergency_contact?.name ?? '',
    ecPhone: emp.emergency_contact?.phone ?? '',
    bankName: emp.bank_account?.bank_name ?? '',
    bankNumber: emp.bank_account?.account_number ?? '',
    bankHolder: emp.bank_account?.account_holder_name ?? '',
  };
}

// ---------------------------------------------------------------------------
// 1. Tier boundary — which fields land in which submit
// ---------------------------------------------------------------------------

describe('Ubah Profil tier partitioning — instant vs approval', () => {
  it('no edits → both payloads empty (the "Tidak ada perubahan" path)', () => {
    const f = pristineForm(EMP);
    expect(buildInstant(EMP, f)).toEqual({});
    expect(buildChanges(EMP, f)).toEqual({});
  });

  it('alamat is INSTANT — it never enters the change-request', () => {
    const f = { ...pristineForm(EMP), address: 'Jl. Baru No. 9' };
    expect(buildInstant(EMP, f)).toEqual({ address: 'Jl. Baru No. 9' });
    expect(buildChanges(EMP, f)).toEqual({});
  });

  it('bahasa aplikasi is INSTANT', () => {
    const f = { ...pristineForm(EMP), language: 'en' };
    expect(buildInstant(EMP, f)).toEqual({ app_language: 'en' });
    expect(buildChanges(EMP, f)).toEqual({});
  });

  it('foto (object key from presigned PUT) is INSTANT', () => {
    const f = { ...pristineForm(EMP), photoObjectKey: 'profile-photos/SWP-EMP-1/01J.jpg' };
    expect(buildInstant(EMP, f)).toEqual({
      photo_object_key: 'profile-photos/SWP-EMP-1/01J.jpg',
    });
    expect(buildChanges(EMP, f)).toEqual({});
  });

  it('telepon is APPROVAL — it never enters the instant PATCH', () => {
    const f = { ...pristineForm(EMP), phone: '081299998888' };
    expect(buildInstant(EMP, f)).toEqual({});
    expect(buildChanges(EMP, f)).toEqual({ phone: '081299998888' });
  });

  it('kontak darurat is APPROVAL — emitted as a {name, phone} object', () => {
    const f = { ...pristineForm(EMP), ecName: 'Andi', ecPhone: '081333333333' };
    expect(buildInstant(EMP, f)).toEqual({});
    expect(buildChanges(EMP, f)).toEqual({
      emergency_contact: { name: 'Andi', phone: '081333333333' },
    });
  });

  it('changing only the emergency phone still re-sends the whole {name, phone} object', () => {
    const f = { ...pristineForm(EMP), ecPhone: '081344444444' };
    expect(buildChanges(EMP, f)).toEqual({
      emergency_contact: { name: 'Siti', phone: '081344444444' },
    });
  });

  it('rekening bank is APPROVAL — re-sends the full bank object', () => {
    const f = { ...pristineForm(EMP), bankNumber: '9999999999' };
    expect(buildInstant(EMP, f)).toEqual({});
    expect(buildChanges(EMP, f)).toEqual({
      bank_account: { bank_name: 'BCA', account_number: '9999999999', account_holder_name: 'Budi' },
    });
  });
});

// ---------------------------------------------------------------------------
// 2. Mixed edit — instant applies immediately, approval queues, in one Simpan
// ---------------------------------------------------------------------------

describe('Ubah Profil — mixed instant + approval in one save', () => {
  it('an alamat + telepon edit splits across BOTH submits', () => {
    const f = { ...pristineForm(EMP), address: 'Jl. Baru No. 9', phone: '081299998888' };
    const instant = buildInstant(EMP, f);
    const changes = buildChanges(EMP, f);
    expect(instant).toEqual({ address: 'Jl. Baru No. 9' });
    expect(changes).toEqual({ phone: '081299998888' });
    // Mirrors onSave's result-toast branching: both non-empty → mixed-success.
    const instantApplied = Object.keys(instant).length > 0;
    const approvalSubmitted = Object.keys(changes).length > 0;
    expect(instantApplied && approvalSubmitted).toBe(true);
  });

  it('editing every approval field at once produces a MULTIPLE-style change set', () => {
    const f: FormState = {
      ...pristineForm(EMP),
      phone: '081299998888',
      ecName: 'Andi',
      ecPhone: '081333333333',
      bankNumber: '9999999999',
    };
    const changes = buildChanges(EMP, f);
    expect(Object.keys(changes).sort()).toEqual(['bank_account', 'emergency_contact', 'phone']);
    expect(buildInstant(EMP, f)).toEqual({});
  });
});
