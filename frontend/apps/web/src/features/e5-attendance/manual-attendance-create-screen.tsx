/**
 * E5 · F5.6 — Buat Kehadiran Manual (Manual Attendance Page)
 *
 * Full-page form: employee + date → autofill placement/schedule → check-in/out → submit.
 * Replaces the old modal approach. No kode absensi.
 */

import { customFetch } from '@swp/api-client';
import { useListEmployees } from '@swp/api-client/e2';
import { Button, FormField, Input, useToast } from '@swp/ui';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import {
  ArrowLeft,
  Building2,
  CalendarClock,
  Clock,
  Info,
  MapPin,
  Search,
  TriangleAlert,
  User,
  Users,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface AutofillData {
  employee_name: string;
  company_name: string;
  site_name: string | null;
  position_name: string | null;
  service_line: string;
  schedule_id: string | null;
  shift_start_at: string | null;
  shift_end_at: string | null;
  // Non-null when an attendance record already exists for this employee+date (cron
  // auto-creates ABSENT/PENDING rows). UI redirects to verify/correct it.
  existing_attendance_id: string | null;
  existing_attendance_status: string | null;
  existing_verification_status: string | null;
}

interface ManualCreateBody {
  employee_id: string;
  check_in_at: string;
  check_out_at?: string;
  note?: string;
}

interface EmployeeOption {
  id: string;
  full_name: string;
}

// ---------------------------------------------------------------------------
// ManualAttendanceCreateScreen
// ---------------------------------------------------------------------------

export function ManualAttendanceCreateScreen() {
  const { t } = useTranslation('attendance');
  const { t: tc } = useTranslation('common');
  const { toast } = useToast();
  const navigate = useNavigate();

  // Step 1: employee + date
  const [employeeSearch, setEmployeeSearch] = useState('');
  const [selectedEmployeeId, setSelectedEmployeeId] = useState('');
  const [selectedDate, setSelectedDate] = useState(() => new Date().toISOString().slice(0, 10));

  // Step 3: check-in/out
  const [checkInAt, setCheckInAt] = useState('');
  const [checkOutAt, setCheckOutAt] = useState('');

  // Step 4: note
  const [note, setNote] = useState('');

  const employeeSelected = selectedEmployeeId !== '';

  // Employee search
  const employeesQuery = useListEmployees(
    { limit: 50, q: employeeSearch || undefined },
    { query: { enabled: employeeSearch.length > 0, staleTime: 30_000 } },
  );
  const employeeRows =
    (employeesQuery.data?.data as { data?: EmployeeOption[] } | undefined)?.data ?? [];

  // Autofill fetch — only when both employee + date are selected
  const autofillUrl =
    employeeSelected && selectedDate
      ? `/attendance:manual-autofill?employee_id=${encodeURIComponent(selectedEmployeeId)}&date=${selectedDate}`
      : null;

  // Returns AutofillData on success, or `null` when the employee has no active
  // placement covering the chosen date (NO_ACTIVE_PLACEMENT, 404/422). `null` is a
  // resolved state — surfaced as a non-blocking warning, NOT a fetch error. Only
  // genuine failures (network/5xx) land in the error state.
  const autofillQuery = useQuery<AutofillData | null>({
    queryKey: ['manual-autofill', selectedEmployeeId, selectedDate],
    queryFn: async () => {
      // customFetch returns the `{ data, status, headers }` envelope and throws
      // ApiError on any non-2xx. The success body is `{ data: AutofillData }`.
      try {
        const res = await customFetch<{ data: { data: AutofillData } }>(autofillUrl!);
        return res.data.data;
      } catch (err) {
        const status = (err as { status?: number })?.status;
        if (status === 404 || status === 422) return null;
        throw new Error(t('manualPageError'));
      }
    },
    enabled: autofillUrl !== null,
    retry: false,
  });

  // No active placement on the chosen date — non-blocking, informational.
  const noPlacement = autofillQuery.data === null && !autofillQuery.isLoading;

  const selectedEmployee = employeeRows.find((e) => e.id === selectedEmployeeId);

  // Create mutation
  const mutation = useMutation({
    mutationFn: (body: ManualCreateBody) =>
      customFetch('/attendance:manual-create', {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      toast({ tone: 'success', title: t('manualSuccess') });
      navigate({ to: '/attendance' });
    },
    onError: () => {
      toast({ tone: 'error', title: t('manualError') });
    },
  });

  function handleSubmit() {
    if (!selectedEmployeeId || !checkInAt) return;
    const body: ManualCreateBody = {
      employee_id: selectedEmployeeId,
      check_in_at: new Date(checkInAt).toISOString(),
    };
    if (checkOutAt) {
      body.check_out_at = new Date(checkOutAt).toISOString();
    }
    if (note) {
      body.note = note;
    }
    mutation.mutate(body);
  }

  // An attendance record already exists for this employee+date (cron auto-created it).
  // Creating another would duplicate/conflict — steer the admin to verify/correct it.
  const existingAttendanceId = autofillQuery.data?.existing_attendance_id ?? null;

  const isValid = selectedEmployeeId && checkInAt;
  const canSubmit = isValid && !mutation.isPending && !existingAttendanceId;

  return (
    <div className="flex flex-col gap-4 p-6 bg-app-bg h-full overflow-y-auto">
      {/* Back row */}
      <div className="flex items-center gap-[7px]">
        <button
          type="button"
          className="flex items-center gap-[7px] text-text-2 hover:text-text"
          onClick={() => navigate({ to: '/attendance' })}
        >
          <ArrowLeft size={16} aria-hidden />
          <span className="text-[13px] font-medium">{t('manualBack')}</span>
        </button>
      </div>

      {/* Header */}
      <div className="rounded-xl bg-surface border border-border px-5 py-[18px] flex items-center justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="text-[18px] font-semibold text-text">{t('manualCreateTitle')}</h1>
          <p className="text-[12px] text-text-2">{t('manualCreateDesc')}</p>
        </div>
      </div>

      {/* Form body: left form sections + right summary */}
      <div className="flex gap-4">
        {/* Left col */}
        <div className="flex-1 flex flex-col gap-4">
          {/* Section: Karyawan & Tanggal — no overflow-hidden so the search dropdown isn't clipped */}
          <div className="rounded-xl bg-surface border border-border">
            <div className="px-5 pt-[18px] pb-3 border-b border-border-soft flex flex-col gap-[2px]">
              <span className="text-[15px] font-semibold text-text">
                {t('manualSelectEmployee')}
              </span>
              <span className="text-[12px] text-text-2">{t('manualCreateDesc')}</span>
            </div>
            <div className="flex flex-col gap-[14px] p-5">
              <div className="flex gap-[14px]">
                {/* Employee */}
                <div className="flex-1 flex flex-col gap-1.5">
                  <span className="text-[13px] font-medium text-text-2">
                    {t('manualSelectEmployee')} *
                  </span>
                  {selectedEmployee ? (
                    <div className="flex items-center justify-between rounded-md border border-border bg-surface-2 px-3 py-2">
                      <div className="flex min-w-0 items-center gap-2">
                        <User className="size-4 shrink-0 text-text-3" />
                        <span className="truncate text-[13px] font-medium text-text">
                          {selectedEmployee.full_name}
                        </span>
                        <span className="shrink-0 font-mono text-[11px] text-text-3">
                          {selectedEmployee.id}
                        </span>
                      </div>
                      <button
                        type="button"
                        className="shrink-0 text-[12px] text-text-3 hover:text-bad"
                        onClick={() => {
                          setSelectedEmployeeId('');
                          setEmployeeSearch('');
                        }}
                      >
                        {tc('cancel')}
                      </button>
                    </div>
                  ) : (
                    <div className="relative">
                      <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-text-3" />
                      <Input
                        placeholder={t('manualSelectEmployee')}
                        value={employeeSearch}
                        onChange={(e) => setEmployeeSearch(e.target.value)}
                        className="w-full pl-9"
                      />
                      {employeeSearch && (
                        <div className="absolute z-10 mt-1 max-h-[200px] w-full overflow-y-auto rounded-lg border border-border bg-surface shadow-overlay">
                          {employeesQuery.isLoading ? (
                            <p className="px-3 py-2 text-[12px] text-text-3">{tc('loading')}</p>
                          ) : employeeRows.length === 0 ? (
                            <p className="px-3 py-2 text-[12px] text-text-3">{tc('empty')}</p>
                          ) : (
                            employeeRows.map((emp) => (
                              <button
                                key={emp.id}
                                type="button"
                                className="flex w-full items-center gap-2 px-3 py-2 text-left text-[13px] text-text hover:bg-surface-2"
                                onClick={() => setSelectedEmployeeId(emp.id)}
                              >
                                <User className="size-4 shrink-0 text-text-3" />
                                <span className="truncate">{emp.full_name}</span>
                                <span className="ml-auto shrink-0 font-mono text-[11px] text-text-3">
                                  {emp.id}
                                </span>
                              </button>
                            ))
                          )}
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* Date */}
                <FormField
                  htmlFor="manual-date"
                  label={`${t('manualCreateDate')} *`}
                  className="flex-1"
                >
                  <Input
                    id="manual-date"
                    type="date"
                    value={selectedDate}
                    onChange={(e) => setSelectedDate(e.target.value)}
                    disabled={!employeeSelected}
                  />
                </FormField>
              </div>
            </div>
          </div>

          {/* Section: Jam Kehadiran */}
          <div className="rounded-xl bg-surface border border-border overflow-hidden">
            <div className="px-5 pt-[18px] pb-3 border-b border-border-soft flex flex-col gap-[2px]">
              <span className="text-[15px] font-semibold text-text">{t('manualCheckInAt')}</span>
              <span className="text-[12px] text-text-2">{t('manualCreateDesc')}</span>
            </div>
            <div className="flex flex-col gap-[14px] p-5">
              <div className="flex gap-[14px]">
                <FormField
                  htmlFor="manual-checkin"
                  label={`${t('manualCheckInAt')} *`}
                  className="flex-1"
                >
                  <Input
                    id="manual-checkin"
                    type="datetime-local"
                    value={checkInAt}
                    onChange={(e) => setCheckInAt(e.target.value)}
                    required
                  />
                </FormField>
                <FormField
                  htmlFor="manual-checkout"
                  label={t('manualCheckOutAt')}
                  className="flex-1"
                >
                  <Input
                    id="manual-checkout"
                    type="datetime-local"
                    value={checkOutAt}
                    onChange={(e) => setCheckOutAt(e.target.value)}
                  />
                </FormField>
              </div>
            </div>
          </div>

          {/* Section: Catatan */}
          <div className="rounded-xl bg-surface border border-border overflow-hidden">
            <div className="px-5 pt-[18px] pb-3 border-b border-border-soft flex flex-col gap-[2px]">
              <span className="text-[15px] font-semibold text-text">{t('manualNote')}</span>
            </div>
            <div className="flex flex-col gap-[14px] p-5">
              <textarea
                id="manual-note"
                value={note}
                onChange={(e) => setNote(e.target.value)}
                placeholder={t('manualNote')}
                className="min-h-[100px] w-full resize-y rounded-md border border-border bg-surface px-3 py-2 text-[13px] text-text placeholder:text-text-3 outline-none focus-visible:ring-2 focus-visible:ring-ring"
                rows={4}
              />
            </div>
          </div>
        </div>

        {/* Right col: placement summary + guideline */}
        <div className="w-[380px] flex flex-col gap-4">
          {/* Placement summary */}
          <div className="rounded-xl bg-surface border border-border overflow-hidden">
            <div className="px-5 pt-4 pb-3 border-b border-border-soft flex items-center gap-2">
              <CalendarClock size={16} className="text-primary shrink-0" aria-hidden />
              <span className="text-[14px] font-semibold text-text">
                {t('manualPlacementSummary')}
              </span>
            </div>
            <div className="p-5">
              {!employeeSelected && (
                <p className="text-[12px] text-text-3">{t('manualSummaryEmpty')}</p>
              )}

              {employeeSelected && autofillQuery.isLoading && (
                <p className="text-[12px] text-text-3">{t('manualAutofillLoading')}</p>
              )}

              {employeeSelected && autofillQuery.isError && (
                <div className="rounded-lg bg-bad-bg px-4 py-3">
                  <p className="mb-2 text-[13px] text-bad">{autofillQuery.error.message}</p>
                  <button
                    type="button"
                    className="text-[12px] font-medium text-bad underline hover:no-underline"
                    onClick={() => void autofillQuery.refetch()}
                  >
                    {tc('retry')}
                  </button>
                </div>
              )}

              {/* No active placement on this date — non-blocking, informational */}
              {employeeSelected && noPlacement && (
                <div className="rounded-lg border border-warning-bd bg-warning-bg/40 px-4 py-3">
                  <div className="mb-1 flex items-center gap-2">
                    <TriangleAlert size={15} className="text-warning shrink-0" aria-hidden />
                    <span className="text-[13px] font-semibold text-warning">
                      {t('manualNoPlacementTitle')}
                    </span>
                  </div>
                  <p className="text-[12px] text-text-2 leading-relaxed">
                    {t('manualNoPlacementBody')}
                  </p>
                </div>
              )}

              {employeeSelected && autofillQuery.data && (
                <div className="flex flex-col gap-3">
                  <InfoRow
                    icon={Building2}
                    label={t('companyName', { ns: 'common' })}
                    value={autofillQuery.data.company_name}
                  />
                  <InfoRow
                    icon={MapPin}
                    label={t('site_name', { ns: 'common' })}
                    value={autofillQuery.data.site_name ?? '-'}
                  />
                  <InfoRow
                    icon={Users}
                    label={t('position_name', { ns: 'common' })}
                    value={autofillQuery.data.position_name ?? '-'}
                  />
                  <InfoRow
                    icon={Clock}
                    label="Lini Layanan"
                    value={t(`serviceLine.${autofillQuery.data.service_line}` as never)}
                  />

                  {/* Schedule */}
                  {autofillQuery.data.schedule_id ? (
                    <div className="mt-1 rounded-lg border border-border bg-brand-bg/30 px-4 py-3">
                      <p className="mb-2 text-[13px] font-medium text-brand">
                        {t('manualAutofillScheduleFound')}
                      </p>
                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <span className="text-[11px] text-text-3">{t('manualShiftStart')}</span>
                          <p className="text-[13px] font-medium text-text">
                            {formatShiftTime(autofillQuery.data.shift_start_at)}
                          </p>
                        </div>
                        <div>
                          <span className="text-[11px] text-text-3">{t('manualShiftEnd')}</span>
                          <p className="text-[13px] font-medium text-text">
                            {formatShiftTime(autofillQuery.data.shift_end_at)}
                          </p>
                        </div>
                      </div>
                    </div>
                  ) : (
                    <div className="mt-1 rounded-lg border border-warning-bd bg-warning-bg/30 px-4 py-3">
                      <p className="text-[13px] text-warning">{t('manualAutofillNoSchedule')}</p>
                    </div>
                  )}

                  {/* Attendance already exists (cron auto-created) → steer to verify/correct */}
                  {existingAttendanceId && (
                    <div className="mt-1 rounded-lg border border-info-bd bg-info-bg px-4 py-3">
                      <div className="mb-1 flex items-center gap-2">
                        <Info size={15} className="text-info-tx shrink-0" aria-hidden />
                        <span className="text-[13px] font-semibold text-info-tx">
                          {t('manualExistingTitle')}
                        </span>
                      </div>
                      <p className="mb-3 text-[12px] text-info-tx leading-relaxed">
                        {t('manualExistingBody')}
                      </p>
                      <Button
                        type="button"
                        variant="secondary"
                        onClick={() =>
                          navigate({
                            to: '/attendance/$attendanceId' as never,
                            params: { attendanceId: existingAttendanceId } as never,
                          })
                        }
                      >
                        {t('manualExistingCta')}
                      </Button>
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>

          {/* Guideline */}
          <div className="rounded-xl bg-info-bg border border-info-bd overflow-hidden p-4 flex flex-col gap-2">
            <div className="flex items-center gap-2">
              <Info size={16} className="text-info-tx shrink-0" aria-hidden />
              <span className="text-[13px] font-semibold text-info-tx">
                {t('manualGuidelineTitle')}
              </span>
            </div>
            <p className="text-[11px] text-info-tx leading-relaxed">{t('manualGuidelineBody')}</p>
          </div>
        </div>
      </div>

      {/* Footer */}
      <div className="rounded-xl bg-surface border border-border px-5 py-[14px] flex items-center justify-between">
        {mutation.isError ? (
          <span className="text-[11px] text-bad">{t('manualError')}</span>
        ) : existingAttendanceId ? (
          <span className="text-[11px] text-info-tx">{t('manualExistingBody')}</span>
        ) : (
          <span className="text-[11px] text-text-3">{t('manualCreateDesc')}</span>
        )}
        <div className="flex items-center gap-[10px]">
          <Button type="button" variant="secondary" onClick={() => navigate({ to: '/attendance' })}>
            {t('manualBack')}
          </Button>
          <Button type="button" disabled={!canSubmit} onClick={handleSubmit}>
            {mutation.isPending ? tc('loading') : t('manualSubmit')}
          </Button>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function InfoRow({
  icon: Icon,
  label,
  value,
}: { icon: React.ComponentType<{ className?: string }>; label: string; value: string }) {
  return (
    <div className="flex items-center gap-2">
      <Icon className="size-3.5 shrink-0 text-text-3" />
      <div className="min-w-0">
        <p className="text-[11px] text-text-3">{label}</p>
        <p className="truncate text-[13px] font-medium text-text">{value}</p>
      </div>
    </div>
  );
}

function formatShiftTime(rfc3339: string | null): string {
  if (!rfc3339) return '-';
  const d = new Date(rfc3339);
  return d.toLocaleTimeString('id-ID', {
    hour: '2-digit',
    minute: '2-digit',
    timeZoneName: 'short',
  });
}
