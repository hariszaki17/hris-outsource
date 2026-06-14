/**
 * Unit tests for the agent /me/akun "Ubah Profil" instant-edit payload (EPICS §8 E11, 2026-06-14).
 *
 * Scope: pure-logic assertions (no DOM render) — mirrors the e6 leave-quotas / e5 attendance
 * test pattern. The diff logic lives inline in `me-akun-screen.tsx`'s `onSave`; these tests
 * re-encode the SAME diff rules so they can't silently drift.
 *
 * Since the E2 profile change-request surface was removed, ALL editable fields — alamat, bahasa
 * aplikasi, foto, telepon, kontak darurat, rekening bank — are now applied INSTANTLY in a single
 * `PATCH /me/profile` (SelfProfileUpdate). A field is only included when it actually changed from
 * the current employee record. See docs/eng/AGENT-WEB-ACCESS.md + E2 employee-profile.md.
 */

import type { BankAccount, EmergencyContact, SelfProfileUpdate } from '@swp/api-client/e2';
import { describe, expect, it } from 'vitest';

// ---------------------------------------------------------------------------
// Mirrors me-akun-screen.tsx onSave() — single instant PATCH /me/profile payload
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

/** Instant PATCH payload — every changed field (all instant now, no approval tier). */
function buildPatch(emp: EmpSnapshot, f: FormState): SelfProfileUpdate {
  const patch: SelfProfileUpdate = {};
  if (f.address !== (emp.address ?? '')) patch.address = f.address;
  if (f.language !== (emp.app_language ?? 'id')) patch.app_language = f.language as 'id' | 'en';
  if (f.phone !== (emp.phone ?? '')) patch.phone = f.phone;
  if (
    f.ecName !== (emp.emergency_contact?.name ?? '') ||
    f.ecPhone !== (emp.emergency_contact?.phone ?? '')
  ) {
    patch.emergency_contact = { name: f.ecName, phone: f.ecPhone };
  }
  if (
    f.bankName !== (emp.bank_account?.bank_name ?? '') ||
    f.bankNumber !== (emp.bank_account?.account_number ?? '') ||
    f.bankHolder !== (emp.bank_account?.account_holder_name ?? '')
  ) {
    patch.bank_account = {
      bank_name: f.bankName,
      account_number: f.bankNumber,
      account_holder_name: f.bankHolder,
    };
  }
  if (f.photoObjectKey) patch.photo_object_key = f.photoObjectKey;
  return patch;
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
// 1. Diff — only changed fields land in the single instant payload
// ---------------------------------------------------------------------------

describe('Ubah Profil — single instant PATCH /me/profile payload', () => {
  it('no edits → empty payload (the "Tidak ada perubahan" path)', () => {
    expect(buildPatch(EMP, pristineForm(EMP))).toEqual({});
  });

  it('alamat is instant', () => {
    const f = { ...pristineForm(EMP), address: 'Jl. Baru No. 9' };
    expect(buildPatch(EMP, f)).toEqual({ address: 'Jl. Baru No. 9' });
  });

  it('bahasa aplikasi is instant', () => {
    const f = { ...pristineForm(EMP), language: 'en' };
    expect(buildPatch(EMP, f)).toEqual({ app_language: 'en' });
  });

  it('foto (object key from presigned PUT) is instant', () => {
    const f = { ...pristineForm(EMP), photoObjectKey: 'profile-photos/SWP-EMP-1/01J.jpg' };
    expect(buildPatch(EMP, f)).toEqual({
      photo_object_key: 'profile-photos/SWP-EMP-1/01J.jpg',
    });
  });

  it('telepon is now instant (was approval-tier)', () => {
    const f = { ...pristineForm(EMP), phone: '081299998888' };
    expect(buildPatch(EMP, f)).toEqual({ phone: '081299998888' });
  });

  it('kontak darurat is instant — emitted as a {name, phone} object', () => {
    const f = { ...pristineForm(EMP), ecName: 'Andi', ecPhone: '081333333333' };
    expect(buildPatch(EMP, f)).toEqual({
      emergency_contact: { name: 'Andi', phone: '081333333333' },
    });
  });

  it('changing only the emergency phone still re-sends the whole {name, phone} object', () => {
    const f = { ...pristineForm(EMP), ecPhone: '081344444444' };
    expect(buildPatch(EMP, f)).toEqual({
      emergency_contact: { name: 'Siti', phone: '081344444444' },
    });
  });

  it('rekening bank is instant — re-sends the full bank object', () => {
    const f = { ...pristineForm(EMP), bankNumber: '9999999999' };
    expect(buildPatch(EMP, f)).toEqual({
      bank_account: { bank_name: 'BCA', account_number: '9999999999', account_holder_name: 'Budi' },
    });
  });
});

// ---------------------------------------------------------------------------
// 2. Mixed edit — every field flows through ONE instant submit
// ---------------------------------------------------------------------------

describe('Ubah Profil — all fields in one instant save', () => {
  it('an alamat + telepon edit lands in a single payload', () => {
    const f = { ...pristineForm(EMP), address: 'Jl. Baru No. 9', phone: '081299998888' };
    expect(buildPatch(EMP, f)).toEqual({
      address: 'Jl. Baru No. 9',
      phone: '081299998888',
    });
  });

  it('editing every field at once produces one combined payload', () => {
    const f: FormState = {
      ...pristineForm(EMP),
      address: 'Jl. Baru No. 9',
      language: 'en',
      phone: '081299998888',
      ecName: 'Andi',
      ecPhone: '081333333333',
      bankNumber: '9999999999',
    };
    const patch = buildPatch(EMP, f);
    expect(Object.keys(patch).sort()).toEqual([
      'address',
      'app_language',
      'bank_account',
      'emergency_contact',
      'phone',
    ]);
  });
});
