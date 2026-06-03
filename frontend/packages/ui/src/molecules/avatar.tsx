import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/**
 * Avatar (molecule) — comp/Avatar `YVANc` (38×38 brand, 34×34 neutral variant).
 * Renders initials or an image inside a shaped, toned container.
 *
 * - tone 'brand' (default): bg-primary-soft / text-primary (DESIGN-SYSTEM §2 brand pair)
 * - tone 'neutral': bg-avatar-neutral / text-text-2 (used by Topbar user chip)
 * - shape 'rounded' (default): rounded-md (≈8 px). shape 'circle': rounded-full.
 *
 * Font size and width/height are dynamic; inline style is the correct approach for
 * run-time numeric values (ENGINEERING.md G4 — tokens for color, inline for geometry).
 */
export interface AvatarProps {
  /** One or two characters shown when no src is supplied. Always uppercased. */
  initials: string;
  /** Optional image URL. When present, renders an <img> filling the avatar. */
  src?: string;
  /** Diameter in px. Default 38 (brand spec). */
  size?: number;
  /** Color scheme. 'brand' → bg-primary-soft / text-primary. 'neutral' → bg-avatar-neutral / text-text-2. */
  tone?: 'brand' | 'neutral';
  /** Border-radius. 'rounded' → rounded-md (8 px). 'circle' → rounded-full. */
  shape?: 'rounded' | 'circle';
  className?: string;
}

export function Avatar({
  initials,
  src,
  size = 38,
  tone = 'brand',
  shape = 'rounded',
  className,
}: AvatarProps) {
  const fontSize = Math.round(size * 0.34);

  const toneClass =
    tone === 'neutral' ? 'bg-avatar-neutral text-text-2' : 'bg-primary-soft text-primary';

  const shapeClass = shape === 'circle' ? 'rounded-full' : 'rounded-md';

  const containerClass = cn(
    'inline-flex shrink-0 items-center justify-center overflow-hidden font-semibold',
    toneClass,
    shapeClass,
    className,
  );

  const containerStyle: React.CSSProperties = { width: size, height: size };

  if (src) {
    return (
      <span className={containerClass} style={containerStyle} aria-label={initials.toUpperCase()}>
        <img
          src={src}
          alt={initials.toUpperCase()}
          className={cn('h-full w-full object-cover', shapeClass)}
        />
      </span>
    );
  }

  return (
    <span
      className={containerClass}
      style={{ ...containerStyle, fontSize }}
      aria-label={initials.toUpperCase()}
      role="img"
    >
      {initials.toUpperCase()}
    </span>
  );
}
