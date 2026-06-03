/**
 * SitePicker — FK picker for selecting a Site within a client company (E2 F2.6).
 *
 * Lists sites via `useListSites(clientCompanyId)`; disabled until a company is chosen.
 * Maps: id → value, name → label, address (+ "primary" hint) → sublabel.
 * Wraps the generic `Combobox` from @swp/ui. i18n namespace: `pickers`.
 */

import { type Site, useListSites } from '@swp/api-client/e2';
import { Combobox } from '@swp/ui';
import { useTranslation } from 'react-i18next';

export interface SitePickerProps {
  /** Parent company — required; picker is disabled until set. */
  clientCompanyId: string | null;
  value: string | null;
  onChange: (value: string | null) => void;
  disabled?: boolean;
  error?: boolean;
  placeholder?: string;
}

export function SitePicker({
  clientCompanyId,
  value,
  onChange,
  disabled,
  error,
  placeholder,
}: SitePickerProps) {
  const { t } = useTranslation('pickers');

  const result = useListSites(
    clientCompanyId ?? '',
    { limit: 100 },
    { query: { enabled: !!clientCompanyId, staleTime: 30_000 } },
  );

  const sites = ((result.data?.data as { data?: Site[] } | undefined)?.data ?? []) as Site[];

  const options = sites.map((s) => ({
    value: s.id,
    label: s.is_primary ? `${s.name} · ${t('site.primary')}` : s.name,
    sublabel: s.address,
  }));

  return (
    <Combobox
      value={value}
      onChange={onChange}
      options={options}
      onSearch={() => {}}
      isLoading={result.isLoading}
      placeholder={placeholder ?? t('site.placeholder')}
      disabled={disabled || !clientCompanyId}
      error={error}
      emptyText={t('site.empty')}
    />
  );
}
