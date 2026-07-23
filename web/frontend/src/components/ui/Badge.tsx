// Badge component for displaying status indicators and labels.

import type { ReactNode } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

export type BadgeVariant =
  | 'default'
  | 'success'
  | 'warning'
  | 'error'
  | 'info'
  | 'outline';

export type BadgeSize = 'sm' | 'md' | 'lg';

export interface BadgeProps {
  children: ReactNode;
  variant?: BadgeVariant | undefined;
  size?: BadgeSize | undefined;
  className?: string | undefined;
  /** Show a pulsing dot indicator. */
  withDot?: boolean | undefined;
  /** Custom dot color class. */
  dotColor?: string | undefined;
}

// Variant styles mapping.
const variantStyles: Record<BadgeVariant, string> = {
  default: 'bg-[#6B7280]/10 text-[var(--c-mut)]',
  success: 'bg-[#178A5B]/10 text-[var(--c-green)]',
  warning: 'bg-[#C98A1B]/12 text-[var(--c-amber)]',
  error: 'bg-[#B3372B]/10 text-[var(--c-rust)]',
  info: 'bg-[#33608D]/10 text-[var(--c-steel)]',
  outline: 'border border-[var(--c-hair)] text-[var(--c-mut)] bg-transparent',
};

// Dot color mapping for variants.
const dotColors: Record<BadgeVariant, string> = {
  default: 'bg-[var(--c-faint)]',
  success: 'bg-[var(--c-green)]',
  warning: 'bg-[#C98A1B]',
  error: 'bg-[var(--c-rust)]',
  info: 'bg-[var(--c-steel)]',
  outline: 'bg-[var(--c-faint)]',
};

// Size styles mapping.
const sizeStyles: Record<BadgeSize, string> = {
  sm: 'px-1.5 py-0.5 text-xs',
  md: 'px-2 py-0.5 text-xs',
  lg: 'px-2.5 py-1 text-sm',
};

export function Badge({
  children,
  variant = 'default',
  size = 'md',
  className,
  withDot = false,
  dotColor,
}: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full font-medium',
        variantStyles[variant],
        sizeStyles[size],
        className,
      )}
    >
      {withDot ? (
        <span
          className={cn(
            'h-1.5 w-1.5 rounded-full',
            dotColor ?? dotColors[variant],
          )}
          aria-hidden="true"
        />
      ) : null}
      {children}
    </span>
  );
}

// Status badge specifically for agent status.
export type AgentStatus = 'active' | 'busy' | 'idle' | 'offline';

const agentStatusConfig: Record<AgentStatus, { variant: BadgeVariant; label: string; dotColor: string }> = {
  active: { variant: 'success', label: 'Active', dotColor: 'bg-[var(--c-green)] animate-pulse' },
  busy: { variant: 'warning', label: 'Busy', dotColor: 'bg-[#C98A1B]' },
  idle: { variant: 'default', label: 'Idle', dotColor: 'bg-[var(--c-faint)]' },
  offline: { variant: 'outline', label: 'Offline', dotColor: 'bg-[var(--c-dim)]' },
};

export interface StatusBadgeProps {
  status: AgentStatus;
  size?: BadgeSize | undefined;
  className?: string | undefined;
  showLabel?: boolean | undefined;
}

export function StatusBadge({
  status,
  size = 'md',
  className,
  showLabel = true,
}: StatusBadgeProps) {
  const config = agentStatusConfig[status];

  return (
    <Badge
      variant={config.variant}
      size={size}
      withDot
      dotColor={config.dotColor}
      className={className}
    >
      {showLabel ? config.label : null}
    </Badge>
  );
}

// Priority badge for messages.
export type MessagePriority = 'low' | 'normal' | 'high' | 'urgent';

const priorityConfig: Record<MessagePriority, { variant: BadgeVariant; label: string }> = {
  low: { variant: 'outline', label: 'Low' },
  normal: { variant: 'default', label: 'Normal' },
  high: { variant: 'warning', label: 'High' },
  urgent: { variant: 'error', label: 'Urgent' },
};

export interface PriorityBadgeProps {
  priority: MessagePriority;
  size?: BadgeSize | undefined;
  className?: string | undefined;
}

export function PriorityBadge({
  priority,
  size = 'sm',
  className,
}: PriorityBadgeProps) {
  const config = priorityConfig[priority];

  return (
    <Badge variant={config.variant} size={size} className={className}>
      {config.label}
    </Badge>
  );
}
