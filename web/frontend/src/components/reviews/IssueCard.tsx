// IssueCard component - displays a review issue with expandable details.

import { useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Button } from '@/components/ui/Button.js';
import { SeverityBadge, IssueTypeBadge, IssueStatusBadge } from './ReviewBadges.js';
import type { ReviewIssue, IssueStatus } from '@/types/reviews.js';

// Combine clsx and tailwind-merge.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Severity border colors.
const severityBorderColors = {
  critical: 'border-red-300 bg-red-50',
  high: 'border-orange-300 bg-orange-50',
  medium: 'border-yellow-300 bg-yellow-50',
  low: 'border-blue-300 bg-blue-50',
};

// Chevron icons for expand/collapse.
function ChevronDownIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
    </svg>
  );
}

function ChevronUpIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
    </svg>
  );
}

// File icon.
function FileIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"
      />
    </svg>
  );
}

export interface IssueCardProps {
  issue: ReviewIssue;
  onStatusChange?: (status: IssueStatus) => void;
  isLoading?: boolean;
  compact?: boolean;
  className?: string;
}

export function IssueCard({
  issue,
  onStatusChange,
  isLoading = false,
  compact = false,
  className,
}: IssueCardProps) {
  const [expanded, setExpanded] = useState(!compact);

  const handleMarkFixed = () => {
    onStatusChange?.('fixed');
  };

  const handleMarkWontFix = () => {
    onStatusChange?.('wont_fix');
  };

  const lineRange =
    issue.line_end && issue.line_end !== issue.line_start
      ? `${issue.line_start}-${issue.line_end}`
      : String(issue.line_start);

  return (
    <div
      className={cn(
        'rounded-lg border overflow-hidden transition-all',
        severityBorderColors[issue.severity],
        className,
      )}
    >
      {/* Header - always visible */}
      <div
        className="px-3 py-2 flex items-center justify-between cursor-pointer"
        onClick={() => setExpanded(!expanded)}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            setExpanded(!expanded);
          }
        }}
      >
        <div className="flex items-center gap-2 min-w-0 flex-1">
          <SeverityBadge severity={issue.severity} />
          <IssueTypeBadge type={issue.type} />
          <span className="text-sm font-medium text-gray-900 truncate">{issue.title}</span>
          {issue.status !== 'open' && <IssueStatusBadge status={issue.status} />}
        </div>

        <div className="flex items-center gap-2 flex-shrink-0">
          {/* File location link */}
          <a
            href={`#file-${encodeURIComponent(issue.file_path)}-L${issue.line_start}`}
            className="text-sm text-blue-600 hover:underline flex items-center gap-1"
            onClick={(e) => e.stopPropagation()}
          >
            <FileIcon className="h-3.5 w-3.5" />
            <span className="font-mono">
              {issue.file_path.split('/').pop()}:{lineRange}
            </span>
          </a>

          {/* Expand/collapse icon */}
          {expanded ? (
            <ChevronUpIcon className="h-4 w-4 text-gray-400" />
          ) : (
            <ChevronDownIcon className="h-4 w-4 text-gray-400" />
          )}
        </div>
      </div>

      {/* Expanded content */}
      {expanded && (
        <div className="px-3 py-2 border-t bg-white space-y-3">
          {/* Description */}
          <p className="text-sm text-gray-700">{issue.description}</p>

          {/* Code snippet */}
          {issue.code_snippet && (
            <pre className="p-2 bg-gray-800 text-gray-100 rounded text-xs font-mono overflow-x-auto">
              <code>{issue.code_snippet}</code>
            </pre>
          )}

          {/* Suggestion */}
          {issue.suggestion && (
            <div className="p-2 bg-green-50 border border-green-200 rounded">
              <p className="text-sm text-green-800">
                <strong className="font-medium">Suggested fix: </strong>
                {issue.suggestion}
              </p>
            </div>
          )}

          {/* CLAUDE.md reference */}
          {issue.claude_md_ref && (
            <p className="text-xs text-gray-500">
              <span className="font-medium">CLAUDE.md reference:</span> {issue.claude_md_ref}
            </p>
          )}

          {/* Full file path */}
          <p className="text-xs text-gray-500 font-mono">
            {issue.file_path}:{lineRange}
          </p>

          {/* Actions */}
          {issue.status === 'open' && onStatusChange && (
            <div className="flex gap-2 pt-1">
              <Button
                size="sm"
                variant="primary"
                onClick={handleMarkFixed}
                disabled={isLoading}
              >
                Mark as Fixed
              </Button>
              <Button
                size="sm"
                variant="secondary"
                onClick={handleMarkWontFix}
                disabled={isLoading}
              >
                Won't Fix
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// Compact variant for inline display.
export function CompactIssueCard({ issue, className }: { issue: ReviewIssue; className?: string }) {
  return <IssueCard issue={issue} compact className={className} />;
}
