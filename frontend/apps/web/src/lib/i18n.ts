import { DEFAULT_LOCALE, resources } from '@swp/shared';
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

/** i18next init — Bahasa Indonesia default, en-US fallback. Catalogs live in @swp/shared. */
void i18n.use(initReactI18next).init({
  resources,
  lng: DEFAULT_LOCALE,
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
});

export { i18n };
