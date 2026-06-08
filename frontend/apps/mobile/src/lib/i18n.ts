// i18next for mobile. Reuses the shared catalogs (@swp/shared/i18n, default Bahasa) and
// layers mobile-only copy on top via addResourceBundle, so the shared package stays untouched.
// All visible strings go through t() (ENGINEERING.md: copy via i18n, Bahasa default).
import { DEFAULT_LOCALE, resources } from '@swp/shared/i18n';
import i18next from 'i18next';
import { initReactI18next } from 'react-i18next';

const mobileCopy = {
  id: {
    common: {
      appName: 'SWP HRIS',
      loading: 'Memuat…',
      retry: 'Coba lagi',
      errorGeneric: 'Terjadi kesalahan. Coba lagi.',
      emptyGeneric: 'Belum ada data.',
      comingSoon: 'Segera hadir',
    },
    login: {
      title: 'Masuk',
      subtitle: 'Masuk dengan akun SWP Anda.',
      identifier: 'Nomor HP atau email',
      password: 'Kata sandi',
      submit: 'Masuk',
      errorInvalid: 'Nomor/sandi salah. Coba lagi.',
      errorLocked: 'Akun terkunci. Hubungi admin.',
    },
    tabs: { home: 'Beranda', notifications: 'Notifikasi', more: 'Lainnya' },
    beranda: {
      greeting: 'Halo, {{name}}',
      todayShift: 'Shift hari ini',
      offToday: 'Tidak ada shift hari ini',
      otMonth: 'Lembur bulan ini',
      hours: '{{count}} jam',
      unreadNotifs: 'Notifikasi belum dibaca',
    },
    notifications: {
      title: 'Notifikasi',
      empty: 'Belum ada notifikasi.',
      markAllRead: 'Tandai semua dibaca',
      unread: '{{count}} belum dibaca',
    },
    more: {
      title: 'Lainnya',
      changePassword: 'Ubah kata sandi',
      signOut: 'Keluar',
    },
  },
  en: {
    common: {
      appName: 'SWP HRIS',
      loading: 'Loading…',
      retry: 'Retry',
      errorGeneric: 'Something went wrong. Try again.',
      emptyGeneric: 'Nothing here yet.',
      comingSoon: 'Coming soon',
    },
    login: {
      title: 'Sign in',
      subtitle: 'Sign in with your SWP account.',
      identifier: 'Phone or email',
      password: 'Password',
      submit: 'Sign in',
      errorInvalid: 'Wrong phone/password. Try again.',
      errorLocked: 'Account locked. Contact your admin.',
    },
    tabs: { home: 'Home', notifications: 'Notifications', more: 'More' },
    beranda: {
      greeting: 'Hi, {{name}}',
      todayShift: "Today's shift",
      offToday: 'No shift today',
      otMonth: 'Overtime this month',
      hours: '{{count}} h',
      unreadNotifs: 'Unread notifications',
    },
    notifications: {
      title: 'Notifications',
      empty: 'No notifications yet.',
      markAllRead: 'Mark all read',
      unread: '{{count}} unread',
    },
    more: {
      title: 'More',
      changePassword: 'Change password',
      signOut: 'Sign out',
    },
  },
} as const;

void i18next.use(initReactI18next).init({
  resources,
  lng: DEFAULT_LOCALE,
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
  returnNull: false,
});

// Layer mobile copy onto each locale under a dedicated `m` namespace (kept out of the shared
// package, and isolated from the shared `translation` keys). Access via t('m:login.title').
for (const [lng, bundle] of Object.entries(mobileCopy)) {
  i18next.addResourceBundle(lng, 'm', bundle, true, true);
}

export default i18next;
