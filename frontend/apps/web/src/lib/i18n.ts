import { DEFAULT_LOCALE, resources } from '@swp/shared';
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

/**
 * i18next init — Bahasa Indonesia default, en-US fallback. Catalogs live in @swp/shared.
 *
 * `fallbackNS: 'translation'` is load-bearing: each catalog is registered both as the whole
 * `translation` namespace AND with every section spread as its own namespace (shared/i18n). A
 * screen that calls `useTranslation('leave')` then `t('common.retry')` — a cross-namespace key —
 * or `t('leave.title')` — a full-path key — would otherwise miss (the lookup is scoped to the
 * active namespace). Falling back to `translation` (the full catalog) makes every key resolve
 * regardless of the active namespace or whether the call is bare (`t('title')`) or prefixed
 * (`t('leave.title')`). Without this, screens render raw key strings.
 */
void i18n.use(initReactI18next).init({
  resources,
  lng: DEFAULT_LOCALE,
  fallbackLng: 'en',
  defaultNS: 'translation',
  fallbackNS: 'translation',
  interpolation: { escapeValue: false },
});

export { i18n };
