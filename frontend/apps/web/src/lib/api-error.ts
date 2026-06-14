import { ApiError } from '@swp/api-client';
import type { UseFormSetError } from 'react-hook-form';

/**
 * The single ApiError → UX classifier (ENGINEERING.md B1). Components switch on the
 * returned `kind` to render the right state; field errors flow into React Hook Form.
 */
export type ErrorKind =
  | 'unauthenticated'
  | 'forbidden'
  | 'not-found'
  | 'conflict'
  | 'invariant'
  | 'rule'
  | 'validation'
  | 'network'
  | 'unknown';

export function classifyError(error: unknown): { kind: ErrorKind; message: string } {
  if (!(error instanceof ApiError)) {
    return { kind: 'unknown', message: 'errors.unknown' };
  }
  if (error.status === 0) return { kind: 'network', message: 'errors.network' };
  if (error.isUnauthenticated) return { kind: 'unauthenticated', message: 'auth.sessionExpired' };
  if (error.isForbidden) return { kind: 'forbidden', message: 'errors.forbidden' };
  if (error.status === 404) return { kind: 'not-found', message: 'errors.notFound' };
  if (error.isInvariantViolation) return { kind: 'invariant', message: error.message };
  if (error.isConflict) return { kind: 'conflict', message: 'errors.conflict' };
  if (error.fields) return { kind: 'validation', message: error.message };
  if (error.isRuleViolation) return { kind: 'rule', message: error.message };
  return { kind: 'unknown', message: error.message };
}

/** Push an ApiError's `error.fields` into a React Hook Form (CONVENTIONS §11). */
export function applyFieldErrors<T extends Record<string, unknown>>(
  error: unknown,
  setError: UseFormSetError<T>,
): boolean {
  if (error instanceof ApiError && error.fields) {
    for (const [field, message] of Object.entries(error.fields)) {
      setError(field as Parameters<UseFormSetError<T>>[0], { type: 'server', message });
    }
    return true;
  }
  return false;
}
