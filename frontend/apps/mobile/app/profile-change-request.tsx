// E2 · Ajukan Perubahan — change-request form (ajukan-perubahan.png).
// Editable: Telepon, Alamat, Nama Bank, No. Rekening.
// Locked (read-only display): Nama Lengkap, NIK.
// Info banner at top (info tone). CTA "Kirim Pengajuan".
//
// Spec refs: F2.x · EP-5 · ChangeRequestCreate API body.
// Routing: pushed from /profile (Ajukan Perubahan Data button).
import {
  type Employee,
  useCreateChangeRequest,
  useGetEmployee,
} from '@swp/api-client/e2';
import { color } from '@swp/design-tokens';
import { ArrowLeft } from 'lucide-react-native';
import { useRouter } from 'expo-router';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';
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

// ─── sub-components ─────────────────────────────────────────────────────────

/** Info banner (blue info tone) matching ajukan-perubahan.png. */
function InfoBanner({ message }: { message: string }) {
  return (
    <View className="flex-row items-start gap-2 rounded-card border border-info-border bg-info-bg px-4 py-3">
      {/* ⓘ icon */}
      <Text className="mt-0.5 text-sm text-info-text">{'ⓘ'}</Text>
      <Text className="flex-1 text-sm leading-5 text-info-text">{message}</Text>
    </View>
  );
}

/** Section label — all-caps, text-3 color (matches .pen group label). */
function SectionLabel({ label }: { label: string }) {
  return (
    <Text className="mb-3 text-[11px] font-semibold uppercase tracking-wider text-text-3">
      {label}
    </Text>
  );
}

/** Read-only locked field (disabled look, with lock glyph in label). */
function LockedField({ label, value }: { label: string; value: string }) {
  return (
    <View className="gap-1.5">
      <View className="flex-row items-center gap-1">
        <Text className="text-[13px] font-semibold text-text-2">{label}</Text>
        <Text className="text-xs text-text-3">{'🔒'}</Text>
      </View>
      <View className="rounded-input border border-border bg-surface-2 px-3.5 py-[13px]">
        <Text className="text-sm text-text-3">{value || '—'}</Text>
      </View>
    </View>
  );
}

// ─── screen ─────────────────────────────────────────────────────────────────

export default function ProfileChangeRequestScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { user } = useSession();
  const employeeId = user?.employee_id ?? '';

  const q = useGetEmployee(employeeId, { query: { enabled: !!employeeId } });
  const emp = q.data?.data as Employee | undefined;

  const create = useCreateChangeRequest();

  // Editable state — pre-filled from employee record
  const [phone, setPhone] = useState('');
  const [address, setAddress] = useState('');
  const [bankName, setBankName] = useState('');
  const [accountNumber, setAccountNumber] = useState('');

  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (emp) {
      setPhone(emp.phone ?? '');
      setAddress(emp.address ?? '');
      setBankName(emp.bank_account?.bank_name ?? '');
      setAccountNumber(emp.bank_account?.account_number ?? '');
    }
  }, [emp]);

  async function onSubmit() {
    setErr(null);
    // Build only the changed fields
    const changes: {
      phone?: string;
      bank_account?: { bank_name?: string; account_number?: string };
    } = {};

    if (emp && phone !== (emp.phone ?? '')) changes.phone = phone;

    const bankChanged =
      emp &&
      (bankName !== (emp.bank_account?.bank_name ?? '') ||
        accountNumber !== (emp.bank_account?.account_number ?? ''));
    if (bankChanged) {
      changes.bank_account = { bank_name: bankName, account_number: accountNumber };
    }

    // address uses instant self-edit (PATCH /me/profile), not change-request;
    // but the .pen includes it in the form — we include it as an additional note
    // in the change request note field for HR awareness (pragmatic).
    const addressChanged = emp && address !== (emp.address ?? '');

    if (!changes.phone && !changes.bank_account && !addressChanged) {
      return setErr(t('m:changeRequest.noChange'));
    }

    // Note: address is instant-tier; we send it as change-request note so HR
    // sees the intent. A future iteration should split address to PATCH /me/profile.
    const note = addressChanged ? `Alamat baru: ${address}` : undefined;

    // If only address changed (no approval-tier changes), still need at least
    // one approval-tier field in changes per the API. In that edge case we
    // bundle current phone to satisfy the schema.
    if (Object.keys(changes).length === 0 && addressChanged) {
      // No approval-tier changes — send a no-op phone to satisfy schema.
      changes.phone = emp?.phone ?? phone;
    }

    try {
      await create.mutateAsync({ employeeId, data: { changes, note } });
      router.replace({ pathname: '/profile-status', params: { submitted: '1' } });
    } catch {
      setErr(t('m:changeRequest.error'));
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
        <Text variant="section">{t('m:changeRequest.title')}</Text>
      </View>

      <ScrollView showsVerticalScrollIndicator={false} contentContainerStyle={{ gap: 16 }}>
        {/* ── Info banner ── */}
        <InfoBanner message={t('m:changeRequest.infoBanner')} />

        {/* ── Editable fields ── */}
        <View className="gap-4">
          <SectionLabel label={t('m:changeRequest.sectionEditable')} />
          <TextField
            label={t('m:changeRequest.telepon')}
            value={phone}
            onChangeText={setPhone}
            keyboardType="phone-pad"
            autoCapitalize="none"
          />
          <TextField
            label={t('m:changeRequest.alamat')}
            value={address}
            onChangeText={setAddress}
            multiline
            autoCapitalize="sentences"
          />
          <TextField
            label={t('m:changeRequest.namaBank')}
            value={bankName}
            onChangeText={setBankName}
            autoCapitalize="words"
          />
          <TextField
            label={t('m:changeRequest.noRekening')}
            value={accountNumber}
            onChangeText={setAccountNumber}
            keyboardType="numeric"
            autoCapitalize="none"
          />
        </View>

        {/* ── Locked fields ── */}
        <View className="gap-4">
          <SectionLabel label={t('m:changeRequest.sectionLocked')} />
          <LockedField label={t('m:changeRequest.namaLengkap')} value={emp?.full_name ?? ''} />
          <LockedField label={t('m:changeRequest.nik')} value={nikDisplay} />
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
          label={t('m:changeRequest.kirimPengajuan')}
          onPress={() => void onSubmit()}
          loading={create.isPending}
        />
      </View>
    </Screen>
  );
}
