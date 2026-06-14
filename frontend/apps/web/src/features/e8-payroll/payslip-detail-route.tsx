/**
 * E8 · Payroll detail route composition — wires PayslipDetailScreen to the append-only HR
 * audit-note Drawer. Kept out of router.tsx so the i18n/state live with the feature.
 */
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { AuditNoteDrawer } from './audit-note-drawer.tsx';
import { PayslipDetailScreen } from './payslip-detail-screen.tsx';

export function PayslipDetailRoute({ payslipId }: { payslipId: string }) {
  const navigate = useNavigate();
  const [noteDrawerOpen, setNoteDrawerOpen] = useState(false);

  return (
    <>
      <PayslipDetailScreen
        payslipId={payslipId}
        onBack={() => navigate({ to: '/payroll' })}
        onAddNote={() => setNoteDrawerOpen(true)}
      />
      <AuditNoteDrawer
        payslipId={payslipId}
        open={noteDrawerOpen}
        onClose={() => setNoteDrawerOpen(false)}
      />
    </>
  );
}
