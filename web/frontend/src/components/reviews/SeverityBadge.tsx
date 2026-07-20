// SeverityBadge component - displays issue severity with color-coded badge.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { IssueSeverity } from '@/types/api.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Color mapping for severity levels.
const severityStyles: Record<IssueSeverity, string> = {
  critical: 'bg-[#B3372B]/10 text-[var(--c-rust)]',
  major: 'bg-[#C98A1B]/12 text-[var(--c-amber)]',
  minor: 'bg-[#33608D]/10 text-[var(--c-steel)]',
  suggestion: 'bg-[#6B7280]/10 text-[var(--c-mut)]',
};

// Icon for severity levels.
const severityIcons: Record<IssueSeverity, string> = {
  critical: '!!',
  major: '!',
  minor: '~',
  suggestion: '?',
};

export interface SeverityBadgeProps {
  severity: IssueSeverity;
  className?: string;
}

export function SeverityBadge({ severity, className }: SeverityBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium',
        severityStyles[severity] ?? 'bg-[#6B7280]/10 text-[var(--c-mut)]',
        className,
      )}
    >
      <span className="font-bold">{severityIcons[severity]}</span>
      {severity}
    </span>
  );
}
