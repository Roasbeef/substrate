// Annotation type definitions for plan review and diff code review.
// Ported from plannotator's type system with adaptations for substrate.

// =============================================================================
// Plan Annotation Types
// =============================================================================

// PlanAnnotationType enumerates the kinds of inline annotations on plan
// markdown blocks.
export const PlanAnnotationType = {
  DELETION: 'DELETION',
  INSERTION: 'INSERTION',
  REPLACEMENT: 'REPLACEMENT',
  COMMENT: 'COMMENT',
  GLOBAL_COMMENT: 'GLOBAL_COMMENT',
} as const;

export type PlanAnnotationType =
  (typeof PlanAnnotationType)[keyof typeof PlanAnnotationType];

// PlanAnnotation represents a single inline annotation on a plan block.
export interface PlanAnnotation {
  id: string;
  blockId: string;
  type: PlanAnnotationType;
  text?: string | undefined;
  originalText: string;
  startOffset: number;
  endOffset: number;
  createdAt: number;
  updatedAt: number;
  diffContext?: 'added' | 'removed' | 'modified' | undefined;
}

// =============================================================================
// Markdown Block Types
// =============================================================================

// BlockType enumerates the structural types a parsed markdown block can be.
export type BlockType =
  | 'paragraph'
  | 'heading'
  | 'blockquote'
  | 'list-item'
  | 'code'
  | 'hr'
  | 'table';

// Block represents a single structural element parsed from markdown content.
export interface Block {
  id: string;
  type: BlockType;
  content: string;
  level?: number | undefined;
  language?: string | undefined;
  checked?: boolean | undefined;
  order: number;
  startLine: number;
}

// =============================================================================
// Diff Annotation Types
// =============================================================================

// DiffAnnotationType enumerates the kinds of annotations on code diffs.
export type DiffAnnotationType = 'comment' | 'suggestion' | 'concern';

// DiffAnnotationScope distinguishes line-level from file-level annotations.
export type DiffAnnotationScope = 'line' | 'file';

// DiffAnnotation represents a single annotation on a code diff.
export interface DiffAnnotation {
  id: string;
  type: DiffAnnotationType;
  scope: DiffAnnotationScope;
  filePath: string;
  lineStart: number;
  lineEnd: number;
  side: 'old' | 'new';
  text?: string | undefined;
  suggestedCode?: string | undefined;
  originalCode?: string | undefined;
  createdAt: number;
  updatedAt: number;
}

// =============================================================================
// Plan Diff Types
// =============================================================================

// PlanDiffBlockType enumerates the change types in a plan version diff.
export type PlanDiffBlockType = 'added' | 'removed' | 'modified' | 'unchanged';

// PlanDiffBlock represents a single block in a plan version diff.
export interface PlanDiffBlock {
  type: PlanDiffBlockType;
  content: string;
  oldContent?: string;
  lines: number;
}

// PlanDiffStats summarizes the changes between two plan versions.
export interface PlanDiffStats {
  additions: number;
  deletions: number;
  modifications: number;
}

// PlanDiffResult holds the full diff output between two plan versions.
export interface PlanDiffResult {
  blocks: PlanDiffBlock[];
  stats: PlanDiffStats;
}

// =============================================================================
// Editor Mode Types
// =============================================================================

// EditorMode controls which annotation tool is active in the plan viewer.
export type EditorMode = 'selection' | 'comment' | 'redline';

// SelectedLineRange tracks the user's current selection in a diff view.
export interface SelectedLineRange {
  start: number;
  end: number;
  side: 'old' | 'new';
}
