// Avatar component for displaying user or agent profile images.

import type { ReactNode } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

export type AvatarSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl';

export interface AvatarProps {
  /** Image source URL. */
  src?: string | undefined;
  /** Alt text for the image. */
  alt?: string | undefined;
  /** Fallback initials to display when no image. */
  initials?: string | undefined;
  /** Name to generate initials from (alternative to initials prop). */
  name?: string | undefined;
  /** Size of the avatar. */
  size?: AvatarSize | undefined;
  /** Additional class names. */
  className?: string | undefined;
  /** Status indicator to show. */
  status?: 'online' | 'offline' | 'busy' | 'away' | undefined;
}

// Size styles mapping.
const sizeStyles: Record<AvatarSize, string> = {
  xs: 'h-6 w-6 text-xs',
  sm: 'h-8 w-8 text-sm',
  md: 'h-10 w-10 text-base',
  lg: 'h-12 w-12 text-lg',
  xl: 'h-16 w-16 text-xl',
};

// Status dot size mapping.
const statusSizeStyles: Record<AvatarSize, string> = {
  xs: 'h-1.5 w-1.5 ring-1',
  sm: 'h-2 w-2 ring-1',
  md: 'h-2.5 w-2.5 ring-2',
  lg: 'h-3 w-3 ring-2',
  xl: 'h-4 w-4 ring-2',
};

// Status color mapping.
const statusColors: Record<string, string> = {
  online: 'bg-green-500',
  offline: 'bg-gray-400',
  busy: 'bg-red-500',
  away: 'bg-yellow-500',
};

// Generate background color from initials for consistent colors.
function getInitialsBgColor(initials: string): string {
  const colors = [
    'bg-blue-500',
    'bg-green-500',
    'bg-yellow-500',
    'bg-red-500',
    'bg-purple-500',
    'bg-pink-500',
    'bg-indigo-500',
    'bg-cyan-500',
    'bg-teal-500',
    'bg-orange-500',
  ];

  let hash = 0;
  for (let i = 0; i < initials.length; i++) {
    hash = initials.charCodeAt(i) + ((hash << 5) - hash);
  }
  const index = Math.abs(hash) % colors.length;
  return colors[index] ?? 'bg-gray-500';
}

// Get initials from a name.
export function getInitials(name: string, maxLength = 2): string {
  return name
    .split(/\s+/)
    .map((word) => word.charAt(0).toUpperCase())
    .slice(0, maxLength)
    .join('');
}

export function Avatar({
  src,
  alt = '',
  initials,
  name,
  size = 'md',
  className,
  status,
}: AvatarProps) {
  // Generate initials from name if not provided directly.
  const displayInitials = initials ?? (name ? getInitials(name) : undefined);
  const bgColor = displayInitials ? getInitialsBgColor(displayInitials) : 'bg-gray-400';

  return (
    <div className={cn('relative inline-flex', className)}>
      {src ? (
        <img
          src={src}
          alt={alt}
          className={cn(
            'rounded-full object-cover',
            sizeStyles[size],
          )}
        />
      ) : (
        <div
          className={cn(
            'flex items-center justify-center rounded-full font-medium text-white',
            bgColor,
            sizeStyles[size],
          )}
          aria-label={alt}
        >
          {displayInitials ?? '?'}
        </div>
      )}
      {status ? (
        <span
          className={cn(
            'absolute bottom-0 right-0 rounded-full ring-white',
            statusColors[status] ?? 'bg-gray-400',
            statusSizeStyles[size],
          )}
          aria-label={`Status: ${status}`}
        />
      ) : null}
    </div>
  );
}

// Avatar group for stacking multiple avatars.
export interface AvatarGroupProps {
  children: ReactNode;
  max?: number | undefined;
  size?: AvatarSize | undefined;
  className?: string | undefined;
}

export function AvatarGroup({
  children,
  max,
  size = 'md',
  className,
}: AvatarGroupProps) {
  const childArray = Array.isArray(children) ? children : [children];
  const visibleChildren = max !== undefined ? childArray.slice(0, max) : childArray;
  const remainingCount = max !== undefined ? childArray.length - max : 0;

  return (
    <div className={cn('flex -space-x-2', className)}>
      {visibleChildren.map((child, index) => (
        <div
          key={index}
          className="relative ring-2 ring-white rounded-full"
          style={{ zIndex: visibleChildren.length - index }}
        >
          {child}
        </div>
      ))}
      {remainingCount > 0 ? (
        <div
          className={cn(
            'relative flex items-center justify-center rounded-full bg-gray-200 text-gray-600 font-medium ring-2 ring-white',
            sizeStyles[size],
          )}
        >
          +{remainingCount}
        </div>
      ) : null}
    </div>
  );
}
