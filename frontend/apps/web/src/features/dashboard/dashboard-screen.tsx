import { DateText, IdChip, StatusBadge, toneForAttendance } from '@swp/ui';
import { useTranslation } from 'react-i18next';

/**
 * Placeholder dashboard demonstrating the design-system molecules wired to tokens.
 * Real dashboards (E10) consume generated TanStack Query hooks + the x-rbac permission map.
 */
export function DashboardScreen() {
  const { t } = useTranslation();
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="font-bold text-3xl text-text">{t('nav.dashboard')}</h1>
      </div>

      <section className="rounded-lg border border-border bg-surface p-5 shadow-card">
        <h2 className="font-bold text-text text-xl">Kehadiran hari ini</h2>
        <table className="mt-4 w-full text-sm">
          <thead>
            <tr className="border-border border-b text-left text-text-3 text-xs uppercase tracking-wide">
              <th className="pb-2 font-semibold">Agen</th>
              <th className="pb-2 font-semibold">ID</th>
              <th className="pb-2 font-semibold">Masuk</th>
              <th className="pb-2 font-semibold">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border-soft">
            {[
              { name: 'Rudi Wijaya', id: 'SWP-EMP-1042', at: '2026-06-03T01:02:00Z', s: 'HADIR' },
              {
                name: 'Dewi Lestari',
                id: 'SWP-EMP-1077',
                at: '2026-06-03T01:21:00Z',
                s: 'TERLAMBAT',
              },
              { name: 'Budi Santoso', id: 'SWP-EMP-1090', at: '2026-06-03T00:58:00Z', s: 'ABSEN' },
            ].map((r) => (
              <tr key={r.id}>
                <td className="py-2.5 font-medium text-text">{r.name}</td>
                <td className="py-2.5">
                  <IdChip id={r.id} />
                </td>
                <td className="py-2.5 text-text-2">
                  <DateText kind="instant" value={r.at} options={{ timeStyle: 'short' }} />
                </td>
                <td className="py-2.5">
                  <StatusBadge tone={toneForAttendance(r.s)}>{r.s}</StatusBadge>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  );
}
