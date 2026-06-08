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
    tabs: {
      home: 'Beranda',
      attendance: 'Kehadiran',
      notifications: 'Notifikasi',
      more: 'Lainnya',
    },
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
    clock: {
      title: 'Absen',
      clockedInAt: 'Masuk pukul {{time}}',
      notClockedIn: 'Belum absen masuk hari ini',
      clockIn: 'Absen Masuk',
      clockOut: 'Absen Keluar',
      acquiring: 'Mengambil lokasi…',
      gpsDenied: 'Izin lokasi ditolak. Aktifkan lokasi untuk absen.',
      outsideTitle: 'Di luar area lokasi',
      outsideMsg: 'Anda {{distance}} m dari titik lokasi (radius {{radius}} m). Tetap absen masuk?',
      clockAnyway: 'Tetap absen',
      cancel: 'Batal',
      successIn: 'Absen masuk berhasil.',
      successOut: 'Absen keluar berhasil.',
      alreadyIn: 'Anda sudah absen masuk.',
      notIn: 'Anda belum absen masuk.',
      gpsUnavailable: 'Lokasi tidak tersedia. Coba lagi.',
      error: 'Gagal absen. Coba lagi.',
    },
    attendance: {
      title: 'Kehadiran',
      historyTitle: 'Riwayat kehadiran',
      empty: 'Belum ada catatan kehadiran.',
      in: 'Masuk',
      out: 'Keluar',
      lateMin: 'Terlambat {{count}} mnt',
      worked: '{{count}} mnt kerja',
      status: {
        PRESENT: 'Hadir',
        LATE: 'Terlambat',
        INCOMPLETE: 'Tidak lengkap',
        ABSENT: 'Absen',
        ON_LEAVE: 'Cuti',
      },
      flag: {
        LATE: 'Terlambat',
        EARLY: 'Pulang cepat',
        OUTSIDE_GEOFENCE: 'Di luar area',
        UNSCHEDULED: 'Tanpa jadwal',
      },
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
    tabs: {
      home: 'Home',
      attendance: 'Attendance',
      notifications: 'Notifications',
      more: 'More',
    },
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
    clock: {
      title: 'Clock',
      clockedInAt: 'Clocked in at {{time}}',
      notClockedIn: 'Not clocked in today',
      clockIn: 'Clock in',
      clockOut: 'Clock out',
      acquiring: 'Getting location…',
      gpsDenied: 'Location permission denied. Enable location to clock in.',
      outsideTitle: 'Outside site area',
      outsideMsg: 'You are {{distance}} m from the site (radius {{radius}} m). Clock in anyway?',
      clockAnyway: 'Clock in anyway',
      cancel: 'Cancel',
      successIn: 'Clocked in.',
      successOut: 'Clocked out.',
      alreadyIn: 'You are already clocked in.',
      notIn: 'You are not clocked in.',
      gpsUnavailable: 'Location unavailable. Try again.',
      error: 'Clock action failed. Try again.',
    },
    attendance: {
      title: 'Attendance',
      historyTitle: 'Attendance history',
      empty: 'No attendance records yet.',
      in: 'In',
      out: 'Out',
      lateMin: '{{count}} min late',
      worked: '{{count}} min worked',
      status: {
        PRESENT: 'Present',
        LATE: 'Late',
        INCOMPLETE: 'Incomplete',
        ABSENT: 'Absent',
        ON_LEAVE: 'On leave',
      },
      flag: {
        LATE: 'Late',
        EARLY: 'Left early',
        OUTSIDE_GEOFENCE: 'Outside area',
        UNSCHEDULED: 'Unscheduled',
      },
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
