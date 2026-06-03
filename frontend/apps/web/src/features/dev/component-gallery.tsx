import {
  Avatar,
  Banner,
  Breadcrumb,
  Button,
  ConfirmDialog,
  EmptyState,
  Sidebar,
  SidebarBrand,
  SidebarFooter,
  SidebarNavItem,
  SidebarSectionLabel,
  SidebarSpacer,
  Skeleton,
  SkeletonCard,
  SkeletonTableRow,
  Toast,
  Topbar,
  TopbarIconButton,
  TopbarSearch,
  TopbarUser,
  useToast,
} from '@swp/ui';
import {
  Bell,
  CalendarClock,
  ChartColumn,
  CheckCheck,
  ClipboardCheck,
  CornerUpLeft,
  LayoutDashboard,
  MapPin,
  Plane,
  Plus,
  Settings,
  Timer,
  TriangleAlert,
  Users,
} from 'lucide-react';
import { useState } from 'react';

/**
 * Dev-only component gallery (NOT a product screen). Renders every Phase-0 chrome/feedback
 * component family so the batch can be visually reviewed at /dev/components. Remove or gate
 * before production. Components are the contract; this page just exercises them.
 */
function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-4">
      <h2 className="font-bold text-text text-xl">{title}</h2>
      <div className="rounded-lg border border-border bg-surface p-6">{children}</div>
    </section>
  );
}

const NAV = [
  { icon: LayoutDashboard, label: 'Dashboard' },
  { icon: Users, label: 'Karyawan' },
  { icon: MapPin, label: 'Penempatan' },
  { icon: CalendarClock, label: 'Jadwal Shift' },
  { icon: ClipboardCheck, label: 'Kehadiran', active: true },
  { icon: Plane, label: 'Cuti' },
  { icon: Timer, label: 'Lembur' },
  { icon: ChartColumn, label: 'Laporan' },
];

export function ComponentGallery() {
  const { toast } = useToast();
  const [reject, setReject] = useState(false);
  const [bulk, setBulk] = useState(false);
  const [destructive, setDestructive] = useState(false);
  const [discard, setDiscard] = useState(false);

  return (
    <div className="min-h-screen space-y-10 bg-app p-8">
      <header>
        <h1 className="font-bold text-3xl text-text">Phase 0 — Chrome &amp; feedback components</h1>
        <p className="mt-1 text-text-2">
          Visual review surface for the design-system batch (packages/ui).
        </p>
      </header>

      <Section title="Sidebar (comp/Sidebar iCqTB)">
        <div className="h-[640px] w-60 overflow-hidden rounded-lg">
          <Sidebar>
            <SidebarBrand title="SWP HRIS" subtitle="Outsource Ops" />
            <SidebarSectionLabel>MENU</SidebarSectionLabel>
            {NAV.map((n) => (
              <SidebarNavItem key={n.label} icon={n.icon} active={n.active}>
                {n.label}
              </SidebarNavItem>
            ))}
            <SidebarSpacer />
            <SidebarFooter>
              <SidebarNavItem icon={Settings}>Pengaturan</SidebarNavItem>
            </SidebarFooter>
          </Sidebar>
        </div>
      </Section>

      <Section title="Topbar (comp/Topbar caFkE)">
        <div className="overflow-hidden rounded-lg border border-border">
          <Topbar
            left={
              <Breadcrumb
                onMenuClick={() => undefined}
                menuLabel="Menu"
                items={[{ label: 'Kehadiran' }, { label: 'Dashboard', current: true }]}
              />
            }
            right={
              <>
                <TopbarSearch placeholder="Cari agen, lokasi..." />
                <TopbarIconButton icon={Bell} label="Notifikasi" />
                <TopbarUser name="Rudi Wijaya" roleLabel="Shift Leader" initials="RW" />
              </>
            }
          />
        </div>
      </Section>

      <Section title="Avatar (comp/Avatar YVANc)">
        <div className="flex items-center gap-4">
          <Avatar initials="AS" />
          <Avatar initials="RW" size={34} tone="neutral" shape="circle" />
          <Avatar initials="DL" size={48} />
          <Avatar initials="BS" size={48} tone="neutral" />
        </div>
      </Section>

      <Section title="Toast (comp/Toast PtJHa + tones)">
        <div className="space-y-4">
          <div className="flex flex-wrap gap-3">
            <Button
              variant="secondary"
              onClick={() =>
                toast({ tone: 'success', title: 'Berhasil', description: 'Perubahan tersimpan.' })
              }
            >
              Success
            </Button>
            <Button
              variant="secondary"
              onClick={() =>
                toast({
                  tone: 'error',
                  title: 'Gagal',
                  description: 'Coba lagi atau hubungi admin.',
                })
              }
            >
              Error
            </Button>
            <Button
              variant="secondary"
              onClick={() =>
                toast({
                  tone: 'warn',
                  title: 'Perlu perhatian',
                  description: 'Kuota mendekati habis.',
                })
              }
            >
              Warn
            </Button>
            <Button
              variant="secondary"
              onClick={() =>
                toast({
                  tone: 'info',
                  title: 'Pembaruan tersedia',
                  description: 'Jadwal minggu ini diperbarui.',
                })
              }
            >
              Info
            </Button>
            <Button
              variant="secondary"
              onClick={() =>
                toast({
                  tone: 'queued',
                  title: 'Sedang diproses',
                  description: 'Ekspor sedang diproses…',
                  duration: Number.POSITIVE_INFINITY,
                })
              }
            >
              Queued
            </Button>
          </div>
          <div className="grid max-w-[720px] grid-cols-2 gap-3">
            <Toast
              tone="success"
              title="Berhasil"
              description="Perubahan tersimpan."
              onClose={() => undefined}
            />
            <Toast
              tone="error"
              title="Gagal"
              description="Coba lagi atau hubungi admin."
              onClose={() => undefined}
            />
            <Toast
              tone="warn"
              title="Perlu perhatian"
              description="Kuota mendekati habis."
              onClose={() => undefined}
            />
            <Toast
              tone="info"
              title="Pembaruan tersedia"
              description="Jadwal diperbarui."
              onClose={() => undefined}
            />
            <Toast
              tone="queued"
              title="Sedang diproses"
              description="Ekspor sedang diproses…"
              onClose={() => undefined}
            />
          </div>
        </div>
      </Section>

      <Section title="Skeleton (comp/Skeleton* jcW4k/e3rdpj/NmWCA/PRMOL)">
        <div className="space-y-6">
          <div className="flex items-center gap-4">
            <Skeleton circle className="size-9" />
            <div className="space-y-2">
              <Skeleton className="h-3 w-40" />
              <Skeleton className="h-2.5 w-24" />
            </div>
          </div>
          <SkeletonCard />
          <div className="overflow-hidden rounded-lg border border-border-soft">
            <SkeletonTableRow />
            <SkeletonTableRow />
            <SkeletonTableRow />
          </div>
        </div>
      </Section>

      <Section title="EmptyState (comp/Empty* WTymt/BNr4w/mrACi/MRbzz/iwcgE)">
        <div className="grid grid-cols-3 gap-6">
          <EmptyState
            variant="default"
            title="Belum ada data"
            description="Mulai dengan menambahkan item pertama Anda."
          />
          <EmptyState
            variant="filtered"
            title="Tidak ada hasil"
            description="Coba ubah kata kunci atau filter."
            action={
              <Button variant="ghost" size="sm">
                Hapus filter
              </Button>
            }
          />
          <EmptyState
            variant="fresh"
            title="Belum ada data"
            description="Mulai dengan menambahkan item pertama Anda."
            action={
              <Button size="sm">
                <Plus />
                Tambah
              </Button>
            }
          />
          <EmptyState
            variant="no-permission"
            title="Akses ditolak"
            description="Anda tidak memiliki izin untuk melihat halaman ini."
            hint="Hubungi admin untuk meminta akses."
          />
          <EmptyState
            variant="session-expired"
            title="Sesi berakhir"
            description="Silakan masuk kembali untuk melanjutkan."
            action={<Button size="sm">Masuk</Button>}
          />
        </div>
      </Section>

      <Section title="Modal / ConfirmDialog (comp/Modal* EnabP/r4KZl5/V4LG8/z0kH0b)">
        <div className="flex flex-wrap gap-3">
          <Button variant="secondary" onClick={() => setReject(true)}>
            Reject
          </Button>
          <Button variant="secondary" onClick={() => setBulk(true)}>
            Bulk approve
          </Button>
          <Button variant="secondary" onClick={() => setDestructive(true)}>
            Destructive
          </Button>
          <Button variant="secondary" onClick={() => setDiscard(true)}>
            Discard changes
          </Button>
        </div>

        <ConfirmDialog
          open={reject}
          onOpenChange={setReject}
          size="lg"
          icon={CornerUpLeft}
          tone="danger"
          title="Tolak permintaan"
          description="Berikan alasan agar pengaju memahami keputusan."
          cancelLabel="Batal"
          confirmLabel="Tolak"
          confirmTone="danger"
          onConfirm={() => setReject(false)}
        >
          <textarea
            className="h-24 w-full rounded-md border border-border p-3 text-sm outline-none"
            placeholder="Tuliskan alasan…"
          />
        </ConfirmDialog>

        <ConfirmDialog
          open={bulk}
          onOpenChange={setBulk}
          icon={CheckCheck}
          tone="brand"
          title="Setujui beberapa item"
          description="10 item akan disetujui."
          cancelLabel="Batal"
          confirmLabel="Setujui 10 item"
          confirmTone="primary"
          onConfirm={() => setBulk(false)}
        />

        <ConfirmDialog
          open={destructive}
          onOpenChange={setDestructive}
          icon={TriangleAlert}
          tone="danger"
          title="Hapus item?"
          description="Item akan dihapus permanen dari sistem. Data terkait tidak dapat dipulihkan."
          cancelLabel="Batal"
          confirmLabel="Hapus"
          confirmTone="danger"
          onConfirm={() => setDestructive(false)}
        />

        <ConfirmDialog
          open={discard}
          onOpenChange={setDiscard}
          size="sm"
          icon={TriangleAlert}
          tone="warn"
          title="Buang perubahan?"
          description="Anda memiliki perubahan yang belum disimpan. Buang perubahan?"
          cancelLabel="Tetap di sini"
          confirmLabel="Buang perubahan"
          confirmTone="danger"
          onConfirm={() => setDiscard(false)}
        />
      </Section>

      <Section title="Banner (existing — sanity)">
        <Banner
          tone="bad"
          title="Email atau kata sandi salah"
          description="Periksa kembali kredensial Anda."
        />
      </Section>
    </div>
  );
}
