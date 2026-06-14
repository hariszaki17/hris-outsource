/**
 * E11 · Template Persetujuan — LineCard (feature organism)
 *
 * .pen frame: d7tFAM "E11 · Template Persetujuan (HR)" — the per-line card with an
 *   OR-set of member chips + an "Tambah anggota" add control + the OR note.
 *
 * One ordered approval line (`Baris N`): a removable OR-set of approver users. Any one
 * member clears the line at execution (F11.1 TM-4). Composes @swp/ui primitives + the
 * shared EmployeePicker (the member multi-select reuses the single-select picker, one add
 * at a time, mirroring the .pen "Tambah anggota" chip flow).
 *
 * Domain-specific (knows about approval lines/members) → stays a feature organism, never
 * promoted to packages/ui (ENGINEERING.md G2).
 *
 * Traceability: F11.1 · TM-2 (line_no 1..3, line 3 optional) · TM-3 (≥1 member) ·
 *   TM-4 (OR-set) · C-1 (sole-member warn).
 * i18n namespace: approvals (keys under `approvals.template.*`).
 */

import {
  EmployeePicker,
  resolveEmployeeName,
} from '@/features/e2-identity/pickers/employee-picker.tsx';
import { Avatar, Button, StatusBadge } from '@swp/ui';
import { Plus, Split, Trash2, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Member display model — the screen resolves user_id → display_name/active off the
// loaded template's LineMember[]; new picks carry only the id until reload.
// ---------------------------------------------------------------------------

export interface LineMemberView {
  user_id: string;
  /** Resolved display name (from LineMember.display_name). Falls back to the id. */
  display_name?: string;
  /** Whether the member is an active SWP user. `false` → flagged inline (TM-3). */
  active?: boolean;
}

export interface ApprovalTemplateLineCardProps {
  /** 1-based line position. */
  lineNo: number;
  members: LineMemberView[];
  /** Line is removable (only the optional 3rd line). */
  removable: boolean;
  /** Authoring disabled (read-only / saving). */
  disabled?: boolean;
  /** Field-level error for this line (e.g. empty line, inactive member — 422). */
  error?: string;
  onAddMember: (userId: string, displayName?: string) => void;
  onRemoveMember: (userId: string) => void;
  onRemoveLine: () => void;
}

function initialsOf(label: string): string {
  const words = label.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return '?';
  return words
    .slice(0, 2)
    .map((w) => w[0] ?? '')
    .join('')
    .toUpperCase();
}

export function ApprovalTemplateLineCard({
  lineNo,
  members,
  removable,
  disabled = false,
  error,
  onAddMember,
  onRemoveMember,
  onRemoveLine,
}: ApprovalTemplateLineCardProps) {
  const { t } = useTranslation('approvals');
  const [adding, setAdding] = useState(false);

  const isEmpty = members.length === 0;
  const hasInactive = members.some((m) => m.active === false);

  return (
    <div
      data-testid={`line-card-${lineNo}`}
      className="flex flex-col gap-3 rounded-xl border border-border bg-surface p-[18px]"
    >
      {/* Head — ordinal badge + requirement word, optional remove control */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2.5">
          <span className="rounded-full bg-primary-soft px-2 py-0.5 font-bold text-[11px] text-primary-strong tracking-[0.4px]">
            {t('template.lineBadge', { n: lineNo })}
          </span>
          <span
            className={
              removable
                ? 'font-semibold text-[12px] text-text-3'
                : 'font-semibold text-[12px] text-text-2'
            }
          >
            {removable ? t('template.optional') : t('template.required')}
          </span>
        </div>

        {removable && (
          <Button
            variant="ghost"
            size="sm"
            data-testid={`line-remove-${lineNo}`}
            disabled={disabled}
            onClick={onRemoveLine}
            className="text-bad-tx hover:bg-bad-bg"
          >
            <Trash2 className="size-3.5" aria-hidden />
            {t('template.removeLine')}
          </Button>
        )}
      </div>

      {/* Members — OR-set of chips + add control */}
      <div className="flex flex-wrap items-center gap-2">
        {members.map((m) => {
          const label = m.display_name ?? resolveEmployeeName(m.user_id) ?? m.user_id;
          const inactive = m.active === false;
          return (
            <span
              key={m.user_id}
              data-testid={`member-chip-${m.user_id}`}
              className={
                inactive
                  ? 'inline-flex items-center gap-2 rounded-full border border-bad-bd bg-bad-bg py-1 pr-2 pl-1'
                  : 'inline-flex items-center gap-2 rounded-full border border-border bg-surface-2 py-1 pr-2 pl-1'
              }
            >
              <Avatar initials={initialsOf(label)} size={24} shape="circle" />
              <span className="font-medium text-[13px] text-text">{label}</span>
              {inactive && (
                <StatusBadge tone="bad" className="px-1.5 py-0">
                  {t('template.inactiveMember')}
                </StatusBadge>
              )}
              {!disabled && (
                <button
                  type="button"
                  data-testid={`member-remove-${m.user_id}`}
                  onClick={() => onRemoveMember(m.user_id)}
                  aria-label={t('template.removeMember', { name: label })}
                  className="text-text-3 hover:text-text"
                >
                  <X className="size-3.5" aria-hidden />
                </button>
              )}
            </span>
          );
        })}

        {/* Add-member: the picker appears inline once "Tambah anggota" is pressed */}
        {!disabled &&
          (adding ? (
            <div className="min-w-[240px]" data-testid={`line-picker-${lineNo}`}>
              <EmployeePicker
                value={null}
                valueField="user_id"
                onChange={() => {}}
                onPick={(v, label) => {
                  if (v) onAddMember(v, label);
                  setAdding(false);
                }}
                placeholder={t('template.memberPlaceholder')}
              />
            </div>
          ) : (
            <button
              type="button"
              data-testid={`line-add-member-${lineNo}`}
              onClick={() => setAdding(true)}
              className="inline-flex items-center gap-1.5 rounded-full border border-primary bg-surface px-3 py-1.5 font-semibold text-[13px] text-primary hover:bg-primary-soft"
            >
              <Plus className="size-3.5" aria-hidden />
              {t('template.addMember')}
            </button>
          ))}
      </div>

      {/* OR note (TM-4 + INV-3 self-approve) */}
      <div className="flex items-center gap-1.5">
        <Split className="size-3.5 shrink-0 text-text-3" aria-hidden />
        <span className="text-[12px] text-text-3">{t('template.orNote')}</span>
      </div>

      {/* Field-level validation (422 APPROVAL_LINE_INVALID / empty line) */}
      {(error || isEmpty || hasInactive) && (
        <p className="text-[12px] text-bad-tx" role="alert">
          {error ?? (isEmpty ? t('template.errorEmptyLine') : t('template.errorInactiveLine'))}
        </p>
      )}
    </div>
  );
}
