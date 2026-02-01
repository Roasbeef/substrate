// Badge components for review-related displays.

import { Badge, type BadgeVariant } from '@/components/ui/Badge.js';
import type {
  ReviewState,
  ReviewDecision,
  IssueSeverity,
  IssueType,
  IssueStatus,
} from '@/types/reviews.js';

// Review state badge configuration.
const reviewStateConfig: Record<
  ReviewState,
  { variant: BadgeVariant; label: string; dotColor?: string }
> = {
  new: { variant: 'default', label: 'New' },
  pending_review: { variant: 'warning', label: 'Pending', dotColor: 'bg-yellow-400' },
  under_review: {
    variant: 'info',
    label: 'Reviewing',
    dotColor: 'bg-blue-500 animate-pulse',
  },
  changes_requested: { variant: 'error', label: 'Changes', dotColor: 'bg-orange-500' },
  re_review: { variant: 'warning', label: 'Re-review', dotColor: 'bg-yellow-400' },
  approved: { variant: 'success', label: 'Approved', dotColor: 'bg-green-500' },
  rejected: { variant: 'error', label: 'Rejected' },
  cancelled: { variant: 'outline', label: 'Cancelled' },
};

export interface ReviewStateBadgeProps {
  state: ReviewState;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
  showDot?: boolean;
}

export function ReviewStateBadge({
  state,
  size = 'md',
  className,
  showDot = true,
}: ReviewStateBadgeProps) {
  const config = reviewStateConfig[state];

  return (
    <Badge
      variant={config.variant}
      size={size}
      className={className}
      withDot={showDot && !!config.dotColor}
      dotColor={config.dotColor}
    >
      {config.label}
    </Badge>
  );
}

// Review decision badge configuration.
const decisionConfig: Record<
  ReviewDecision,
  { variant: BadgeVariant; label: string; icon?: string }
> = {
  approve: { variant: 'success', label: 'Approved', icon: 'check' },
  request_changes: { variant: 'warning', label: 'Changes Requested', icon: 'warning' },
  comment: { variant: 'info', label: 'Commented', icon: 'comment' },
};

export interface DecisionBadgeProps {
  decision: ReviewDecision;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
  showIcon?: boolean;
}

// Check icon.
function CheckIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="currentColor" viewBox="0 0 20 20">
      <path
        fillRule="evenodd"
        d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
        clipRule="evenodd"
      />
    </svg>
  );
}

// Warning icon.
function WarningIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="currentColor" viewBox="0 0 20 20">
      <path
        fillRule="evenodd"
        d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
        clipRule="evenodd"
      />
    </svg>
  );
}

// Comment icon.
function CommentIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="currentColor" viewBox="0 0 20 20">
      <path
        fillRule="evenodd"
        d="M18 10c0 3.866-3.582 7-8 7a8.841 8.841 0 01-4.083-.98L2 17l1.338-3.123C2.493 12.767 2 11.434 2 10c0-3.866 3.582-7 8-7s8 3.134 8 7zM7 9H5v2h2V9zm8 0h-2v2h2V9zM9 9h2v2H9V9z"
        clipRule="evenodd"
      />
    </svg>
  );
}

export function DecisionBadge({
  decision,
  size = 'md',
  className,
  showIcon = true,
}: DecisionBadgeProps) {
  const config = decisionConfig[decision];

  const iconClass = size === 'sm' ? 'h-3 w-3' : 'h-4 w-4';

  let icon = null;
  if (showIcon) {
    switch (config.icon) {
      case 'check':
        icon = <CheckIcon className={iconClass} />;
        break;
      case 'warning':
        icon = <WarningIcon className={iconClass} />;
        break;
      case 'comment':
        icon = <CommentIcon className={iconClass} />;
        break;
    }
  }

  return (
    <Badge variant={config.variant} size={size} className={className}>
      {icon}
      {config.label}
    </Badge>
  );
}

// Issue severity badge configuration.
const severityConfig: Record<IssueSeverity, { variant: BadgeVariant; label: string }> = {
  critical: { variant: 'error', label: 'Critical' },
  high: { variant: 'warning', label: 'High' },
  medium: { variant: 'info', label: 'Medium' },
  low: { variant: 'default', label: 'Low' },
};

export interface SeverityBadgeProps {
  severity: IssueSeverity;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

export function SeverityBadge({ severity, size = 'sm', className }: SeverityBadgeProps) {
  const config = severityConfig[severity];

  return (
    <Badge variant={config.variant} size={size} className={className}>
      {config.label}
    </Badge>
  );
}

// Issue type badge configuration.
const issueTypeConfig: Record<IssueType, { variant: BadgeVariant; label: string }> = {
  bug: { variant: 'error', label: 'Bug' },
  security: { variant: 'error', label: 'Security' },
  claude_md_violation: { variant: 'warning', label: 'CLAUDE.md' },
  logic_error: { variant: 'warning', label: 'Logic' },
};

export interface IssueTypeBadgeProps {
  type: IssueType;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

export function IssueTypeBadge({ type, size = 'sm', className }: IssueTypeBadgeProps) {
  const config = issueTypeConfig[type];

  return (
    <Badge variant={config.variant} size={size} className={className}>
      {config.label}
    </Badge>
  );
}

// Issue status badge configuration.
const issueStatusConfig: Record<IssueStatus, { variant: BadgeVariant; label: string }> = {
  open: { variant: 'warning', label: 'Open' },
  fixed: { variant: 'success', label: 'Fixed' },
  wont_fix: { variant: 'default', label: "Won't Fix" },
  duplicate: { variant: 'outline', label: 'Duplicate' },
};

export interface IssueStatusBadgeProps {
  status: IssueStatus;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

export function IssueStatusBadge({ status, size = 'sm', className }: IssueStatusBadgeProps) {
  const config = issueStatusConfig[status];

  return (
    <Badge variant={config.variant} size={size} className={className}>
      {config.label}
    </Badge>
  );
}
