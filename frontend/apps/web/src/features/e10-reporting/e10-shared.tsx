/**
 * E10 Reporting & Notifications — shared icon/tone maps (pre-created so the parallel screen
 * agents share one canonical mapping). Status color only via StatusBadge tones (G4).
 */
import { ApprovalInboxRowKind, ExportStatus, NotificationKind } from '@swp/api-client/e10';
import type { StatusTone } from '@swp/design-tokens';
import {
  AlarmClock,
  Bell,
  CalendarClock,
  CalendarX,
  ClipboardCheck,
  Download,
  FileWarning,
  Inbox,
  type LucideIcon,
  MapPin,
  Plane,
  Timer,
  UserCog,
} from 'lucide-react';

/** Notification kind → lucide icon for the NotifCard chip. */
export function notifKindIcon(kind: NotificationKind): LucideIcon {
  switch (kind) {
    case NotificationKind.SCHEDULE_PUBLISHED:
    case NotificationKind.SCHEDULE_CHANGED:
      return CalendarClock;
    case NotificationKind.SHIFT_REMINDER:
      return AlarmClock;
    case NotificationKind.LEAVE_REQUEST_SUBMITTED:
    case NotificationKind.LEAVE_APPROVED:
    case NotificationKind.LEAVE_REJECTED:
      return Plane;
    case NotificationKind.OT_REQUEST_SUBMITTED:
    case NotificationKind.OT_AUTO_DETECTED:
    case NotificationKind.OT_APPROVED:
    case NotificationKind.OT_REJECTED:
      return Timer;
    case NotificationKind.ATTENDANCE_VERIFY_NEEDED:
    case NotificationKind.ATTENDANCE_CORRECTION_SUBMITTED:
    case NotificationKind.ATTENDANCE_AUTO_CLOSED:
      return ClipboardCheck;
    case NotificationKind.HR_CHANGE_REQUEST_SUBMITTED:
      return UserCog;
    case NotificationKind.AGREEMENT_EXPIRING:
      return CalendarX;
    case NotificationKind.PLACEMENT_EXPIRING:
    case NotificationKind.PLACEMENT_LEADER_CHANGED:
      return MapPin;
    case NotificationKind.EXPORT_READY:
      return Download;
    case NotificationKind.EXPORT_FAILED:
      return FileWarning;
    default:
      return Bell;
  }
}

/** Approval-inbox row kind → lucide icon. */
export function inboxKindIcon(kind: ApprovalInboxRowKind): LucideIcon {
  switch (kind) {
    case ApprovalInboxRowKind.ATTENDANCE_VERIFY:
      return ClipboardCheck;
    case ApprovalInboxRowKind.LEAVE_APPROVE:
      return Plane;
    case ApprovalInboxRowKind.OT_APPROVE:
      return Timer;
    case ApprovalInboxRowKind.PLACEMENT_EXPIRING:
      return MapPin;
    case ApprovalInboxRowKind.AGREEMENT_EXPIRING:
      return CalendarX;
    case ApprovalInboxRowKind.HR_CHANGE_REQUEST:
      return UserCog;
    default:
      return Inbox;
  }
}

/** Export job status → tone. */
export function exportStatusTone(status: ExportStatus): StatusTone {
  switch (status) {
    case ExportStatus.COMPLETED:
      return 'ok';
    case ExportStatus.QUEUED:
    case ExportStatus.PROCESSING:
      return 'onprogress';
    case ExportStatus.FAILED:
      return 'bad';
    case ExportStatus.CANCELLED:
      return 'neutral';
    default:
      return 'neutral';
  }
}

/** Map an export job status to the ExportModal `step`. */
export function exportStatusToStep(status: ExportStatus): 'progress' | 'success' | 'error' {
  switch (status) {
    case ExportStatus.COMPLETED:
      return 'success';
    case ExportStatus.FAILED:
    case ExportStatus.CANCELLED:
      return 'error';
    default:
      return 'progress';
  }
}
