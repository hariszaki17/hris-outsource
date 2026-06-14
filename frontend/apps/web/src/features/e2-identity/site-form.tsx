/**
 * E2 · Client Sites & Geofence (F2.6)
 *
 * Exports:
 *   - SitesPanel       — lists a company's sites (geofence status, primary, placements) + Add/Edit
 *   - SiteFormDrawer   — create/edit a site with the interactive MapPicker geofence editor
 *
 * The geofence (center + radius) lives on the Site, not the company (relocated 2026-06-03,
 * EPICS §8). RHF + hand-written Zod (E2 Zod codegen deferred). Field errors → applyFieldErrors.
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type Site,
  type SiteWriteRequest,
  useCreateSite,
  useListSites,
  useUpdateSite,
} from '@swp/api-client/e2';
import {
  Button,
  Drawer,
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  FormField,
  Input,
  MapPicker,
  StateView,
  StatusBadge,
  Toggle,
  useToast,
} from '@swp/ui';
import { Edit2, MapPin, MapPinOff, Plus, Star } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Zod schema (hand-written — E2 Zod codegen deferred)
// ---------------------------------------------------------------------------

const siteSchema = z.object({
  name: z.string().min(1).max(200),
  code: z.string().optional(),
  address: z.string().min(1),
  pic_name: z.string().optional(),
  phone: z.string().optional(),
  lat: z
    .string()
    .optional()
    .refine((v) => !v || (!Number.isNaN(Number(v)) && Number(v) >= -90 && Number(v) <= 90), {
      message: '-90..90',
    }),
  lng: z
    .string()
    .optional()
    .refine((v) => !v || (!Number.isNaN(Number(v)) && Number(v) >= -180 && Number(v) <= 180), {
      message: '-180..180',
    }),
  geofence_radius_m: z.coerce.number().min(25).max(1000).default(100),
  is_primary: z.boolean().default(false),
});

type SiteFormValues = z.infer<typeof siteSchema>;

function buildWriteRequest(values: SiteFormValues): SiteWriteRequest {
  const hasGeo = values.lat && values.lng;
  return {
    name: values.name,
    code: values.code || undefined,
    address: values.address,
    pic_name: values.pic_name || undefined,
    phone: values.phone || undefined,
    geo: hasGeo ? { lat: Number(values.lat), lng: Number(values.lng) } : null,
    geofence_radius_m: values.geofence_radius_m,
    is_primary: values.is_primary,
  };
}

// ---------------------------------------------------------------------------
// SiteFormDrawer — create / edit
// ---------------------------------------------------------------------------

interface SiteFormDrawerProps {
  clientCompanyId: string;
  /** When set, the drawer edits this site; otherwise it creates a new one. */
  site?: Site;
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
}

export function SiteFormDrawer({
  clientCompanyId,
  site,
  open,
  onClose,
  onSaved,
}: SiteFormDrawerProps) {
  const { t } = useTranslation('sites');
  const { toast } = useToast();
  const isEdit = !!site;

  const form = useForm<SiteFormValues>({
    resolver: zodResolver(siteSchema),
    defaultValues: {
      name: '',
      code: '',
      address: '',
      pic_name: '',
      phone: '',
      lat: '',
      lng: '',
      geofence_radius_m: 100,
      is_primary: false,
    },
  });
  const {
    register,
    watch,
    setValue,
    formState: { errors },
  } = form;

  // Hydrate when editing (or reset when switching to create).
  useEffect(() => {
    if (site) {
      form.reset({
        name: site.name ?? '',
        code: site.code ?? '',
        address: site.address ?? '',
        pic_name: site.pic_name ?? '',
        phone: site.phone ?? '',
        lat: site.geo?.lat != null ? String(site.geo.lat) : '',
        lng: site.geo?.lng != null ? String(site.geo.lng) : '',
        geofence_radius_m: site.geofence_radius_m ?? 100,
        is_primary: site.is_primary ?? false,
      });
    } else {
      form.reset({
        name: '',
        code: '',
        address: '',
        pic_name: '',
        phone: '',
        lat: '',
        lng: '',
        geofence_radius_m: 100,
        is_primary: false,
      });
    }
  }, [site, form]);

  const createMut = useCreateSite();
  const updateMut = useUpdateSite();
  const pending = createMut.isPending || updateMut.isPending;

  const values = watch();
  const center =
    values.lat && values.lng ? { lat: Number(values.lat), lng: Number(values.lng) } : null;
  const radius = Number(values.geofence_radius_m) || 100;

  function onSubmit(v: SiteFormValues) {
    const data = buildWriteRequest(v);
    const onError = (err: unknown) => {
      if (applyFieldErrors(err, form.setError as never)) return;
      const { message } = classifyError(err);
      toast({
        tone: 'error',
        title: t(isEdit ? 'toast.updateFailed' : 'toast.createFailed'),
        description: message,
      });
    };
    if (isEdit && site) {
      updateMut.mutate(
        { siteId: site.id, data },
        {
          onSuccess: () => {
            toast({ tone: 'success', title: t('toast.updated') });
            onSaved();
          },
          onError,
        },
      );
    } else {
      createMut.mutate(
        { clientCompanyId, data },
        {
          onSuccess: () => {
            toast({ tone: 'success', title: t('toast.created') });
            onSaved();
          },
          onError,
        },
      );
    }
  }

  return (
    <Drawer
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
      width={640}
    >
      <DrawerHeader title={t(isEdit ? 'form.editTitle' : 'form.createTitle')} onClose={onClose} />
      <DrawerBody>
        <form
          id="site-form"
          onSubmit={form.handleSubmit(onSubmit)}
          noValidate
          className="flex flex-col gap-4"
        >
          {/* Identity */}
          <div className="flex gap-[14px]">
            <FormField
              htmlFor="site-name"
              label={`${t('form.name')} *`}
              error={errors.name?.message}
              className="flex-[2]"
            >
              <Input
                id="site-name"
                {...register('name')}
                placeholder={t('form.namePlaceholder')}
                aria-required
              />
            </FormField>
            <FormField
              htmlFor="site-code"
              label={t('form.code')}
              error={errors.code?.message}
              className="flex-1"
            >
              <Input id="site-code" {...register('code')} className="font-mono" />
            </FormField>
          </div>
          <FormField
            htmlFor="site-address"
            label={`${t('form.address')} *`}
            error={errors.address?.message}
          >
            <Input
              id="site-address"
              {...register('address')}
              placeholder={t('form.addressPlaceholder')}
              aria-required
            />
          </FormField>
          <div className="flex gap-[14px]">
            <FormField
              htmlFor="site-pic"
              label={t('form.pic')}
              error={errors.pic_name?.message}
              className="flex-1"
            >
              <Input id="site-pic" {...register('pic_name')} />
            </FormField>
            <FormField
              htmlFor="site-phone"
              label={t('form.phone')}
              error={errors.phone?.message}
              className="flex-1"
            >
              <Input id="site-phone" {...register('phone')} className="font-mono" />
            </FormField>
          </div>

          {/* Geofence */}
          <div className="flex flex-col gap-[10px] rounded-xl border border-border bg-surface p-4">
            <div className="flex items-center gap-2">
              <MapPin size={15} className="text-primary" aria-hidden />
              <span className="text-[14px] font-semibold text-text">{t('form.geofenceTitle')}</span>
            </div>
            {!center && <p className="text-[12px] text-warn-tx">{t('form.noGeoHint')}</p>}
            <p className="text-[12px] text-text-2">{t('form.mapHint')}</p>
            <MapPicker
              value={center}
              radiusM={radius}
              onChange={(next) => {
                setValue('lat', String(next.lat.toFixed(6)), { shouldDirty: true });
                setValue('lng', String(next.lng.toFixed(6)), { shouldDirty: true });
              }}
              height={300}
            />
            <div className="flex gap-[14px]">
              <FormField
                htmlFor="site-lat"
                label={t('form.lat')}
                error={errors.lat?.message}
                className="flex-1"
              >
                <Input
                  id="site-lat"
                  {...register('lat')}
                  placeholder="-6.2253"
                  className="font-mono"
                />
              </FormField>
              <FormField
                htmlFor="site-lng"
                label={t('form.lng')}
                error={errors.lng?.message}
                className="flex-1"
              >
                <Input
                  id="site-lng"
                  {...register('lng')}
                  placeholder="106.7995"
                  className="font-mono"
                />
              </FormField>
              <FormField
                htmlFor="site-radius"
                label={t('form.radius')}
                error={errors.geofence_radius_m?.message}
                className="flex-1"
              >
                <Input
                  id="site-radius"
                  {...register('geofence_radius_m')}
                  type="number"
                  min={25}
                  max={1000}
                  className="font-mono"
                />
              </FormField>
            </div>
          </div>

          {/* Primary */}
          <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-4 py-3">
            <span className="flex flex-col">
              <span className="text-[13px] font-medium text-text">{t('form.primary')}</span>
              <span className="text-[11px] text-text-3">{t('form.primaryHint')}</span>
            </span>
            <Toggle
              checked={values.is_primary}
              onCheckedChange={(c) => setValue('is_primary', c, { shouldDirty: true })}
              aria-label={t('form.primary')}
            />
          </div>
        </form>
      </DrawerBody>
      <DrawerFooter>
        <span className="flex-1" />
        <Button type="button" variant="secondary" onClick={onClose}>
          {t('form.cancel')}
        </Button>
        <Button type="submit" form="site-form" disabled={pending}>
          {pending ? t('form.saving') : t('form.save')}
        </Button>
      </DrawerFooter>
    </Drawer>
  );
}

// ---------------------------------------------------------------------------
// SitesPanel — list a company's sites
// ---------------------------------------------------------------------------

export function SitesPanel({ clientCompanyId }: { clientCompanyId: string }) {
  const { t } = useTranslation('sites');
  const query = useListSites(clientCompanyId);
  const sites = ((query.data?.data as { data?: Site[] } | undefined)?.data ?? []) as Site[];

  const [createOpen, setCreateOpen] = useState(false);
  const [editSite, setEditSite] = useState<Site | null>(null);

  return (
    <div className="rounded-xl bg-surface border border-border overflow-hidden">
      <div className="flex items-center justify-between px-5 pt-4 pb-3 border-b border-border-soft">
        <div className="flex items-center gap-2">
          <MapPin size={16} className="text-primary" aria-hidden />
          <span className="text-[14px] font-semibold text-text">{t('panel.title')}</span>
        </div>
        <Button type="button" variant="secondary" onClick={() => setCreateOpen(true)}>
          <Plus size={14} aria-hidden className="mr-1.5" />
          {t('panel.addSite')}
        </Button>
      </div>

      {query.isPending ? (
        <div className="p-5">
          <StateView kind="loading" title={t('state.loading')} />
        </div>
      ) : query.isError ? (
        <div className="p-5">
          <StateView
            kind="error"
            title={t('state.errorTitle')}
            description={classifyError(query.error).message}
            onRetry={() => void query.refetch()}
          />
        </div>
      ) : (
        <ul>
          {sites.map((site) => (
            <li
              key={site.id}
              className="flex items-center justify-between gap-4 px-5 py-[14px] border-t border-border-soft first:border-t-0"
            >
              <div className="flex flex-col gap-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-[14px] font-medium text-text truncate">{site.name}</span>
                  {site.is_primary && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-primary-soft px-2 py-0.5 text-[10px] font-semibold text-primary-strong">
                      <Star size={9} aria-hidden />
                      {t('panel.primary')}
                    </span>
                  )}
                </div>
                <span className="text-[12px] text-text-2 truncate">{site.address}</span>
              </div>
              <div className="flex items-center gap-3 shrink-0">
                <span className="text-[11px] text-text-3">
                  {t('panel.placements', { count: site.active_placement_count ?? 0 })}
                </span>
                <StatusBadge tone={site.geofence_active ? 'ok' : 'warn'} dot>
                  {site.geofence_active ? t('panel.geofenceActive') : t('panel.geofenceInactive')}
                </StatusBadge>
                <button
                  type="button"
                  aria-label={t('panel.edit')}
                  onClick={() => setEditSite(site)}
                  className="text-text-2 hover:text-primary"
                >
                  <Edit2 size={15} aria-hidden />
                </button>
              </div>
            </li>
          ))}
          {sites.length === 0 && (
            <li className="flex items-center gap-2 px-5 py-6 text-[13px] text-text-3">
              <MapPinOff size={15} aria-hidden />
              {t('panel.empty')}
            </li>
          )}
        </ul>
      )}

      {createOpen && (
        <SiteFormDrawer
          clientCompanyId={clientCompanyId}
          open
          onClose={() => setCreateOpen(false)}
          onSaved={() => {
            setCreateOpen(false);
            query.refetch();
          }}
        />
      )}
      {editSite && (
        <SiteFormDrawer
          clientCompanyId={clientCompanyId}
          site={editSite}
          open
          onClose={() => setEditSite(null)}
          onSaved={() => {
            setEditSite(null);
            query.refetch();
          }}
        />
      )}
    </div>
  );
}
