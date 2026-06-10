/**
 * E5 · F5.6 — Buat Kehadiran Manual (Manual Attendance Page)
 *
 * Full-page form: employee + date → autofill placement/schedule → check-in/out → submit.
 * Replaces the old modal approach. No kode absensi.
 */

import { customFetch } from '@swp/api-client';
import { useListEmployees } from '@swp/api-client/e2';
import { Button, Input, useToast } from '@swp/ui';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { ArrowLeft, Building2, CalendarClock, Clock, MapPin, Search, User, Users } from 'lucide-react';
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
}

interface ManualCreateBody {
  employee_id: string;
  check_in_at: string;
  check_out_at?: string;
  note?: string;
}

interface EmployeeOption {
  id: string;
  name: string;
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
  const employeeRows = ((employeesQuery.data?.data as { data?: EmployeeOption[] } | undefined)?.data ?? []);

  // Autofill fetch — only when both employee + date are selected
  const autofillUrl = employeeSelected && selectedDate
    ? `/attendance:manual-autofill?employee_id=${encodeURIComponent(selectedEmployeeId)}&date=${selectedDate}`
    : null;

  const autofillQuery = useQuery<AutofillData>({
    queryKey: ['manual-autofill', selectedEmployeeId, selectedDate],
    queryFn: async () => {
      const res = await customFetch(autofillUrl!);
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        if (res.status === 422) throw new Error(t('manualNoPlacement'));
        throw new Error(t('manualPageError'));
      }
      const json = await res.json();
      return json.data as AutofillData;
    },
    enabled: autofillUrl !== null,
    retry: false,
  });

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

  const isValid = selectedEmployeeId && checkInAt;
  const canSubmit = isValid && !mutation.isPending;

  return (
    <div className="mx-auto max-w-2xl px-4 py-6">
      {/* Header with back button */}
      <div className="mb-6 flex items-center gap-4">
        <button
          type="button"
          className="flex size-9 items-center justify-center rounded-lg text-text-3 hover:bg-surface-2 hover:text-text"
          onClick={() => navigate({ to: '/attendance' })}
        >
          <ArrowLeft className="size-5" />
        </button>
        <div>
          <h1 className="text-2xl font-bold text-text">{t('manualCreateTitle')}</h1>
          <p className="text-[13px] text-text-2">{t('manualCreateDesc')}</p>
        </div>
      </div>

      <div className="flex flex-col gap-6">
        {/* STEP 1: Employee selection */}
        <div className="rounded-xl border border-border bg-surface p-5">
          <h2 className="mb-1 text-[15px] font-semibold text-text">1. {t('manualSelectEmployee')}</h2>
          <p className="mb-4 text-[12px] text-text-3">{t('manualCreateDesc')}</p>

          {selectedEmployee ? (
            <div className="flex items-center justify-between rounded-lg border border-border bg-surface-2 px-3 py-2">
              <div className="flex items-center gap-2">
                <User className="size-4 text-text-3" />
                <span className="text-[13px] font-medium text-text">{selectedEmployee.name}</span>
                <span className="font-mono text-[11px] text-text-3">{selectedEmployee.id}</span>
              </div>
              <button
                type="button"
                className="text-[12px] text-text-3 hover:text-bad"
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
                <div className="mt-1 max-h-[200px] overflow-y-auto rounded-lg border border-border bg-surface shadow-overlay">
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
                        <span>{emp.name}</span>
                        <span className="ml-auto font-mono text-[11px] text-text-3">{emp.id}</span>
                      </button>
                    ))
                  )}
                </div>
              )}
            </div>
          )}
        </div>

        {/* STEP 2: Date picker */}
        <div className="rounded-xl border border-border bg-surface p-5">
          <h2 className="mb-1 text-[15px] font-semibold text-text">2. {t('manualCreateDate')}</h2>
          <p className="mb-4 text-[12px] text-text-3">{t('manualCreateDesc')}</p>
          <Input
            type="date"
            value={selectedDate}
            onChange={(e) => setSelectedDate(e.target.value)}
            className="w-full max-w-xs"
            disabled={!employeeSelected}
          />
        </div>

        {/* STEP 3: Autofill results */}
        {employeeSelected && selectedDate && (
          <div className="rounded-xl border border-border bg-surface p-5">
            <div className="mb-1 flex items-center gap-2">
              <CalendarClock className="size-4 text-primary" />
              <h2 className="text-[15px] font-semibold text-text">3. {t('manualCreateTitle')}</h2>
            </div>

            {autofillQuery.isLoading && (
              <p className="py-4 text-[13px] text-text-3">{t('manualAutofillLoading')}</p>
            )}

            {autofillQuery.isError && (
              <div className="rounded-lg bg-bad-bg px-4 py-3">
                <p className="text-[13px] text-bad">{autofillQuery.error.message}</p>
              </div>
            )}

            {autofillQuery.data && (
              <div className="mt-3 space-y-3">
                {/* Placement info card */}
                <div className="rounded-lg border border-border bg-surface-2 p-4">
                  <div className="grid grid-cols-2 gap-x-6 gap-y-3">
                    <InfoRow icon={Building2} label={t('companyName', { ns: 'common' })} value={autofillQuery.data.company_name} />
                    <InfoRow icon={MapPin} label={t('site_name', { ns: 'common' })} value={autofillQuery.data.site_name ?? '-'} />
                    <InfoRow icon={Users} label={t('position_name', { ns: 'common' })} value={autofillQuery.data.position_name ?? '-'} />
                    <InfoRow icon={Clock} label="Lini Layanan" value={t(`serviceLine.${autofillQuery.data.service_line}` as any)} />
                  </div>
                </div>

                {/* Schedule info */}
                {autofillQuery.data.schedule_id ? (
                  <div className="rounded-lg border border-border bg-brand-bg/30 px-4 py-3">
                    <p className="mb-2 text-[13px] font-medium text-brand">{t('manualAutofillScheduleFound')}</p>
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
                  <div className="rounded-lg border border-border bg-warning-bg/30 px-4 py-3">
                    <p className="text-[13px] text-warning">{t('manualAutofillNoSchedule')}</p>
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* STEP 4: Check-in/out times */}
        <div className="rounded-xl border border-border bg-surface p-5">
          <div className="mb-1 flex items-center gap-2">
            <Clock className="size-4 text-primary" />
            <h2 className="text-[15px] font-semibold text-text">4. {t('manualCheckInAt')}</h2>
          </div>
          <p className="mb-4 text-[12px] text-text-3">{t('manualCreateDesc')}</p>

          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-1.5">
              <label htmlFor="manual-checkin" className="text-[12px] font-medium text-text-2">
                {t('manualCheckInAt')}
              </label>
              <Input
                id="manual-checkin"
                type="datetime-local"
                value={checkInAt}
                onChange={(e) => setCheckInAt(e.target.value)}
                required
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor="manual-checkout" className="text-[12px] font-medium text-text-2">
                {t('manualCheckOutAt')}
              </label>
              <Input
                id="manual-checkout"
                type="datetime-local"
                value={checkOutAt}
                onChange={(e) => setCheckOutAt(e.target.value)}
              />
            </div>
          </div>
        </div>

        {/* STEP 5: Note + Submit */}
        <div className="rounded-xl border border-border bg-surface p-5">
          <h2 className="mb-1 text-[15px] font-semibold text-text">5. Catatan</h2>

          <div className="mb-5 flex flex-col gap-1.5">
            <label htmlFor="manual-note" className="text-[12px] font-medium text-text-2">
              {t('manualNote')}
            </label>
            <textarea
              id="manual-note"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder={t('manualNote')}
              className="min-h-[80px] w-full resize-y rounded-lg border border-border bg-surface-2 px-3 py-2 text-[13px] text-text placeholder:text-text-3 focus:border-primary focus:outline-none"
              rows={3}
            />
          </div>

          {/* Error feedback */}
          {mutation.isError && (
            <div className="mb-4 rounded-lg bg-bad-bg px-3 py-2">
              <p className="text-[12px] text-bad">{t('manualError')}</p>
            </div>
          )}

          <div className="flex items-center gap-3">
            <Button variant="ghost" onClick={() => navigate({ to: '/attendance' })}>
              {t('manualBack')}
            </Button>
            <Button
              variant="filled"
              tone="primary"
              disabled={!canSubmit}
              onClick={handleSubmit}
            >
              {mutation.isPending ? tc('loading') : t('manualSubmit')}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function InfoRow({ icon: Icon, label, value }: { icon: React.ComponentType<{ className?: string }>; label: string; value: string }) {
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
  return d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', timeZoneName: 'short' });
}
