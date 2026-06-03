// Atoms / primitives
export { Button, buttonVariants, type ButtonProps } from './primitives/button.tsx';
export { Input, type InputProps } from './primitives/input.tsx';
export { Checkbox, type CheckboxProps } from './primitives/checkbox.tsx';
export { SearchField, type SearchFieldProps } from './primitives/search-field.tsx';
export { FilterSelect, type FilterSelectProps } from './primitives/filter-select.tsx';
export { Toggle, type ToggleProps } from './primitives/toggle.tsx';

// Molecules — display & status
export {
  StatusBadge,
  toneForAttendance,
  toneForPlacement,
  type StatusBadgeProps,
} from './molecules/status-badge.tsx';
export { IdChip, type IdChipProps } from './molecules/id-chip.tsx';
export { DateText, type DateTextProps } from './molecules/date-text.tsx';
export { Avatar, type AvatarProps } from './molecules/avatar.tsx';
export { StatCard, type StatCardProps, type StatTone } from './molecules/stat-card.tsx';
export { MapPicker, type MapPickerProps, type LatLng } from './molecules/map-picker.tsx';

// Molecules — forms
export { FormField, FormSection, type FormFieldProps } from './molecules/form-field.tsx';

// Molecules — async / feedback states (no dead-flow: ENGINEERING.md B2 / G4)
export { StateView, type StateViewProps } from './molecules/state-view.tsx';
export { Banner, type BannerProps } from './molecules/banner.tsx';
export {
  EmptyState,
  type EmptyStateProps,
  type EmptyVariant,
} from './molecules/empty-state.tsx';
export {
  Skeleton,
  SkeletonCard,
  SkeletonTableRow,
  type SkeletonProps,
  type SkeletonCardProps,
  type SkeletonTableRowProps,
} from './molecules/skeleton.tsx';
export {
  Toast,
  ToastProvider,
  Toaster,
  useToast,
  type ToastTone,
  type ToastProps,
  type ToastProviderProps,
} from './molecules/toast.tsx';

// Molecules — overlays
export {
  Modal,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ConfirmDialog,
  type ModalProps,
  type ModalHeaderProps,
  type ModalBodyProps,
  type ModalFooterProps,
  type ConfirmDialogProps,
  type ModalSize,
  type ModalTone,
  type ConfirmTone,
} from './molecules/modal.tsx';
export {
  Drawer,
  DrawerHeader,
  DrawerBody,
  DrawerFooter,
  type DrawerProps,
  type DrawerHeaderProps,
  type DrawerBodyProps,
  type DrawerFooterProps,
} from './molecules/drawer.tsx';
export { NotifCard, type NotifCardProps } from './molecules/notif-card.tsx';
export {
  ExportModal,
  type ExportModalProps,
  type ExportModalLabels,
  type ExportStep,
  type ExportQuickRange,
  type ExportFilterChip,
  type ExportFileInfo,
} from './molecules/export-modal.tsx';

// Molecules — combobox / async FK picker primitive
export { Combobox, type ComboboxProps, type ComboboxOption } from './molecules/combobox.tsx';

// Molecules — data tables & pagination
export {
  DataTable,
  type Column,
  type DataTableProps,
} from './molecules/data-table.tsx';
export {
  CursorPagination,
  type CursorPaginationProps,
} from './molecules/cursor-pagination.tsx';

// Molecules — app shell navigation
export {
  Sidebar,
  SidebarBrand,
  SidebarSectionLabel,
  SidebarNavItem,
  SidebarSpacer,
  SidebarFooter,
  type SidebarProps,
  type SidebarBrandProps,
  type SidebarSectionLabelProps,
  type SidebarNavItemProps,
  type SidebarSpacerProps,
  type SidebarFooterProps,
} from './molecules/sidebar.tsx';
export {
  Topbar,
  Breadcrumb,
  TopbarSearch,
  TopbarIconButton,
  TopbarUser,
  type TopbarProps,
  type BreadcrumbItem,
  type BreadcrumbProps,
  type TopbarSearchProps,
  type TopbarIconButtonProps,
  type TopbarUserProps,
} from './molecules/topbar.tsx';
export {
  SettingsSubnav,
  SettingsSubnavItem,
  type SettingsSubnavProps,
  type SettingsSubnavItemProps,
} from './molecules/settings-subnav.tsx';

// Molecules — audit trail
export {
  AuditTrailViewer,
  AuditTrailInline,
  AuditTrailDrawer,
  type AuditEntry,
  type AuditEventType,
  type AuditEntryCompact,
  type AuditTrailViewerProps,
  type AuditTrailInlineProps,
  type AuditTrailDrawerProps,
} from './molecules/audit-trail.tsx';

// Utilities
export { cn } from './lib/cn.ts';
