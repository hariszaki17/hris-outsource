import { en } from './en.ts';
import { type Messages, id } from './id.ts';

export type Locale = 'id' | 'en';
export const DEFAULT_LOCALE: Locale = 'id';

/**
 * Resources keyed by locale; consumed by the app's i18next init.
 *
 * Each catalog is registered TWO ways so both call styles work uniformly:
 *   - the whole catalog under the default `translation` namespace → `t('employees.title')`
 *   - every top-level section ALSO spread as its own namespace → `useTranslation('employees')` + `t('title')`
 * Keys use `.` as the key separator and never contain `:`, so the namespaces never collide.
 */
export const resources: Record<Locale, { translation: Messages } & Messages> = {
  id: { translation: id, ...id },
  en: { translation: en, ...en },
};

export type { Messages };
export { id, en };
