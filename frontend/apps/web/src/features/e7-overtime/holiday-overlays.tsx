/**
 * E7 · Kalender Hari Libur — Add/Edit holiday modal + delete confirm.
 *
 * .pen frame referenced:
 *   vd4na  "E7 · Aturan OT & Kalender Libur (HR)" — right column (Kalender Hari Libur)
 *
 * Holiday CRUD via E7 `/holidays` (HolidayWriteRequest). Delete is guarded server-side by
 * `in_use_by_overtime` → `HOLIDAY_IN_USE` (409); we surface that as a warning + disabled confirm.
 */
import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type Holiday,
  HolidayCategory,
  type HolidayWriteRequest,
  useCreateHoliday,
  useDeleteHoliday,
  useUpdateHoliday,
} from '@swp/api-client/e7';
import {
  Banner,
  Button,
  ConfirmDialog,
  FilterSelect,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Toggle,
  useToast,
} from '@swp/ui';
import { CalendarPlus, Trash2 } from 'lucide-react';
import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Zod (hand-authored — E7 Zod deferred)
// ---------------------------------------------------------------------------

const holidaySchema = z.object({
  name: z.string().min(2, 'Nama minimal 2 karakter').max(120),
  date: z.string().min(1, 'Tanggal wajib diisi'),
  category: z.enum([HolidayCategory.NATIONAL, HolidayCategory.REGIONAL, HolidayCategory.CUSTOM]),
  recurring: z.boolean(),
});

type HolidayFormValues = z.infer<typeof holidaySchema>;

// ---------------------------------------------------------------------------
// Add / Edit modal
// ---------------------------------------------------------------------------

export interface HolidayFormModalProps {
  /** When set, the modal is in edit mode for this holiday; otherwise create. */
  editing: Holiday | null;
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
}

export function HolidayFormModal({ editing, open, onClose, onSaved }: HolidayFormModalProps) {
  const { t } = useTranslation('overtime');
  const { toast } = useToast();
  const create = useCreateHoliday();
  const update = useUpdateHoliday();
  const isEdit = editing !== null;

  const {
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<HolidayFormValues>({
    resolver: zodResolver(holidaySchema),
    defaultValues: {
      name: '',
      date: '',
      category: HolidayCategory.NATIONAL,
      recurring: false,
    },
  });

  // Hydrate when (re)opening for an edit, reset for create.
  useEffect(() => {
    if (!open) return;
    if (editing) {
      reset({
        name: editing.name,
        date: editing.date,
        category: editing.category,
        recurring: editing.recurring,
      });
    } else {
      reset({ name: '', date: '', category: HolidayCategory.NATIONAL, recurring: false });
    }
  }, [open, editing, reset]);

  const recurring = watch('recurring');
  const category = watch('category');

  const onSubmit = handleSubmit(async (values) => {
    const body: HolidayWriteRequest = {
      name: values.name,
      date: values.date,
      category: values.category,
      recurring: values.recurring,
    };
    try {
      if (isEdit && editing) {
        await update.mutateAsync({ id: editing.id, data: body });
        toast({ tone: 'success', title: t('holidays.toastUpdated') });
      } else {
        await create.mutateAsync({ data: body });
        toast({ tone: 'success', title: t('holidays.toastCreated') });
      }
      onSaved();
      onClose();
    } catch (err) {
      const classified = classifyError(err);
      applyFieldErrors(err, setError);
      toast({
        tone: 'error',
        title: classified.kind === 'forbidden' ? t('errors.forbidden') : t('errors.processFailed'),
      });
    }
  });

  return (
    <Modal open={open} onOpenChange={(o) => !o && onClose()} size="md">
      <form onSubmit={onSubmit}>
        <ModalHeader
          icon={CalendarPlus}
          tone="info"
          title={isEdit ? t('holidays.editTitle') : t('holidays.addTitle')}
          onClose={onClose}
        />
        <ModalBody>
          <FormField
            htmlFor="holiday-name"
            label={t('holidays.fieldName')}
            required
            error={errors.name?.message}
          >
            <Input
              id="holiday-name"
              {...register('name')}
              placeholder={t('holidays.fieldNamePlaceholder')}
            />
          </FormField>
          <FormField
            htmlFor="holiday-date"
            label={t('holidays.fieldDate')}
            required
            error={errors.date?.message}
          >
            <Input id="holiday-date" type="date" {...register('date')} />
          </FormField>
          <FormField
            htmlFor="holiday-category"
            label={t('holidays.fieldCategory')}
            error={errors.category?.message}
          >
            <FilterSelect
              id="holiday-category"
              value={category}
              onChange={(e) => setValue('category', e.target.value as HolidayCategory)}
            >
              <option value={HolidayCategory.NATIONAL}>{t('holidayCategory.NATIONAL')}</option>
              <option value={HolidayCategory.REGIONAL}>{t('holidayCategory.REGIONAL')}</option>
              <option value={HolidayCategory.CUSTOM}>{t('holidayCategory.CUSTOM')}</option>
            </FilterSelect>
          </FormField>
          <div className="flex items-center justify-between rounded-lg border border-border bg-surface-2 px-3 py-[10px]">
            <div className="flex flex-col">
              <span className="text-[13px] font-medium text-text">
                {t('holidays.fieldRecurring')}
              </span>
              <span className="text-[11px] text-text-3">{t('holidays.fieldRecurringHint')}</span>
            </div>
            <Toggle
              checked={recurring}
              onCheckedChange={(v) => setValue('recurring', v)}
              aria-label={t('holidays.fieldRecurring')}
            />
          </div>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? t('common.processing') : t('common.save')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// Delete confirm
// ---------------------------------------------------------------------------

export interface DeleteHolidayConfirmProps {
  holiday: Holiday | null;
  open: boolean;
  onClose: () => void;
  onDeleted: () => void;
}

export function DeleteHolidayConfirm({
  holiday,
  open,
  onClose,
  onDeleted,
}: DeleteHolidayConfirmProps) {
  const { t } = useTranslation('overtime');
  const { toast } = useToast();
  const del = useDeleteHoliday();

  const inUse = holiday?.in_use_by_overtime ?? false;

  const onConfirm = async () => {
    if (!holiday) return;
    try {
      await del.mutateAsync({ id: holiday.id });
      toast({ tone: 'success', title: t('holidays.toastDeleted') });
      onDeleted();
      onClose();
    } catch (err) {
      const classified = classifyError(err);
      toast({
        tone: 'error',
        title:
          classified.kind === 'conflict' ? t('holidays.inUseError') : t('errors.processFailed'),
      });
    }
  };

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(o) => !o && onClose()}
      icon={Trash2}
      tone="danger"
      confirmTone="danger"
      title={t('holidays.deleteTitle')}
      description={holiday ? t('holidays.deleteBody', { name: holiday.name }) : ''}
      confirmLabel={t('holidays.deleteConfirm')}
      cancelLabel={t('common.cancel')}
      onConfirm={onConfirm}
      confirmDisabled={inUse}
      loading={del.isPending}
    >
      {inUse ? <Banner tone="warn" title={t('holidays.inUseWarning')} /> : null}
    </ConfirmDialog>
  );
}
