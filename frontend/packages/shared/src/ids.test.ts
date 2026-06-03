import { describe, expect, it } from 'vitest';
import { ID_PREFIX, asId, isId, prefixOf } from './ids.ts';

describe('SWP ids', () => {
  it('accepts a well-formed prefixed id', () => {
    expect(isId(ID_PREFIX.EMPLOYEE, 'SWP-EMP-1042')).toBe(true);
  });

  it('rejects the wrong prefix', () => {
    expect(isId(ID_PREFIX.PLACEMENT, 'SWP-EMP-1042')).toBe(false);
  });

  it('rejects malformed ids', () => {
    expect(isId(ID_PREFIX.EMPLOYEE, '1042')).toBe(false);
    expect(isId(ID_PREFIX.EMPLOYEE, 'SWP-EMP-')).toBe(false);
  });

  it('asId throws on mismatch', () => {
    expect(() => asId(ID_PREFIX.PLACEMENT, 'SWP-EMP-1042')).toThrow();
  });

  it('extracts the prefix', () => {
    expect(prefixOf('SWP-PL-882')).toBe('SWP-PL');
    expect(prefixOf('nope')).toBeNull();
  });
});
