/**
 * E8 · Payroll detail route composition — wires PayslipDetailScreen to the append-only HR
 * audit-note Drawer and the export entry. Kept out of router.tsx so the i18n/toast/state live
 * with the feature.
 *
 * Export: the detail "Ekspor" entry surfaces a queued toast and defers the full multi-step
 * export flow to the E10 Export-modal family (D5 — XLSX-only v1; the `.pen` `i1uLk` flow is
 * owned by E10). When E10 lands, swap the toast for the modal opener.
 */
import { useToast } from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AuditNoteDrawer } from './audit-note-drawer.tsx';
import { PayslipDetailScreen } from './payslip-detail-screen.tsx';

export function PayslipDetailRoute({ payslipId }: { payslipId: string }) {
  const { t } = useTranslation('payroll');
  const { toast } = useToast();
  const navigate = useNavigate();
  const [noteDrawerOpen, setNoteDrawerOpen] = useState(false);

  return (
    <>
      <PayslipDetailScreen
        payslipId={payslipId}
        onBack={() => navigate({ to: '/payroll' })}
        onAddNote={() => setNoteDrawerOpen(true)}
        onExportClick={() =>
          toast({
            tone: 'queued',
            title: t('export.queuedToastTitle'),
            description: t('export.confidentialLockNote'),
          })
        }
      />
      <AuditNoteDrawer
        payslipId={payslipId}
        open={noteDrawerOpen}
        onClose={() => setNoteDrawerOpen(false)}
      />
    </>
  );
}
