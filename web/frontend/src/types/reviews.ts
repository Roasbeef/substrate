// Review-related type definitions for the Subtrate native review mode.

// Review states follow the FSM lifecycle.
export type ReviewState =
  | 'new'
  | 'pending_review'
  | 'under_review'
  | 'changes_requested'
  | 're_review'
  | 'approved'
  | 'rejected'
  | 'cancelled';

// Reviewer decision types.
export type ReviewDecision = 'approve' | 'request_changes' | 'comment';

// Issue severity levels.
export type IssueSeverity = 'critical' | 'high' | 'medium' | 'low';

// Issue types.
export type IssueType = 'bug' | 'security' | 'claude_md_violation' | 'logic_error';

// Issue resolution status.
export type IssueStatus = 'open' | 'fixed' | 'wont_fix' | 'duplicate';

// Review type (full, incremental, etc.)
export type ReviewType = 'full' | 'incremental' | 'security' | 'performance';

// Review priority.
export type ReviewPriority = 'urgent' | 'normal' | 'low';

// Core review entity.
export interface Review {
  id: number;
  review_id: string;
  thread_id: string;
  requester_id: number;
  requester_name?: string;

  // PR information
  pr_number?: number;
  branch: string;
  base_branch: string;
  commit_sha: string;
  repo_path: string;

  // Configuration
  review_type: ReviewType;
  priority: ReviewPriority;
  state: ReviewState;

  // Timestamps
  created_at: string;
  updated_at: string;
  completed_at?: string;
}

// Review with detailed information including iterations.
export interface ReviewWithDetails extends Review {
  iterations: ReviewIteration[];
  open_issues_count: number;
  total_issues_count: number;
}

// Review iteration (each round of review).
export interface ReviewIteration {
  id: number;
  review_id: string;
  iteration_num: number;
  reviewer_id: string;
  reviewer_session_id?: string;

  // Results
  decision: ReviewDecision;
  summary: string;
  issues: ReviewIssue[];
  suggestions: Suggestion[];

  // Metrics
  files_reviewed: number;
  lines_analyzed: number;
  duration_ms: number;
  cost_usd: number;

  // Timestamps
  started_at: string;
  completed_at?: string;
}

// Review issue (problem found during review).
export interface ReviewIssue {
  id: number;
  review_id: string;
  iteration_num: number;

  // Issue details
  type: IssueType;
  severity: IssueSeverity;

  // Location
  file_path: string;
  line_start: number;
  line_end?: number;

  // Content
  title: string;
  description: string;
  code_snippet?: string;
  suggestion?: string;
  claude_md_ref?: string;

  // Resolution tracking
  status: IssueStatus;
  resolved_at?: string;
  resolved_in_iteration?: number;

  created_at: string;
}

// Non-blocking suggestion.
export interface Suggestion {
  title: string;
  description: string;
  file_path?: string;
}

// Reviewer status for multi-reviewer tracking.
export interface ReviewerStatus {
  reviewer_id: string;
  reviewer_name: string;
  status: 'pending' | 'reviewing' | 'completed';
  decision?: ReviewDecision;
  issue_count: number;
}

// Review statistics.
export interface ReviewStats {
  total_reviews: number;
  approved: number;
  pending: number;
  in_progress: number;
  changes_requested: number;
  avg_iterations: number;
  avg_duration_ms: number;
}

// File diff information.
export interface FileDiff {
  file_path: string;
  old_content: string;
  new_content: string;
  additions: number;
  deletions: number;
  issues: ReviewIssue[];
}

// Multi-file patch information.
export interface ReviewPatch {
  review_id: string;
  content: string;
  files: FileDiffSummary[];
}

// Summary of a file in a patch.
export interface FileDiffSummary {
  name: string;
  additions: number;
  deletions: number;
  status: 'added' | 'modified' | 'deleted' | 'renamed';
}

// API Request types

export interface ListReviewsParams {
  filter?: ReviewState | 'all';
  requester_id?: number;
  limit?: number;
  offset?: number;
}

export interface CreateReviewRequest {
  branch: string;
  base_branch?: string;
  commit_sha: string;
  repo_path: string;
  pr_number?: number;
  review_type?: ReviewType;
  priority?: ReviewPriority;
  reviewers?: string[];
  description?: string;
}

export interface ResubmitReviewRequest {
  commit_sha: string;
}

export interface UpdateIssueStatusRequest {
  status: IssueStatus;
}

// API Response types

export interface ReviewListResponse {
  reviews: Review[];
  total: number;
}

export interface ReviewDetailResponse extends ReviewWithDetails {
  thread_messages?: ThreadMessage[];
}

export interface ThreadMessage {
  id: number;
  sender_id: number;
  sender_name: string;
  body: string;
  created_at: string;
  is_review_message: boolean;
}

// WebSocket event types for real-time updates.

export type ReviewWebSocketEventType =
  | 'review_created'
  | 'review_updated'
  | 'review_iteration_added'
  | 'issue_resolved'
  | 'review_approved'
  | 'review_rejected';

export interface ReviewWebSocketEvent {
  type: ReviewWebSocketEventType;
  payload: {
    review_id: string;
    iteration_num?: number;
    issue_id?: number;
    state?: ReviewState;
    decision?: ReviewDecision;
  };
}

// Helper types for UI components

export interface ReviewFilterOptions {
  state: ReviewState | 'all';
  reviewType?: ReviewType;
  priority?: ReviewPriority;
  mine: boolean;
}

export interface IssueFilterOptions {
  status: IssueStatus | 'all';
  severity?: IssueSeverity;
  type?: IssueType;
  file?: string;
}

// Color and style mappings for UI

export const stateColors: Record<ReviewState, string> = {
  new: 'gray',
  pending_review: 'yellow',
  under_review: 'purple',
  changes_requested: 'orange',
  re_review: 'yellow',
  approved: 'green',
  rejected: 'red',
  cancelled: 'gray',
};

export const stateLabels: Record<ReviewState, string> = {
  new: 'New',
  pending_review: 'Pending',
  under_review: 'Reviewing',
  changes_requested: 'Changes Requested',
  re_review: 'Re-review',
  approved: 'Approved',
  rejected: 'Rejected',
  cancelled: 'Cancelled',
};

export const severityColors: Record<IssueSeverity, string> = {
  critical: 'red',
  high: 'orange',
  medium: 'yellow',
  low: 'blue',
};

export const decisionColors: Record<ReviewDecision, string> = {
  approve: 'green',
  request_changes: 'orange',
  comment: 'blue',
};

export const decisionLabels: Record<ReviewDecision, string> = {
  approve: 'Approved',
  request_changes: 'Changes Requested',
  comment: 'Commented',
};

// Utility functions

export function isTerminalState(state: ReviewState): boolean {
  return state === 'approved' || state === 'rejected' || state === 'cancelled';
}

export function canResubmit(state: ReviewState): boolean {
  return state === 'changes_requested' || state === 're_review';
}

export function canCancel(state: ReviewState): boolean {
  return !isTerminalState(state);
}
