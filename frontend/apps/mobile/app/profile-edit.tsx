// E2 · Ubah Profil — instant self-edit form (design frame n465cT, repurposed).
//
// EPICS §8 E11 (2026-06-14): profile change-requests REMOVED. Every agent-editable
// field is now applied INSTANTLY via PATCH /me/profile (useUpdateMyProfile,
// SelfProfileUpdate). There is no approval tier, no "menunggu persetujuan" state.
//
// Editable (instant): Telepon, Kontak Darurat (nama + telepon), Bank (nama bank +
//   no. rekening + nama pemilik), Alamat, Bahasa. Photo upload is available via the
//   SelfProfileUpdate.photo_object_key path but has no UI control yet (no .pen frame).
// Locked (read-only display): Nama Lengkap, NIK (HR-only / statutory).
//
// Spec refs: F2.x · EP-5 · SelfProfileUpdate (PATCH /me/profile).
// Routing: pushed from /profile (Ubah Profil button).
import {
  type AppLanguage,
  type Employee,
  useGetEmployee,
  useUpdateMyProfile,
} from '@swp/api-client/e2';
import { color } from '@swp/design-tokens';
import { useRouter } from 'expo-router';
import { ArrowLeft } from 'lucide-react-native';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Alert, Pressable, ScrollView, View } from 'react-native';
import { useSession } from '../src/providers/session';
import { Button } from '../src/ui/Button';
import { Screen } from '../src/ui/Screen';
import { Text } from '../src/ui/Text';
import { TextField } from '../src/ui/TextField';

// ─── helpers ────────────────────────────────────────────────────────────────

function maskNik(nik: string): string {
  if (nik.length <= 8) return nik;
  const dots = '•'.repeat(nik.length - 8);
  return `${nik.slice(0, 4)}${dots}${nik.slice(-4)}`;
}

/** Inline phone-uniqueness/format guard (E.164 +62…). Empty is allowed (no change). */
function phoneError(phone: string, t: (k: string) => string): string | null {
  const trimmed = phone.trim();
  if (trimmed.length === 0) return null;
  // E.164: leading + then 8–15 digits (D2 — phone is the unique login identifier).
  if (!/^\+\d{8,15}$/.test(trimmed)) return t('m:profileEdit.phoneInvalid');
  return null;
}

// ─── sub-components ─────────────────────────────────────────────────────────

/** Section label — all-caps, text-3 color (matches .pen group label). */
function SectionLabel({ label }: { label: string }) {
  return (
    <Text variant="badge" className="mb-3 uppercase tracking-wider text-text-3">
      {label}
    </Text>
  );
}

/** Read-only locked field (disabled look, with lock glyph in label). */
function LockedField({ label, value }: { label: string; value: string }) {
  return (
    <View className="gap-1.5">
      <View className="flex-row items-center gap-1">
        <Text variant="label" weight="semibold" className="text-text-2">
          {label}
        </Text>
        <Text variant="caption" className="text-text-3">
          {'🔒'}
        </Text>
      </View>
      <View className="rounded-input border border-border bg-surface-2 px-3.5 py-[13px]">
        <Text variant="body" className="text-text-3">
          {value || '—'}
        </Text>
      </View>
    </View>
  );
}

/** Language segmented toggle (id / en). */
function LanguageToggle({
  value,
  onChange,
  labelId,
  labelEn,
}: {
  value: AppLanguage;
  onChange: (v: AppLanguage) => void;
  labelId: string;
  labelEn: string;
}) {
  const opts: { key: AppLanguage; label: string }[] = [
    { key: 'id', label: labelId },
    { key: 'en', label: labelEn },
  ];
  return (
    <View className="flex-row gap-2">
      {opts.map((o) => {
        const active = o.key === value;
        return (
          <Pressable
            key={o.key}
            onPress={() => onChange(o.key)}
            className={`flex-1 items-center rounded-input border px-3 py-[11px] ${
              active ? 'border-primary bg-primary-soft' : 'border-border bg-surface'
            }`}
          >
            <Text
              variant="body"
              weight={active ? 'semibold' : 'regular'}
              className={active ? 'text-primary' : 'text-text-2'}
            >
              {o.label}
            </Text>
          </Pressable>
        );
      })}
    </View>
  );
}

// ─── screen ─────────────────────────────────────────────────────────────────

export default function ProfileEditScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { user } = useSession();
  const employeeId = user?.employee_id ?? '';

  const q = useGetEmployee(employeeId, { query: { enabled: !!employeeId } });
  const emp = q.data?.data as Employee | undefined;

  // Instant-tier → PATCH /me/profile (applies immediately, no approval).
  const updateProfile = useUpdateMyProfile();

  // Editable state — pre-filled from employee record
  const [phone, setPhone] = useState('');
  const [emergencyName, setEmergencyName] = useState('');
  const [emergencyPhone, setEmergencyPhone] = useState('');
  const [bankName, setBankName] = useState('');
  const [accountNumber, setAccountNumber] = useState('');
  const [accountHolderName, setAccountHolderName] = useState('');
  const [address, setAddress] = useState('');
  const [language, setLanguage] = useState<AppLanguage>('id');

  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (emp) {
      setPhone(emp.phone ?? '');
      setEmergencyName(emp.emergency_contact?.name ?? '');
      setEmergencyPhone(emp.emergency_contact?.phone ?? '');
      setBankName(emp.bank_account?.bank_name ?? '');
      setAccountNumber(emp.bank_account?.account_number ?? '');
      setAccountHolderName(emp.bank_account?.account_holder_name ?? '');
      setAddress(emp.address ?? '');
      setLanguage(emp.app_language ?? 'id');
    }
  }, [emp]);

  const phoneInvalid = useMemo(() => phoneError(phone, t), [phone, t]);

  async function onSubmit() {
    setErr(null);
    if (!emp) return;
    if (phoneInvalid) return setErr(phoneInvalid);

    // Build a SelfProfileUpdate body from only the fields that changed. Every field
    // applies instantly via PATCH /me/profile — no approval tier.
    const body: {
      phone?: string;
      emergency_contact?: { name?: string; phone?: string };
      bank_account?: {
        bank_name?: string;
        account_number?: string;
        account_holder_name?: string;
      };
      address?: string;
      app_language?: AppLanguage;
    } = {};

    if (phone !== (emp.phone ?? '')) body.phone = phone;

    const emergencyChanged =
      emergencyName !== (emp.emergency_contact?.name ?? '') ||
      emergencyPhone !== (emp.emergency_contact?.phone ?? '');
    if (emergencyChanged) {
      body.emergency_contact = { name: emergencyName, phone: emergencyPhone };
    }

    const bankChanged =
      bankName !== (emp.bank_account?.bank_name ?? '') ||
      accountNumber !== (emp.bank_account?.account_number ?? '') ||
      accountHolderName !== (emp.bank_account?.account_holder_name ?? '');
    if (bankChanged) {
      body.bank_account = {
        bank_name: bankName,
        account_number: accountNumber,
        account_holder_name: accountHolderName,
      };
    }

    if (address !== (emp.address ?? '')) body.address = address;
    if (language !== (emp.app_language ?? 'id')) body.app_language = language;

    if (Object.keys(body).length === 0) {
      return setErr(t('m:profileEdit.noChange'));
    }

    try {
      await updateProfile.mutateAsync({ data: body });
      Alert.alert(t('m:profileEdit.title'), t('m:profileEdit.saved'));
      router.back();
    } catch {
      // Phone uniqueness is server-enforced (D2) — surface it inline if the PATCH
      // rejects the phone, otherwise a generic error.
      setErr(body.phone ? t('m:profileEdit.phoneTaken') : t('m:profileEdit.error'));
    }
  }

  if (q.isLoading) {
    return (
      <Screen>
        <View className="flex-1 items-center justify-center">
          <ActivityIndicator />
        </View>
      </Screen>
    );
  }

  const nikDisplay = emp ? maskNik(emp.nik) : '';

  return (
    <Screen>
      {/* ── Page header (back + title) ── */}
      <View className="mb-4 flex-row items-center gap-3">
        <Button
          variant="ghost"
          label=""
          onPress={() => router.back()}
          className="w-auto px-0"
          icon={<ArrowLeft size={20} color={color.text} />}
        />
        <Text variant="section">{t('m:profileEdit.title')}</Text>
      </View>

      <ScrollView showsVerticalScrollIndicator={false} contentContainerStyle={{ gap: 16 }}>
        {/* ── Kontak ── */}
        <View className="gap-4">
          <SectionLabel label={t('m:profileEdit.sectionContact')} />
          <TextField
            testID="profile-phone"
            label={t('m:profileEdit.telepon')}
            value={phone}
            onChangeText={setPhone}
            keyboardType="phone-pad"
            autoCapitalize="none"
            invalid={!!phoneInvalid}
            error={phoneInvalid ?? undefined}
          />
          <TextField
            testID="profile-address"
            label={t('m:profileEdit.alamat')}
            value={address}
            onChangeText={setAddress}
            multiline
            autoCapitalize="sentences"
          />
        </View>

        {/* ── Kontak Darurat ── */}
        <View className="gap-4">
          <SectionLabel label={t('m:profileEdit.sectionEmergency')} />
          <TextField
            testID="profile-emergency-name"
            label={t('m:profileEdit.emergencyName')}
            value={emergencyName}
            onChangeText={setEmergencyName}
            autoCapitalize="words"
          />
          <TextField
            testID="profile-emergency-phone"
            label={t('m:profileEdit.emergencyPhone')}
            value={emergencyPhone}
            onChangeText={setEmergencyPhone}
            keyboardType="phone-pad"
            autoCapitalize="none"
          />
        </View>

        {/* ── Bank ── */}
        <View className="gap-4">
          <SectionLabel label={t('m:profileEdit.sectionBank')} />
          <TextField
            testID="profile-bank-name"
            label={t('m:profileEdit.namaBank')}
            value={bankName}
            onChangeText={setBankName}
            autoCapitalize="words"
          />
          <TextField
            testID="profile-account-number"
            label={t('m:profileEdit.noRekening')}
            value={accountNumber}
            onChangeText={setAccountNumber}
            keyboardType="numeric"
            autoCapitalize="none"
          />
          <TextField
            testID="profile-account-holder"
            label={t('m:profileEdit.namaPemilikRekening')}
            value={accountHolderName}
            onChangeText={setAccountHolderName}
            autoCapitalize="words"
          />
        </View>

        {/* ── Bahasa ── */}
        <View className="gap-3">
          <SectionLabel label={t('m:profileEdit.sectionLanguage')} />
          <LanguageToggle
            value={language}
            onChange={setLanguage}
            labelId={t('m:profileEdit.langId')}
            labelEn={t('m:profileEdit.langEn')}
          />
        </View>

        {/* ── Locked fields ── */}
        <View className="gap-4">
          <SectionLabel label={t('m:profileEdit.sectionLocked')} />
          <LockedField label={t('m:profileEdit.namaLengkap')} value={emp?.full_name ?? ''} />
          <LockedField label={t('m:profileEdit.nik')} value={nikDisplay} />
        </View>

        {err ? (
          <Text variant="caption" className="text-bad-text">
            {err}
          </Text>
        ) : null}

        {/* bottom padding */}
        <View className="h-4" />
      </ScrollView>

      {/* ── Sticky submit CTA ── */}
      <View className="pt-4">
        <Button
          testID="profile-save"
          label={t('m:profileEdit.save')}
          onPress={() => void onSubmit()}
          loading={updateProfile.isPending}
        />
      </View>
    </Screen>
  );
}
