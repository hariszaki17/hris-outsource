/**
 * E11 · Template Persetujuan (HR) — per-company approval-chain editor.
 *
 * .pen frames:
 *   d7tFAM  "E11 · Template Persetujuan (HR)"   — the chain editor (2–3 ordered OR-set lines)
 *   uoTwN   "E11 · Overlay — Konfirmasi Reset Pending" — the save-confirm modal
 *
 * Feature: F11.1 — Approval Template Management.
 * Business rules encoded:
 *   TM-1 (0 or 1 template per company) · TM-2 (2–3 ordered lines, line 3 optional, min 2 →
 *   block save) · TM-3 (≥1 active member/line; inactive/empty → 422 APPROVAL_LINE_INVALID,
 *   shown inline) · TM-4 (OR-set membership) · TM-5 (approvals.template.manage gate) ·
 *   TM-6 / INV-6 (saving bumps version + resets all pending instances to line 1 — the
 *   uoTwN confirm modal warns first) · TM-7 (delete → super-admin fallback).
 *   404 on load = no template yet = super-admin fallback → empty/create state, NOT an error.
 *
 * Defense-in-depth (ENGINEERING.md C1): authoring gated on `approvals.template.manage`;
 *   lacking it renders a read-only/no-permission state. The Go API is the real gate.
 *
 * i18n namespace: approvals (keys under `approvals.template.*`). All copy Bahasa (E4).
 *
 * Homing: the screen takes a `companyId` prop; integration wires the route under the Klien
 *   company detail (Settings tab). It does not edit router.tsx/nav.ts.
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import { ApiError } from '@swp/api-client';
import {
  type ApprovalLine,
  type ApprovalTemplate,
  useDeleteApprovalTemplate,
  useGetApprovalTemplate,
  useUpsertApprovalTemplate,
} from '@swp/api-client/e11';
import { Button, ConfirmDialog, StateView, useToast } from '@swp/ui';
import { useQueryClient } from '@tanstack/react-query';
import { ArrowDown, Check, Plus, RefreshCw, Trash2, TriangleAlert } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ApprovalTemplateLineCard, type LineMemberView } from './approval-template-line-card.tsx';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ApprovalTemplateEditorScreenProps {
  /** The client company whose approval chain is being edited (`SWP-CMP-…`). */
  companyId: string;
}

// ---------------------------------------------------------------------------
// Local editable model — a line is an ordered OR-set of member views.
// ---------------------------------------------------------------------------

interface EditableLine {
  members: LineMemberView[];
}

const MIN_LINES = 2;
const MAX_LINES = 3;

/** Seed the editor from a loaded template, or a fresh 2-line skeleton (create state). */
function linesFromTemplate(tpl: ApprovalTemplate | undefined): EditableLine[] {
  if (!tpl?.lines?.length) {
    return [{ members: [] }, { members: [] }];
  }
  return [...tpl.lines]
    .sort((a: ApprovalLine, b: ApprovalLine) => a.line_no - b.line_no)
    .map((line) => ({
      members: (line.members ?? []).map((m) => ({
        user_id: m.user_id,
        display_name: m.display_name,
        active: m.active,
      })),
    }));
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export default function ApprovalTemplateEditorScreen({
  companyId,
}: ApprovalTemplateEditorScreenProps) {
  const { t } = useTranslation('approvals');
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const currentUser = useCurrentUser();
  const canManage = currentUser?.permissions.includes('approvals.template.manage') ?? false;

  const templateQuery = useGetApprovalTemplate(companyId, {
    query: {
      // 404 = no template = super-admin fallback. Treat as a value, not a thrown error,
      // so we can render the create/empty state instead of an error surface.
      retry: (count, error) =>
        error instanceof ApiError && error.status === 404 ? false : count < 2,
    },
  });

  const upsert = useUpsertApprovalTemplate();
  const remove = useDeleteApprovalTemplate();

  // 404 → no template yet (super-admin fallback). Any other failure → error surface.
  const loadError =
    templateQuery.error instanceof ApiError && templateQuery.error.status !== 404
      ? templateQuery.error
      : null;
  // A 404 (no template, incl. AFTER a delete) is the fallback — even though TanStack Query
  // retains the previously-loaded `data` on a refetch error, so we must check the error too.
  const is404 = templateQuery.error instanceof ApiError && templateQuery.error.status === 404;
  const template =
    !is404 && templateQuery.data?.data && !('error' in templateQuery.data.data)
      ? (templateQuery.data.data as ApprovalTemplate)
      : undefined;
  const isFallback = !templateQuery.isLoading && !template && !loadError;

  // Editable state, (re)seeded whenever the loaded template changes.
  const [lines, setLines] = useState<EditableLine[]>(() => linesFromTemplate(template));
  // Field-level errors keyed by line index (from 422 APPROVAL_LINE_INVALID).
  const [lineErrors, setLineErrors] = useState<Record<number, string>>({});
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  // biome-ignore lint/correctness/useExhaustiveDependencies: intentionally re-seed only on the loaded template's identity/version change, not on every `template` object reference.
  useEffect(() => {
    setLines(linesFromTemplate(template));
    setLineErrors({});
  }, [template?.id, template?.version]);

  // --- Derived save-gate (TM-2 / TM-3) -------------------------------------
  const everyLineHasMember = lines.every((l) => l.members.length > 0);
  const enoughLines = lines.length >= MIN_LINES;
  const canSave = canManage && enoughLines && everyLineHasMember && !upsert.isPending;

  const blockReason = useMemo(() => {
    if (!enoughLines) return t('template.blockMinLines');
    if (!everyLineHasMember) return t('template.blockEmptyLine');
    return null;
  }, [enoughLines, everyLineHasMember, t]);

  // --- Mutations -----------------------------------------------------------
  function buildPayload() {
    return { lines: lines.map((l) => ({ members: l.members.map((m) => m.user_id) })) };
  }

  async function doSave() {
    try {
      await upsert.mutateAsync({ companyId, data: buildPayload() });
      setConfirmOpen(false);
      setLineErrors({});
      await queryClient.invalidateQueries();
      toast({ tone: 'success', title: t('template.saveSuccess') });
    } catch (err) {
      setConfirmOpen(false);
      handleWriteError(err);
    }
  }

  async function doDelete() {
    try {
      await remove.mutateAsync({ companyId });
      setDeleteOpen(false);
      await queryClient.invalidateQueries();
      toast({ tone: 'success', title: t('template.deleteSuccess') });
    } catch (err) {
      setDeleteOpen(false);
      handleWriteError(err);
    }
  }

  /** Route a write error: 422 line-invalid → inline per-line; everything else → toast. */
  function handleWriteError(err: unknown) {
    if (err instanceof ApiError && err.status === 422 && err.code === 'APPROVAL_LINE_INVALID') {
      const next: Record<number, string> = {};
      if (err.fields) {
        for (const [field, message] of Object.entries(err.fields)) {
          // Field path like "lines[0].members" / "lines.0" → extract the line index.
          const m = field.match(/lines?[.[](\d+)/);
          if (m) next[Number(m[1])] = message;
        }
      }
      // No field map → flag every member-bearing line generically.
      if (Object.keys(next).length === 0) {
        lines.forEach((l, i) => {
          if (l.members.length > 0) next[i] = t('template.errorInactiveLine');
        });
      }
      setLineErrors(next);
      toast({ tone: 'error', title: t('template.errorLineInvalid') });
      return;
    }
    const { message } = classifyError(err);
    toast({ tone: 'error', title: t('template.saveError'), description: message });
  }

  // --- Line/member editors -------------------------------------------------
  const addLine = () =>
    setLines((prev) => (prev.length < MAX_LINES ? [...prev, { members: [] }] : prev));
  const removeLine = (idx: number) => setLines((prev) => prev.filter((_, i) => i !== idx));
  const addMember = (idx: number, userId: string, displayName?: string) => {
    // A user may belong to at most ONE line per template (no reassignment across
    // lines). Reject + explain if already assigned anywhere in the chain.
    if (lines.some((l) => l.members.some((m) => m.user_id === userId))) {
      toast({ tone: 'error', title: t('template.memberAlreadyAssigned') });
      return;
    }
    setLines((prev) =>
      prev.map((l, i) =>
        i === idx
          ? { members: [...l.members, { user_id: userId, display_name: displayName, active: true }] }
          : l,
      ),
    );
  };
  const removeMember = (idx: number, userId: string) =>
    setLines((prev) =>
      prev.map((l, i) =>
        i === idx ? { members: l.members.filter((m) => m.user_id !== userId) } : l,
      ),
    );

  // -------------------------------------------------------------------------
  // Render states (no dead-flow: loading · error · no-permission · content)
  // -------------------------------------------------------------------------

  function TitleBand({ children }: { children?: React.ReactNode }) {
    return (
      <div className="flex items-start justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="font-bold text-3xl text-text">{t('template.title')}</h1>
          <p className="text-sm text-text-2">{t('template.subtitle')}</p>
        </div>
        {children}
      </div>
    );
  }

  if (templateQuery.isLoading) {
    return (
      <div className="flex flex-col gap-[18px]">
        <TitleBand />
        <StateView kind="loading" title={t('template.loading')} />
      </div>
    );
  }

  if (loadError) {
    return (
      <div className="flex flex-col gap-[18px]">
        <TitleBand />
        <StateView
          kind="error"
          title={t('template.loadErrorTitle')}
          description={t('template.loadErrorBody')}
          onRetry={() => templateQuery.refetch()}
          retryLabel={t('template.retry')}
        />
      </div>
    );
  }

  if (!canManage) {
    return (
      <div className="flex flex-col gap-[18px]">
        <TitleBand />
        <StateView
          kind="no-permission"
          title={t('template.noPermissionTitle')}
          description={t('template.noPermissionBody')}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[18px]">
      <TitleBand>
        <div className="flex items-center gap-2.5">
          {template && (
            <Button
              variant="secondary"
              size="sm"
              data-testid="template-delete"
              disabled={remove.isPending}
              onClick={() => setDeleteOpen(true)}
              className="text-bad-tx"
            >
              <Trash2 className="size-4" aria-hidden />
              {t('template.deleteAction')}
            </Button>
          )}
          <Button
            variant="primary"
            size="sm"
            data-testid="template-save"
            disabled={!canSave}
            onClick={() => setConfirmOpen(true)}
          >
            <Check className="size-4" aria-hidden />
            {t('template.saveAction')}
          </Button>
        </div>
      </TitleBand>

      {/* Fallback note — no template means the super-admin fallback approves (INV-7). */}
      {isFallback && (
        <div
          data-testid="template-fallback"
          className="flex items-start gap-2.5 rounded-[10px] border border-info-bd bg-info-bg px-3.5 py-3"
        >
          <TriangleAlert className="mt-px size-4 shrink-0 text-info-tx" aria-hidden />
          <p className="text-[13px] leading-[1.45] text-info-tx">{t('template.fallbackNote')}</p>
        </div>
      )}

      {/* Reset banner — the always-visible warning that saving re-bases pending (INV-6). */}
      <div className="flex items-start gap-2.5 rounded-[10px] border border-warn-bd bg-warn-bg px-3.5 py-3">
        <TriangleAlert className="mt-px size-4 shrink-0 text-warn-tx" aria-hidden />
        <p className="text-[13px] leading-[1.45] text-warn-tx">{t('template.resetBanner')}</p>
      </div>

      {/* Chain — ordered line cards with sequential connectors */}
      <div className="flex flex-col">
        {lines.map((line, idx) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: line order is the identity (line_no = idx+1; add/remove + errors keyed by position).
          <div key={idx} className="flex flex-col">
            <ApprovalTemplateLineCard
              lineNo={idx + 1}
              members={line.members}
              removable={idx >= MIN_LINES}
              disabled={upsert.isPending}
              error={lineErrors[idx]}
              onAddMember={(userId) => addMember(idx, userId)}
              onRemoveMember={(userId) => removeMember(idx, userId)}
              onRemoveLine={() => removeLine(idx)}
            />
            {idx < lines.length - 1 && (
              <div className="flex justify-center py-2">
                <span className="inline-flex items-center gap-1.5 rounded-full border border-border bg-app-bg px-2.5 py-1">
                  <ArrowDown className="size-3.5 text-text-2" aria-hidden />
                  <span className="font-semibold text-[12px] text-text-2">
                    {t('template.connector')}
                  </span>
                </span>
              </div>
            )}
          </div>
        ))}

        {/* Add-line row — enabled until MAX_LINES, then shows the maxed copy */}
        <div className="flex justify-center pt-2">
          {lines.length < MAX_LINES ? (
            <button
              type="button"
              data-testid="template-add-line"
              onClick={addLine}
              className="inline-flex items-center gap-2 rounded-[10px] border border-border bg-surface px-[18px] py-3 font-medium text-[13px] text-text-2 hover:bg-surface-2"
            >
              <Plus className="size-4 text-text-3" aria-hidden />
              {t('template.addLine')}
            </button>
          ) : (
            <span className="inline-flex items-center gap-2 rounded-[10px] border border-border bg-surface px-[18px] py-3 font-medium text-[13px] text-text-3">
              <Plus className="size-4 text-text-3" aria-hidden />
              {t('template.maxLines')}
            </span>
          )}
        </div>
      </div>

      {/* Block-save hint (TM-2 / TM-3) — explains why Save is disabled */}
      {blockReason && <output className="text-[12px] text-text-3">{blockReason}</output>}

      {/* Save confirm — reset-pending warning FIRST, then upsert (uoTwN) */}
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        icon={RefreshCw}
        tone="warn"
        size="lg"
        title={t('template.resetConfirmTitle')}
        description={t('template.resetConfirmBody')}
        cancelLabel={t('template.cancel')}
        confirmLabel={t('template.resetConfirmAction')}
        confirmTone="primary"
        loading={upsert.isPending}
        onConfirm={doSave}
        closeLabel={t('template.cancel')}
      />

      {/* Delete confirm — reverts to super-admin fallback (TM-7) */}
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        icon={Trash2}
        tone="danger"
        title={t('template.deleteConfirmTitle')}
        description={t('template.deleteConfirmBody')}
        cancelLabel={t('template.cancel')}
        confirmLabel={t('template.deleteConfirmAction')}
        confirmTone="danger"
        loading={remove.isPending}
        onConfirm={doDelete}
        closeLabel={t('template.cancel')}
      />
    </div>
  );
}
