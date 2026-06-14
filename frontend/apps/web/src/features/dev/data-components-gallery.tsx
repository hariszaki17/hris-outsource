import {
  type AuditEntry,
  AuditTrailDrawer,
  AuditTrailInline,
  AuditTrailViewer,
  Button,
  type Column,
  CursorPagination,
  DataTable,
  EmptyState,
  FilterSelect,
  IdChip,
  SearchField,
  SettingsSubnav,
  SettingsSubnavItem,
  StatCard,
  StatusBadge,
  Toggle,
} from '@swp/ui';
import {
  CircleCheck,
  KeyRound,
  LayoutDashboard,
  MoreVertical,
  ScrollText,
  Settings,
  UserX,
  Users,
  UsersRound,
} from 'lucide-react';
import { useState } from 'react';

/**
 * Dev-only gallery for the Phase-0 data & form components (DataTable, fields, StatCard,
 * SettingsSubnav, AuditTrail). NOT a product screen — visual review surface at
 * /dev/components-data.
 */
function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-4">
      <h2 className="font-bold text-text text-xl">{title}</h2>
      <div className="rounded-lg border border-border bg-surface p-6">{children}</div>
    </section>
  );
}

interface Employee {
  id: string;
  name: string;
  position: string;
  company: string;
  status: 'ok' | 'bad';
  statusLabel: string;
}

const EMPLOYEES: Employee[] = [
  {
    id: 'SWP-EMP-1042',
    name: 'Budi Santoso',
    position: 'Petugas Parkir',
    company: 'Plaza Senayan',
    status: 'ok',
    statusLabel: 'Aktif',
  },
  {
    id: 'SWP-EMP-1077',
    name: 'Ayu Lestari',
    position: 'Building Attendant',
    company: 'PT Graha Mandiri',
    status: 'ok',
    statusLabel: 'Aktif',
  },
  {
    id: 'SWP-EMP-1090',
    name: 'Citra Dewi',
    position: 'Cleaning Service',
    company: 'Menara BCA',
    status: 'ok',
    statusLabel: 'Aktif',
  },
  {
    id: 'SWP-EMP-1104',
    name: 'Hadi Saputra',
    position: 'Cleaning Service',
    company: '—',
    status: 'bad',
    statusLabel: 'Nonaktif',
  },
];

const COLUMNS: Column<Employee>[] = [
  {
    id: 'name',
    header: 'Karyawan',
    width: 240,
    cell: (r) => (
      <div className="flex flex-col">
        <span className="font-medium text-text">{r.name}</span>
        <IdChip id={r.id} />
      </div>
    ),
  },
  { id: 'position', header: 'Posisi', width: 180, cell: (r) => r.position },
  { id: 'company', header: 'Penempatan', cell: (r) => r.company },
  {
    id: 'status',
    header: 'Status',
    width: 120,
    cell: (r) => (
      <StatusBadge dot tone={r.status}>
        {r.statusLabel}
      </StatusBadge>
    ),
  },
];

const AUDIT: AuditEntry[] = [
  {
    id: 'a1',
    type: 'note',
    actor: 'Sari Hadi',
    verb: 'menambahkan catatan',
    time: '5 mnt lalu',
    comment: { tone: 'warn', text: 'Konfirmasi via WhatsApp; lampirkan SK perpanjangan.' },
  },
  { id: 'a2', type: 'approved', actor: 'Sari Hadi', verb: 'menyetujui', time: '1 jam lalu' },
  {
    id: 'a3',
    type: 'updated',
    actor: 'Ari Saputra',
    verb: 'mengubah shift_id',
    time: '3 jam lalu',
    diff: { field: 'shift_id', from: 'S-PAGI', to: 'S-MALAM' },
  },
  {
    id: 'a4',
    type: 'rejected',
    actor: 'Sari Hadi',
    verb: 'menolak bukti',
    time: 'Kemarin · 16:42',
    comment: { tone: 'bad', text: 'Bukti foto tidak jelas. Mohon upload ulang.' },
  },
  {
    id: 'a5',
    type: 'created',
    actor: 'Ari Saputra',
    verb: 'membuat penempatan',
    time: '5 hari lalu',
  },
];

export function DataComponentsGallery() {
  const [toggleA, setToggleA] = useState(true);
  const [toggleB, setToggleB] = useState(false);
  const [drawer, setDrawer] = useState(false);
  const [selected, setSelected] = useState<string[]>(['SWP-EMP-1077']);

  return (
    <div className="min-h-screen space-y-10 bg-app p-8">
      <header>
        <h1 className="font-bold text-3xl text-text">Phase 0 — Data &amp; form components</h1>
        <p className="mt-1 text-text-2">Visual review surface for the data/form batch.</p>
      </header>

      <Section title="StatCard (comp/StatCard lmwet)">
        <div className="grid grid-cols-4 gap-4">
          <StatCard
            label="Total Karyawan"
            value="128"
            sub="seluruh agen & staf"
            icon={Users}
            tone="brand"
          />
          <StatCard label="Aktif" value="119" sub="sedang bekerja" icon={CircleCheck} tone="ok" />
          <StatCard
            label="Nonaktif"
            value="9"
            sub="resign / selesai kontrak"
            icon={UserX}
            tone="bad"
          />
          <StatCard
            label="Tanpa Login"
            value="14"
            sub="belum diprovisi"
            icon={KeyRound}
            tone="warn"
          />
        </div>
      </Section>

      <Section title="Fields (SearchField vJBJZ · FilterSelect t60nEC · Toggle Uma0O)">
        <div className="flex flex-wrap items-center gap-4">
          <SearchField placeholder="Cari nama / NIK / NIP" className="w-72" />
          <FilterSelect defaultValue="all" aria-label="Posisi">
            <option value="all">Semua Posisi</option>
            <option value="parkir">Petugas Parkir</option>
          </FilterSelect>
          <FilterSelect defaultValue="all" aria-label="Status">
            <option value="all">Semua Status</option>
            <option value="aktif">Aktif</option>
          </FilterSelect>
          <span className="flex items-center gap-2 text-sm text-text-2">
            <Toggle checked={toggleA} onCheckedChange={setToggleA} aria-label="Toggle aktif" /> On
          </span>
          <span className="flex items-center gap-2 text-sm text-text-2">
            <Toggle checked={toggleB} onCheckedChange={setToggleB} aria-label="Toggle nonaktif" />{' '}
            Off
          </span>
        </div>
      </Section>

      <Section title="StatusBadge — with & without dot (StatusPill qxONU reconciled)">
        <div className="flex flex-wrap gap-3">
          <StatusBadge tone="ok">HADIR</StatusBadge>
          <StatusBadge dot tone="ok">
            Aktif
          </StatusBadge>
          <StatusBadge dot tone="bad">
            Nonaktif
          </StatusBadge>
          <StatusBadge dot tone="warn">
            Menunggu
          </StatusBadge>
          <StatusBadge dot tone="info">
            Terjadwal
          </StatusBadge>
        </div>
      </Section>

      <Section title="DataTable + CursorPagination (derived from E2 · Karyawan — Daftar WElYh)">
        <DataTable
          aria-label="Daftar karyawan"
          columns={COLUMNS}
          data={EMPLOYEES}
          getRowId={(r) => r.id}
          selectable
          selectedIds={selected}
          onSelectionChange={setSelected}
          onRowClick={() => undefined}
          rowActions={() => (
            <button
              type="button"
              aria-label="Aksi baris"
              className="flex size-[30px] items-center justify-center rounded-md hover:bg-surface-2"
            >
              <MoreVertical className="size-4 text-text-3" aria-hidden />
            </button>
          )}
          footer={
            <CursorPagination
              rangeLabel="Menampilkan 1–4 dari 128 karyawan"
              hasPrev={false}
              hasNext
              onPrev={() => undefined}
              onNext={() => undefined}
            />
          }
        />
        <div className="mt-6">
          <p className="mb-2 text-sm text-text-2">Loading & empty states:</p>
          <div className="grid grid-cols-2 gap-4">
            <DataTable
              aria-label="loading"
              columns={COLUMNS}
              data={[]}
              getRowId={(r) => r.id}
              isLoading
              skeletonRows={4}
            />
            <DataTable
              aria-label="empty"
              columns={COLUMNS}
              data={[]}
              getRowId={(r) => r.id}
              empty={
                <EmptyState
                  variant="filtered"
                  title="Tidak ada hasil"
                  description="Coba ubah filter."
                />
              }
            />
          </div>
        </div>
      </Section>

      <Section title="SettingsSubnav (comp/SettingsSubnav WhMQv)">
        <SettingsSubnav label="PENGATURAN">
          <SettingsSubnavItem icon={LayoutDashboard}>Ringkasan</SettingsSubnavItem>
          <SettingsSubnavItem icon={UsersRound} active>
            Pengguna &amp; Peran
          </SettingsSubnavItem>
          <SettingsSubnavItem icon={ScrollText}>Audit Log</SettingsSubnavItem>
          <SettingsSubnavItem icon={Settings}>Umum</SettingsSubnavItem>
        </SettingsSubnav>
      </Section>

      <Section title="AuditTrail (Viewer jzBi0 · Inline qtz6q · Drawer BUAHW)">
        <div className="flex flex-wrap items-start gap-6">
          <div className="w-[520px]">
            <AuditTrailViewer
              title="Riwayat audit"
              count="24 entri"
              entries={AUDIT}
              footer={
                <CursorPagination
                  rangeLabel="Menampilkan 1–5 dari 24"
                  hasPrev={false}
                  hasNext
                  onPrev={() => undefined}
                  onNext={() => undefined}
                />
              }
            />
          </div>
          <div className="w-[380px] space-y-4">
            <AuditTrailInline
              title="Riwayat audit"
              entries={AUDIT.slice(0, 4)}
              onViewAll={() => setDrawer(true)}
            />
            <Button variant="secondary" onClick={() => setDrawer(true)}>
              Open AuditTrailDrawer
            </Button>
          </div>
        </div>
        <AuditTrailDrawer
          open={drawer}
          onOpenChange={setDrawer}
          title="Riwayat audit"
          subtitle="Penempatan SWP-PL-04821 · Plaza Senayan"
          entries={AUDIT}
          count="24 entri"
          onExport={() => undefined}
        />
      </Section>
    </div>
  );
}
