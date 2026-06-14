/**
 * Toast molecule — comp/Toast (.pen base `PtJHa`; tones: ToastSuccess `ofb0U`,
 * ToastError `zaisr`, ToastWarn `d8u3Q`, ToastInfo `onGI4`, ToastQueued `lC1k8`).
 *
 * ENGINEERING.md G4 (interaction catalogue → named molecule): this is the ONE canonical
 * toast surface for transient feedback. G3: tones are a prop — never fork.
 *
 * System: ToastProvider (context) + useToast() hook + Toaster (viewport).
 * Zero external deps — React context + setTimeout only.
 */

import { uuid } from '@swp/shared';
import { CircleCheck, CircleX, Info, LoaderCircle, TriangleAlert, X } from 'lucide-react';
import {
  type ReactNode,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { cn } from '../lib/cn.ts';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type ToastTone = 'success' | 'error' | 'warn' | 'info' | 'queued';

export interface ToastProps {
  tone: ToastTone;
  title: string;
  description?: string;
  onClose?: () => void;
  /** aria-label for the close button. Default: 'Tutup' (i18n key: common.close). */
  closeLabel?: string;
  className?: string;
}

// ---------------------------------------------------------------------------
// Tone maps (tokens only — never raw hex)
// ---------------------------------------------------------------------------

type ToneConfig = {
  border: string;
  iconColor: string;
  Icon: React.ComponentType<{ className?: string }>;
  role: 'status' | 'alert';
};

const toneConfig: Record<ToastTone, ToneConfig> = {
  success: {
    border: 'border-ok-bd',
    iconColor: 'text-ok-tx',
    Icon: CircleCheck,
    role: 'status',
  },
  error: {
    border: 'border-bad-bd',
    iconColor: 'text-bad-tx',
    Icon: CircleX,
    role: 'alert',
  },
  warn: {
    border: 'border-warn-bd',
    iconColor: 'text-warn-tx',
    Icon: TriangleAlert,
    role: 'status',
  },
  info: {
    border: 'border-info-bd',
    iconColor: 'text-info-tx',
    Icon: Info,
    role: 'status',
  },
  queued: {
    border: 'border-info-bd',
    iconColor: 'text-info-tx',
    Icon: LoaderCircle,
    role: 'status',
  },
};

// ---------------------------------------------------------------------------
// Toast — presentational card
// ---------------------------------------------------------------------------

export function Toast({
  tone,
  title,
  description,
  onClose,
  closeLabel = 'Tutup',
  className,
}: ToastProps) {
  const { border, iconColor, Icon, role } = toneConfig[tone];

  return (
    <div
      role={role}
      className={cn(
        'flex w-[340px] items-start gap-3 rounded-lg border bg-surface px-3.5 py-3 shadow-overlay',
        border,
        className,
      )}
    >
      <Icon
        className={cn(
          'mt-0.5 size-[19px] shrink-0',
          iconColor,
          tone === 'queued' && 'animate-spin',
        )}
      />

      <div className="flex flex-1 flex-col gap-0.5">
        <p className="text-[13px] font-bold leading-snug text-text">{title}</p>
        {description && <p className="text-xs leading-[1.4] text-text-2">{description}</p>}
      </div>

      {onClose && (
        <button
          type="button"
          aria-label={closeLabel}
          onClick={onClose}
          className="ml-auto shrink-0 rounded text-text-3 transition-colors hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-info-bd"
        >
          <X className="size-[15px]" />
        </button>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Toast system — context, provider, hook, viewport
// ---------------------------------------------------------------------------

interface ToastItem {
  id: string;
  tone: ToastTone;
  title: string;
  description?: string;
  duration: number;
}

interface ToastContextValue {
  items: ToastItem[];
  dismiss: (id: string) => void;
  add: (item: ToastItem) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export interface ToastProviderProps {
  children: ReactNode;
}

export function ToastProvider({ children }: ToastProviderProps) {
  const [items, setItems] = useState<ToastItem[]>([]);

  const dismiss = useCallback((id: string) => {
    setItems((prev) => prev.filter((item) => item.id !== id));
  }, []);

  const add = useCallback((item: ToastItem) => {
    setItems((prev) => [...prev, item]);
  }, []);

  const value = useMemo(() => ({ items, dismiss, add }), [items, dismiss, add]);

  return <ToastContext.Provider value={value}>{children}</ToastContext.Provider>;
}

// ---------------------------------------------------------------------------
// useToast hook
// ---------------------------------------------------------------------------

interface UseToastReturn {
  toast: (opts: {
    tone: ToastTone;
    title: string;
    description?: string;
    duration?: number;
  }) => string;
  dismiss: (id: string) => void;
}

export function useToast(): UseToastReturn {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error('useToast must be used within a <ToastProvider>.');
  }

  const { add, dismiss } = ctx;

  const toast = useCallback(
    (opts: {
      tone: ToastTone;
      title: string;
      description?: string;
      duration?: number;
    }): string => {
      const id = uuid();
      // queued tone: default to Infinity (no auto-dismiss); others: 5000ms
      const defaultDuration = opts.tone === 'queued' ? Number.POSITIVE_INFINITY : 5000;
      const duration = opts.duration ?? defaultDuration;
      add({ id, tone: opts.tone, title: opts.title, description: opts.description, duration });
      return id;
    },
    [add],
  );

  return { toast, dismiss };
}

// ---------------------------------------------------------------------------
// ToastTimer — handles auto-dismiss for a single item
// ---------------------------------------------------------------------------

interface ToastTimerProps {
  item: ToastItem;
  onDismiss: (id: string) => void;
}

function ToastTimer({ item, onDismiss }: ToastTimerProps) {
  // Keep a stable ref to the latest onDismiss so the timer effect never needs
  // to re-subscribe when the callback identity changes (advanced-event-handler-refs).
  const dismissRef = useRef(onDismiss);
  useEffect(() => {
    dismissRef.current = onDismiss;
  }, [onDismiss]);

  useEffect(() => {
    if (!Number.isFinite(item.duration)) return;
    const timerId = setTimeout(() => dismissRef.current(item.id), item.duration);
    return () => clearTimeout(timerId);
  }, [item.id, item.duration]);

  return (
    <Toast
      tone={item.tone}
      title={item.title}
      description={item.description}
      onClose={() => onDismiss(item.id)}
    />
  );
}

// ---------------------------------------------------------------------------
// Toaster — fixed viewport; place once near root (inside ToastProvider)
// ---------------------------------------------------------------------------

export function Toaster() {
  const ctx = useContext(ToastContext);
  if (!ctx) return null;

  const { items, dismiss } = ctx;

  return (
    <div className="fixed right-4 top-4 z-50 flex flex-col gap-2">
      {items.map((item) => (
        <ToastTimer key={item.id} item={item} onDismiss={dismiss} />
      ))}
    </div>
  );
}
