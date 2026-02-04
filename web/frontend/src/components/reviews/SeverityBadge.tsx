// SeverityBadge component - displays issue severity with color-coded badge.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { IssueSeverity } from '@/types/api.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Color mapping for severity levels.
const severityStyles: Record<IssueSeverity, string> = {
  critical: 'bg-red-100 text-red-800',
  major: 'bg-orange-100 text-orange-800',
  minor: 'bg-yellow-100 text-yellow-800',
  suggestion: 'bg-blue-100 text-blue-800',
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
        severityStyles[severity] ?? 'bg-gray-100 text-gray-600',
        className,
      )}
    >
      <span className="font-bold">{severityIcons[severity]}</span>
      {severity}
    </span>
  );
}
