import { ApiError } from '@swp/api-client';
import { describe, expect, it } from 'vitest';
import { classifyError } from './api-error.ts';

const err = (status: number, code = 'X', fields?: Record<string, string>) =>
  new ApiError(status, { error: { code, message: 'm', fields } });

describe('classifyError', () => {
  it('maps 401 to unauthenticated', () => {
    expect(classifyError(err(401)).kind).toBe('unauthenticated');
  });
  it('maps 403 to forbidden', () => {
    expect(classifyError(err(403)).kind).toBe('forbidden');
  });
  it('maps INV_* 409 to invariant', () => {
    expect(classifyError(err(409, 'INV_1_VIOLATION')).kind).toBe('invariant');
  });
  it('maps plain 409 to conflict', () => {
    expect(classifyError(err(409, 'CONFLICT')).kind).toBe('conflict');
  });
  it('maps 422 with fields to validation', () => {
    expect(classifyError(err(422, 'QUOTA_EXCEEDED', { duration_days: 'x' })).kind).toBe(
      'validation',
    );
  });
  it('maps network (status 0) to network', () => {
    expect(classifyError(err(0)).kind).toBe('network');
  });
  it('maps unknown errors', () => {
    expect(classifyError(new Error('boom')).kind).toBe('unknown');
  });
});
