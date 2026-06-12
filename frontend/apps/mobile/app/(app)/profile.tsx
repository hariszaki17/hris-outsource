// E2 · Profil Saya — read-only identity card (profil-saya.png, frame agent self-service).
// Agent sees: avatar chip (initials, avatar-neutral bg), name/role/NIP row, status badge;
// Data Pribadi section (locked fields), Kontak section ("Dapat diajukan" tag),
// Bank section ("Dapat diajukan" tag), CTA → Ajukan Perubahan Data.
//
// Spec refs: F2.x · EP-5 · design-system §2 (avatar-neutral token).
// IDs (NIP) render in font-mono (IBM Plex Mono) per DESIGN-SYSTEM.
import { type Employee, useGetEmployee } from '@swp/api-client/e2';
import { color } from '@swp/design-tokens';
import { useRouter } from 'expo-router';
import { Bell, ChevronRight, Clock, FileText } from 'lucide-react-native';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, ScrollView, View } from 'react-native';
import { useSession } from '../../src/providers/session';
import { Button } from '../../src/ui/Button';
import { Card } from '../../src/ui/Card';
import { Screen } from '../../src/ui/Screen';
import { Text } from '../../src/ui/Text';

// ─── helpers ────────────────────────────────────────────────────────────────

/** Derive initials from full name (max 2 chars). */
function initials(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

/** Mask NIK: show first 4 + dots + last 4 (e.g. 3174••••••••0007). */
function maskNik(nik: string): string {
  if (nik.length <= 8) return nik;
  const dots = '•'.repeat(nik.length - 8);
  return `${nik.slice(0, 4)}${dots}${nik.slice(-4)}`;
}

/** Format birth date + place. */
function birthLabel(place?: string, date?: string): string {
  if (!place && !date) return '—';
  if (!date) return place ?? '—';
  // date is ISO yyyy-MM-dd — format as "d MMM yyyy" in id-ID
  try {
    const d = new Date(date);
    const formatted = d.toLocaleDateString('id-ID', {
      day: 'numeric',
      month: 'long',
      year: 'numeric',
    });
    return place ? `${place}, ${formatted}` : formatted;
  } catch {
    return `${place ?? ''}, ${date}`.replace(/^, /, '');
  }
}

// ─── sub-components ─────────────────────────────────────────────────────────

/** Label above a read-only value. */
function FieldRow({
  label,
  value,
  locked,
  mono,
}: {
  label: string;
  value: string;
  locked?: boolean;
  mono?: boolean;
}) {
  return (
    <View className="gap-0.5">
      <View className="flex-row items-center gap-1">
        <Text variant="caption" className="text-text-3">
          {label}
        </Text>
        {locked ? (
          // lock icon — inline SVG-less fallback using a unicode lock glyph styled small
          <Text className="text-xs text-text-3">{'🔒' /* 🔒 */}</Text>
        ) : null}
      </View>
      <Text variant="body" className={`text-text font-semibold ${mono ? 'font-mono' : ''}`}>
        {value || '—'}
      </Text>
    </View>
  );
}

/** Section header with optional right badge. */
function SectionHeader({
  title,
  badge,
}: {
  title: string;
  badge?: React.ReactNode;
}) {
  return (
    <View className="mb-3 flex-row items-center justify-between">
      <Text variant="strong">{title}</Text>
      {badge ?? null}
    </View>
  );
}

/** "Dapat diajukan" pill badge (info tone). */
function CanRequestBadge({ label }: { label: string }) {
  return (
    <View className="flex-row items-center gap-1 rounded-pill border border-info-border bg-info-bg px-2 py-0.5">
      {/* pencil icon — unicode fallback */}
      <Text className="text-xs text-info-text">{'✏'}</Text>
      <Text className="text-xs font-semibold text-info-text">{label}</Text>
    </View>
  );
}

// ─── screen ─────────────────────────────────────────────────────────────────

export default function ProfileScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { user, signOut } = useSession();
  const employeeId = user?.employee_id ?? '';
  const q = useGetEmployee(employeeId, { query: { enabled: !!employeeId } });

  const emp = q.data?.data as Employee | undefined;

  const isActive = emp?.status === 'ACTIVE';
  const nipDisplay = emp?.nip ?? '';
  const nikDisplay = emp ? maskNik(emp.nik) : '';

  // Avatar initials
  const avatarText = emp ? initials(emp.full_name) : '';

  if (q.isLoading) {
    return (
      <Screen>
        <View className="flex-1 items-center justify-center">
          <ActivityIndicator />
        </View>
      </Screen>
    );
  }

  return (
    <Screen>
      {/* ── Page header ── */}
      <View className="mb-4 flex-row items-center justify-between">
        <Text variant="pageTitle">{t('m:profile.title')}</Text>
        <Pressable
          accessibilityLabel={t('m:tabs.notifications')}
          hitSlop={12}
          onPress={() => router.push('/notifications')}
          className="h-9 w-9 items-center justify-center"
        >
          <Bell size={22} color={color.text} />
        </Pressable>
      </View>

      <ScrollView showsVerticalScrollIndicator={false} contentContainerStyle={{ gap: 12 }}>
        {/* ── Identity card ── */}
        <Card>
          <View className="flex-row items-center justify-between">
            <View className="flex-row items-center gap-3">
              {/* Avatar chip */}
              <View className="h-11 w-11 items-center justify-center rounded-card bg-avatar-neutral">
                <Text className="text-sm font-semibold text-text-2">{avatarText}</Text>
              </View>
              {/* Name + role + NIP */}
              <View>
                <Text variant="strong">{emp?.full_name ?? '—'}</Text>
                <View className="flex-row items-center gap-1">
                  <Text variant="secondary">{t('m:profile.roleAgent')}</Text>
                  {nipDisplay ? (
                    <>
                      <Text variant="secondary" className="text-text-3">
                        {'·'}
                      </Text>
                      <Text variant="secondary" className="font-mono text-text-2">
                        {nipDisplay}
                      </Text>
                    </>
                  ) : null}
                </View>
              </View>
            </View>
            {/* Status badge */}
            <View
              className={`flex-row items-center gap-1 rounded-pill px-3 py-1 ${
                isActive ? 'border border-ok-border bg-ok-bg' : 'border border-bad-border bg-bad-bg'
              }`}
            >
              <View className={`h-2 w-2 rounded-full ${isActive ? 'bg-ok-text' : 'bg-bad-text'}`} />
              <Text
                className={`text-xs font-semibold ${isActive ? 'text-ok-text' : 'text-bad-text'}`}
              >
                {isActive ? t('m:profile.statusActive') : t('m:profile.statusInactive')}
              </Text>
            </View>
          </View>
        </Card>

        {/* ── Data Pribadi (locked) ── */}
        <Card>
          <SectionHeader title={t('m:profile.sectionDataPribadi')} />
          <View className="gap-4">
            <FieldRow label={t('m:profile.namaLengkap')} value={emp?.full_name ?? ''} locked />
            <FieldRow label={t('m:profile.nik')} value={nikDisplay} locked mono />
            <FieldRow
              label={t('m:profile.tempatTglLahir')}
              value={birthLabel(emp?.birth_place, emp?.birth_date)}
            />
          </View>
        </Card>

        {/* ── Kontak ── */}
        <Card>
          <SectionHeader
            title={t('m:profile.sectionKontak')}
            badge={<CanRequestBadge label={t('m:profile.dapatDiajukan')} />}
          />
          <View className="gap-4">
            <FieldRow label={t('m:profile.telepon')} value={emp?.phone ?? ''} />
            <FieldRow label={t('m:profile.email')} value={emp?.email_personal ?? ''} />
            <FieldRow label={t('m:profile.alamat')} value={emp?.address ?? ''} />
          </View>
        </Card>

        {/* ── Bank ── */}
        <Card>
          <SectionHeader
            title={t('m:profile.sectionBank')}
            badge={<CanRequestBadge label={t('m:profile.dapatDiajukan')} />}
          />
          <View className="gap-4">
            <FieldRow label={t('m:profile.namaBank')} value={emp?.bank_account?.bank_name ?? ''} />
            <FieldRow
              label={t('m:profile.noRekening')}
              value={emp?.bank_account?.account_number ?? ''}
            />
          </View>
        </Card>

        {/* ── CTA ── */}
        <Button
          label={t('m:profile.ajukanPerubahanData')}
          onPress={() => router.push('/profile-change-request')}
        />

        {/* ── Lainnya (overflow: payslip / lembur / settings) ── */}
        <Card>
          <SectionHeader title={t('m:profile.sectionLainnya')} />
          <View className="gap-1">
            <OverflowRow
              icon={<FileText size={18} color={color.text2} />}
              label={t('m:menu.payslip')}
              onPress={() => router.push('/payslip')}
            />
            <OverflowRow
              icon={<Clock size={18} color={color.text2} />}
              label={t('m:menu.overtime')}
              onPress={() => router.push('/overtime')}
            />
          </View>
        </Card>

        <Button variant="secondary" label={t('m:more.signOut')} onPress={() => void signOut()} />

        {/* bottom padding for scroll */}
        <View className="h-4" />
      </ScrollView>
    </Screen>
  );
}

function OverflowRow({
  icon,
  label,
  onPress,
}: {
  icon: ReactNode;
  label: string;
  onPress: () => void;
}) {
  return (
    <Pressable onPress={onPress} className="flex-row items-center gap-3 rounded-control py-2.5">
      {icon}
      <Text variant="body" className="flex-1">
        {label}
      </Text>
      <ChevronRight size={18} color={color.text3} />
    </Pressable>
  );
}
