// AnnotationToolbar displays a floating popover near text selections with
// buttons for creating different annotation types (comment, delete, replace,
// insert). Positioned using the selection bounding rect via createPortal.

import { useEffect, useRef, useCallback, useMemo } from 'react';
import { createPortal } from 'react-dom';
import { PlanAnnotationType } from '@/types/annotations.js';

export interface AnnotationToolbarProps {
  selectionRect: DOMRect;
  onAnnotate: (type: PlanAnnotationType) => void;
  onRequestComment: () => void;
  onClose: () => void;
}

export function AnnotationToolbar({
  selectionRect,
  onAnnotate,
  onRequestComment,
  onClose,
}: AnnotationToolbarProps) {
  const toolbarRef = useRef<HTMLDivElement>(null);

  // Calculate position above the selection.
  const position = useMemo(() => {
    const toolbarHeight = 40;
    const gap = 8;
    const top = selectionRect.top + window.scrollY - toolbarHeight - gap;
    const left = selectionRect.left + selectionRect.width / 2;
    return { top, left };
  }, [selectionRect]);

  // Close on escape key.
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  // Close on click outside.
  const handleMouseDown = useCallback(
    (e: MouseEvent) => {
      if (
        toolbarRef.current &&
        !toolbarRef.current.contains(e.target as Node)
      ) {
        onClose();
      }
    },
    [onClose],
  );

  useEffect(() => {
    document.addEventListener('mousedown', handleMouseDown);
    return () =>
      document.removeEventListener('mousedown', handleMouseDown);
  }, [handleMouseDown]);

  return createPortal(
    <div
      ref={toolbarRef}
      className="fixed z-[100] flex items-center gap-0.5 rounded-lg border border-gray-200 bg-white px-1 py-1 shadow-lg"
      style={{
        top: `${position.top}px`,
        left: `${position.left}px`,
        transform: 'translateX(-50%)',
      }}
    >
      {/* Comment button. */}
      <button
        type="button"
        onClick={onRequestComment}
        className="flex items-center gap-1 rounded-md px-2 py-1.5 text-xs font-medium text-gray-700 hover:bg-blue-50 hover:text-blue-700"
        title="Add comment"
      >
        <svg
          className="h-3.5 w-3.5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379.35.026.67.21.865.501L12 21l2.755-4.133a1.14 1.14 0 01.865-.501 48.172 48.172 0 003.423-.379c1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0012 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v6.018z"
          />
        </svg>
        Comment
      </button>

      <div className="mx-0.5 h-5 w-px bg-gray-200" />

      {/* Delete button. */}
      <button
        type="button"
        onClick={() => onAnnotate(PlanAnnotationType.DELETION)}
        className="flex items-center gap-1 rounded-md px-2 py-1.5 text-xs font-medium text-gray-700 hover:bg-red-50 hover:text-red-700"
        title="Mark for deletion"
      >
        <svg
          className="h-3.5 w-3.5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0"
          />
        </svg>
        Delete
      </button>

      {/* Replace button. */}
      <button
        type="button"
        onClick={() => onAnnotate(PlanAnnotationType.REPLACEMENT)}
        className="flex items-center gap-1 rounded-md px-2 py-1.5 text-xs font-medium text-gray-700 hover:bg-amber-50 hover:text-amber-700"
        title="Replace text"
      >
        <svg
          className="h-3.5 w-3.5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5"
          />
        </svg>
        Replace
      </button>

      {/* Insert button. */}
      <button
        type="button"
        onClick={() => onAnnotate(PlanAnnotationType.INSERTION)}
        className="flex items-center gap-1 rounded-md px-2 py-1.5 text-xs font-medium text-gray-700 hover:bg-green-50 hover:text-green-700"
        title="Insert text"
      >
        <svg
          className="h-3.5 w-3.5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M12 4.5v15m7.5-7.5h-15"
          />
        </svg>
        Insert
      </button>
    </div>,
    document.body,
  );
}
