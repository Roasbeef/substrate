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
  default: 'bg-[#6B7280]/10 text-[#6B7280]',
  success: 'bg-[#178A5B]/10 text-[#178A5B]',
  warning: 'bg-[#C98A1B]/12 text-[#92610E]',
  error: 'bg-[#B3372B]/10 text-[#B3372B]',
  info: 'bg-[#33608D]/10 text-[#33608D]',
  outline: 'border border-[#E6E4DD] text-[#6B7280] bg-transparent',
};

// Dot color mapping for variants.
const dotColors: Record<BadgeVariant, string> = {
  default: 'bg-[#9BA0A6]',
  success: 'bg-[#178A5B]',
  warning: 'bg-[#C98A1B]',
  error: 'bg-[#B3372B]',
  info: 'bg-[#33608D]',
  outline: 'bg-[#9BA0A6]',
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
  active: { variant: 'success', label: 'Active', dotColor: 'bg-[#178A5B] animate-pulse' },
  busy: { variant: 'warning', label: 'Busy', dotColor: 'bg-[#C98A1B]' },
  idle: { variant: 'default', label: 'Idle', dotColor: 'bg-[#9BA0A6]' },
  offline: { variant: 'outline', label: 'Offline', dotColor: 'bg-[#B0ADA4]' },
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
