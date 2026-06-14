// Agent · Pengajuan (Requests hub).
// G0 EXCEPTION: comp/AgentMobileNav (gfptk) defines a "Pengajuan" tab, but brainstorm.pen ships
// no Pengajuan hub frame — so this is assembled from packages/ui primitives (precedent: the
// agent-web-access G0 exception, EPICS §8). It is a launcher into the designed submission flows
// (Ajukan/Cuti Saya/Lembur/Koreksi). Replace with the framed design when one is added to the .pen.
import { color } from '@swp/design-tokens';
import { useRouter } from 'expo-router';
import { CalendarPlus, ChevronRight, Plane, Timer } from 'lucide-react-native';
import type { ComponentType } from 'react';
import { useTranslation } from 'react-i18next';
import { Pressable, ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Text } from '../../src/ui/Text';

type Row = { icon: ComponentType<{ size?: number; color?: string }>; label: string; route: string };

function Section({ title, rows }: { title: string; rows: Row[] }) {
  const router = useRouter();
  return (
    <View className="gap-2">
      <Text variant="badge" weight="bold" className="tracking-[0.4px] text-text-3">
        {title}
      </Text>
      <View className="overflow-hidden rounded-card border border-border bg-surface">
        {rows.map((r, i) => (
          <Pressable
            key={r.route}
            onPress={() => router.push(r.route)}
            className={`flex-row items-center gap-3 px-4 py-3.5 ${i > 0 ? 'border-t border-border-soft' : ''}`}
          >
            <r.icon size={18} color={color.text2} />
            <Text variant="body" className="flex-1">
              {r.label}
            </Text>
            <ChevronRight size={18} color={color.text3} />
          </Pressable>
        ))}
      </View>
    </View>
  );
}

export default function Pengajuan() {
  const { t } = useTranslation();
  const insets = useSafeAreaInsets();

  return (
    <View className="flex-1 bg-app-bg">
      <View className="bg-surface px-4 pb-3.5" style={{ paddingTop: insets.top + 8 }}>
        <Text variant="screenTitle">{t('m:pengajuan.title')}</Text>
        <Text variant="caption" className="text-text-3">
          {t('m:pengajuan.subtitle')}
        </Text>
      </View>

      <ScrollView contentContainerStyle={{ padding: 16, gap: 16 }}>
        <Section
          title={t('m:pengajuan.secCuti')}
          rows={[
            { icon: CalendarPlus, label: t('m:pengajuan.ajukanCuti'), route: '/leave-new' },
            { icon: Plane, label: t('m:pengajuan.cutiSaya'), route: '/leave' },
          ]}
        />
        <Section
          title={t('m:pengajuan.secLembur')}
          rows={[
            { icon: CalendarPlus, label: t('m:pengajuan.ajukanLembur'), route: '/overtime-new' },
            { icon: Timer, label: t('m:pengajuan.lemburSaya'), route: '/overtime' },
          ]}
        />
      </ScrollView>
    </View>
  );
}
