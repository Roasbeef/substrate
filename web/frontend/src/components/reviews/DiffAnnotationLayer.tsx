// DiffAnnotationLayer provides an annotation overlay for code diffs rendered
// by @pierre/diffs. Adds clickable gutter markers, inline annotation cards,
// and a comment popover for creating new annotations.

import { useState, useCallback } from 'react';
import type {
  DiffAnnotation,
  DiffAnnotationType,
  DiffAnnotationScope,
} from '@/types/annotations.js';
import { DiffCommentPopover } from './DiffCommentPopover.js';
import { DiffAnnotationCard } from './DiffAnnotationCard.js';

export interface DiffAnnotationLayerProps {
  filePath: string;
  annotations: DiffAnnotation[];
  onAddAnnotation: (params: {
    filePath: string;
    type: DiffAnnotationType;
    scope: DiffAnnotationScope;
    lineStart: number;
    lineEnd: number;
    side: 'old' | 'new';
    text: string;
    suggestedCode?: string | undefined;
  }) => void;
  onUpdateAnnotation: (
    id: string,
    text: string,
    suggestedCode?: string | undefined,
  ) => void;
  onDeleteAnnotation: (id: string) => void;
  children: React.ReactNode;
}

export function DiffAnnotationLayer({
  filePath,
  annotations,
  onAddAnnotation,
  onUpdateAnnotation,
  onDeleteAnnotation,
  children,
}: DiffAnnotationLayerProps) {
  const [popoverState, setPopoverState] = useState<{
    rect: DOMRect;
    lineStart: number;
    lineEnd: number;
    side: 'old' | 'new';
    originalCode?: string | undefined;
  } | null>(null);

  // Handle gutter click — opens the comment popover.
  const handleGutterClick = useCallback(
    (e: React.MouseEvent) => {
      const target = e.target as HTMLElement;

      // Look for line number elements in the @pierre/diffs DOM.
      // The library renders line numbers in elements with data attributes
      // or specific class names. We look for elements containing line numbers.
      const lineEl =
        target.closest('[data-line-number]') ??
        target.closest('.line-number') ??
        target.closest('td.line-num');

      if (!lineEl) return;

      const lineNum = parseInt(
        lineEl.getAttribute('data-line-number') ??
          lineEl.textContent?.trim() ?? '0',
        10,
      );
      if (isNaN(lineNum) || lineNum <= 0) return;

      // Determine side (old vs new) from the element context.
      const isOldSide =
        lineEl.classList.contains('old') ||
        lineEl.closest('.old-side') !== null ||
        lineEl.closest('[data-side="old"]') !== null;

      const rect = lineEl.getBoundingClientRect();
      setPopoverState({
        rect,
        lineStart: lineNum,
        lineEnd: lineNum,
        side: isOldSide ? 'old' : 'new',
      });
    },
    [],
  );

  // Handle popover submission.
  const handleSubmit = useCallback(
    (params: {
      type: DiffAnnotationType;
      scope: DiffAnnotationScope;
      text: string;
      suggestedCode?: string | undefined;
    }) => {
      if (!popoverState) return;

      onAddAnnotation({
        filePath,
        type: params.type,
        scope: params.scope,
        lineStart: popoverState.lineStart,
        lineEnd: popoverState.lineEnd,
        side: popoverState.side,
        text: params.text,
        suggestedCode: params.suggestedCode,
      });

      setPopoverState(null);
    },
    [popoverState, filePath, onAddAnnotation],
  );

  // Get annotations for this file, sorted by line.
  const fileAnnotations = annotations
    .filter((a) => a.filePath === filePath)
    .sort((a, b) => a.lineStart - b.lineStart);

  return (
    <div className="relative">
      {/* Diff content with click handler on gutter. */}
      <div onClick={handleGutterClick} className="cursor-pointer">
        {children}
      </div>

      {/* Inline annotation cards below the diff. */}
      {fileAnnotations.length > 0 && (
        <div className="border-t border-gray-200 bg-gray-50/50 py-1">
          {fileAnnotations.map((ann) => (
            <DiffAnnotationCard
              key={ann.id}
              annotation={ann}
              onUpdate={onUpdateAnnotation}
              onDelete={onDeleteAnnotation}
            />
          ))}
        </div>
      )}

      {/* Gutter annotation markers. */}
      {fileAnnotations.length > 0 && (
        <div className="absolute left-0 top-0 pointer-events-none">
          {/* Markers are rendered as absolute dots — simplified for now
              since exact positioning depends on @pierre/diffs layout. */}
        </div>
      )}

      {/* Comment popover. */}
      {popoverState && (
        <DiffCommentPopover
          anchorRect={popoverState.rect}
          filePath={filePath}
          lineStart={popoverState.lineStart}
          lineEnd={popoverState.lineEnd}
          side={popoverState.side}
          originalCode={popoverState.originalCode}
          onSubmit={handleSubmit}
          onClose={() => setPopoverState(null)}
        />
      )}
    </div>
  );
}
