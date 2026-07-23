// ReviewIssueCard component - displays a single review issue with details.

import { useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { ReviewIssue, IssueStatus } from '@/types/api.js';
import { SeverityBadge } from './SeverityBadge.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Status icon mapping.
const statusIcons: Record<IssueStatus, string> = {
  open: 'O',
  fixed: 'F',
  wont_fix: 'W',
  duplicate: 'D',
};

// Status colors.
const statusStyles: Record<IssueStatus, string> = {
  open: 'text-[var(--c-rust)]',
  fixed: 'text-[var(--c-green)]',
  wont_fix: 'text-[var(--c-mut)]',
  duplicate: 'text-[var(--c-mut)]',
};

// Status labels.
const statusLabels: Record<IssueStatus, string> = {
  open: 'Open',
  fixed: 'Fixed',
  wont_fix: "Won't Fix",
  duplicate: 'Duplicate',
};

export interface ReviewIssueCardProps {
  issue: ReviewIssue;
  onStatusChange?: (issueId: number, status: IssueStatus) => void;
  isUpdating?: boolean;
}

export function ReviewIssueCard({
  issue,
  onStatusChange,
  isUpdating = false,
}: ReviewIssueCardProps) {
  const [expanded, setExpanded] = useState(false);

  const fileLocation = issue.line_start > 0
    ? `${issue.file_path}:${issue.line_start}${issue.line_end > issue.line_start ? `-${issue.line_end}` : ''}`
    : issue.file_path;

  return (
    <div
      className={cn(
        'rounded-lg border border-[var(--c-hair)] bg-[var(--c-card)] p-4',
        'transition-shadow hover:shadow-sm',
        issue.status === 'fixed' && 'opacity-60',
      )}
    >
      {/* Header row with title, severity, and status. */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <SeverityBadge severity={issue.severity} />
            <span className="rounded bg-[var(--c-fill)] px-1.5 py-0.5 text-xs text-[var(--c-mut)]">
              {issue.issue_type}
            </span>
          </div>
          <h4 className="mt-1.5 text-sm font-medium text-[var(--c-ink)]">
            {issue.title}
          </h4>
        </div>

        {/* Status indicator. */}
        <div className="flex items-center gap-2">
          <span
            className={cn(
              'text-xs font-medium',
              statusStyles[issue.status],
            )}
          >
            [{statusIcons[issue.status]}] {statusLabels[issue.status]}
          </span>
        </div>
      </div>

      {/* File location. */}
      {issue.file_path ? (
        <div className="mt-2">
          <code className="text-xs text-[var(--c-steel)]">{fileLocation}</code>
        </div>
      ) : null}

      {/* Expandable details. */}
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="mt-2 text-xs font-medium text-[var(--c-mut)] hover:text-[var(--c-ink)]"
      >
        {expanded ? 'Hide details' : 'Show details'}
      </button>

      {expanded ? (
        <div className="mt-3 space-y-3">
          {/* Description. */}
          {issue.description ? (
            <div>
              <h5 className="text-xs font-medium text-[var(--c-mut)]">Description</h5>
              <p className="mt-1 text-sm text-[var(--c-ink)] whitespace-pre-wrap">
                {issue.description}
              </p>
            </div>
          ) : null}

          {/* Code snippet. */}
          {issue.code_snippet ? (
            <div>
              <h5 className="text-xs font-medium text-[var(--c-mut)]">Code</h5>
              <pre className="mt-1 overflow-x-auto rounded bg-[var(--c-hover)] p-3 text-xs text-[var(--c-ink)]">
                {issue.code_snippet}
              </pre>
            </div>
          ) : null}

          {/* Suggestion. */}
          {issue.suggestion ? (
            <div>
              <h5 className="text-xs font-medium text-[var(--c-mut)]">Suggestion</h5>
              <p className="mt-1 text-sm text-[var(--c-green)] whitespace-pre-wrap">
                {issue.suggestion}
              </p>
            </div>
          ) : null}

          {/* CLAUDE.md reference. */}
          {issue.claude_md_ref ? (
            <div>
              <h5 className="text-xs font-medium text-[var(--c-mut)]">
                CLAUDE.md Reference
              </h5>
              <p className="mt-1 text-sm italic text-[var(--c-mut)]">
                {issue.claude_md_ref}
              </p>
            </div>
          ) : null}

          {/* Status change buttons. */}
          {onStatusChange && issue.status === 'open' ? (
            <div className="flex gap-2 pt-2 border-t border-[var(--c-fill)]">
              <button
                type="button"
                onClick={() => onStatusChange(issue.id, 'fixed')}
                disabled={isUpdating}
                className={cn(
                  'rounded px-3 py-1 text-xs font-medium',
                  'bg-[#178A5B]/10 text-[var(--c-green)] hover:bg-[#178A5B]/15',
                  'disabled:opacity-50 disabled:cursor-not-allowed',
                )}
              >
                Mark Fixed
              </button>
              <button
                type="button"
                onClick={() => onStatusChange(issue.id, 'wont_fix')}
                disabled={isUpdating}
                className={cn(
                  'rounded px-3 py-1 text-xs font-medium',
                  'bg-[var(--c-fill)] text-[var(--c-mut)] hover:bg-[var(--c-hair)]',
                  'disabled:opacity-50 disabled:cursor-not-allowed',
                )}
              >
                Won't Fix
              </button>
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
