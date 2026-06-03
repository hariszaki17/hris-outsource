/**
 * Transport-level error type + envelope parsing. CONVENTIONS §11 · ENGINEERING.md B1.
 * One place parses `ErrorEnvelope`; the app's error mapper switches on `code`/`status`.
 */

export interface ErrorEnvelope {
  error: {
    code: string;
    message: string;
    fields?: Record<string, string>;
    request_id?: string;
  };
}

/** Thrown by the fetch mutator for any non-2xx response. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields?: Record<string, string>;
  readonly requestId?: string;

  constructor(status: number, envelope?: ErrorEnvelope, fallbackMessage?: string) {
    super(envelope?.error.message ?? fallbackMessage ?? `HTTP ${status}`);
    this.name = 'ApiError';
    this.status = status;
    this.code = envelope?.error.code ?? 'UNKNOWN';
    this.fields = envelope?.error.fields;
    this.requestId = envelope?.error.request_id;
  }

  get isUnauthenticated() {
    return this.status === 401;
  }
  get isForbidden() {
    return this.status === 403;
  }
  get isConflict() {
    return this.status === 409;
  }
  /** Semantic business-rule rejection (quota, geofence, etc.). */
  get isRuleViolation() {
    return this.status === 422;
  }
  /** Placement/scheduling invariant violation (INV_*_VIOLATION). */
  get isInvariantViolation() {
    return this.status === 409 && this.code.startsWith('INV_');
  }
}

/** Build an ApiError from a failed Response, tolerating non-JSON bodies. */
export async function parseErrorEnvelope(res: Response): Promise<ApiError> {
  let envelope: ErrorEnvelope | undefined;
  try {
    const body = await res.json();
    if (body && typeof body === 'object' && 'error' in body) {
      envelope = body as ErrorEnvelope;
    }
  } catch {
    // non-JSON / empty body — fall back to status
  }
  return new ApiError(res.status, envelope);
}
