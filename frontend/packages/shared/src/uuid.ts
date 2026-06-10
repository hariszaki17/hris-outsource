/**
 * Cross-platform UUID v4 generator.
 *
 * Uses `crypto.randomUUID()` when available (modern browsers, Node ≥19).
 * Falls back to `crypto.getRandomValues()` + hex template (React Native Hermes).
 *
 * Never throws — environments without `crypto` at all return a fallback
 * (Math.random-based). That path is for edge runtimes that suppress `crypto`;
 * it is NOT used on any platform we target.
 */

export function uuid(): string {
  // Fast path — available on modern web + Node ≥19
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }

  // Hermes (React Native) — has getRandomValues but not randomUUID
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    // RFC 4122 version 4
    bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
    bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant 10
    return hex(bytes);
  }

  // Last-resort fallback (should never hit on any supported target)
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16);
  });
}

function hex(bytes: Uint8Array): string {
  const parts: string[] = [];
  for (let i = 0; i < bytes.length; i++) {
    parts.push(bytes[i].toString(16).padStart(2, '0'));
  }
  return [
    parts.slice(0, 4).join(''),
    parts.slice(4, 6).join(''),
    parts.slice(6, 8).join(''),
    parts.slice(8, 10).join(''),
    parts.slice(10, 16).join(''),
  ].join('-');
}
