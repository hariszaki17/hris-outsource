import { en } from './en.ts';
import { type Messages, id } from './id.ts';

export type Locale = 'id' | 'en';
export const DEFAULT_LOCALE: Locale = 'id';

/** Resources keyed by locale; consumed by the app's i18next init. */
export const resources: Record<Locale, { translation: Messages }> = {
  id: { translation: id },
  en: { translation: en },
};

export type { Messages };
export { id, en };
