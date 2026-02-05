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
  open: 'text-red-600',
  fixed: 'text-green-600',
  wont_fix: 'text-gray-500',
  duplicate: 'text-gray-500',
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
        'rounded-lg border border-gray-200 bg-white p-4',
        'transition-shadow hover:shadow-sm',
        issue.status === 'fixed' && 'opacity-60',
      )}
    >
      {/* Header row with title, severity, and status. */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <SeverityBadge severity={issue.severity} />
            <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-600">
              {issue.issue_type}
            </span>
          </div>
          <h4 className="mt-1.5 text-sm font-medium text-gray-900">
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
          <code className="text-xs text-blue-600">{fileLocation}</code>
        </div>
      ) : null}

      {/* Expandable details. */}
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="mt-2 text-xs font-medium text-gray-500 hover:text-gray-700"
      >
        {expanded ? 'Hide details' : 'Show details'}
      </button>

      {expanded ? (
        <div className="mt-3 space-y-3">
          {/* Description. */}
          {issue.description ? (
            <div>
              <h5 className="text-xs font-medium text-gray-500">Description</h5>
              <p className="mt-1 text-sm text-gray-700 whitespace-pre-wrap">
                {issue.description}
              </p>
            </div>
          ) : null}

          {/* Code snippet. */}
          {issue.code_snippet ? (
            <div>
              <h5 className="text-xs font-medium text-gray-500">Code</h5>
              <pre className="mt-1 overflow-x-auto rounded bg-gray-50 p-3 text-xs text-gray-800">
                {issue.code_snippet}
              </pre>
            </div>
          ) : null}

          {/* Suggestion. */}
          {issue.suggestion ? (
            <div>
              <h5 className="text-xs font-medium text-gray-500">Suggestion</h5>
              <p className="mt-1 text-sm text-green-700 whitespace-pre-wrap">
                {issue.suggestion}
              </p>
            </div>
          ) : null}

          {/* CLAUDE.md reference. */}
          {issue.claude_md_ref ? (
            <div>
              <h5 className="text-xs font-medium text-gray-500">
                CLAUDE.md Reference
              </h5>
              <p className="mt-1 text-sm italic text-gray-600">
                {issue.claude_md_ref}
              </p>
            </div>
          ) : null}

          {/* Status change buttons. */}
          {onStatusChange && issue.status === 'open' ? (
            <div className="flex gap-2 pt-2 border-t border-gray-100">
              <button
                type="button"
                onClick={() => onStatusChange(issue.id, 'fixed')}
                disabled={isUpdating}
                className={cn(
                  'rounded px-3 py-1 text-xs font-medium',
                  'bg-green-50 text-green-700 hover:bg-green-100',
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
                  'bg-gray-50 text-gray-600 hover:bg-gray-100',
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
