// AnnotationSidebar displays a right-side panel listing all annotations for
// the current plan review. Supports editing, deleting, and navigating to
// annotated blocks.

import { useState } from 'react';
import type { PlanAnnotation } from '@/types/annotations.js';
import { PlanAnnotationType } from '@/types/annotations.js';
import { useAnnotationStore } from '@/stores/annotations.js';

export interface AnnotationSidebarProps {
  annotations: PlanAnnotation[];
  onScrollToBlock?: (blockId: string) => void;
}

// typeLabel returns a human-readable label for the annotation type.
function typeLabel(type: PlanAnnotationType): string {
  switch (type) {
    case PlanAnnotationType.COMMENT:
      return 'Comment';
    case PlanAnnotationType.DELETION:
      return 'Delete';
    case PlanAnnotationType.REPLACEMENT:
      return 'Replace';
    case PlanAnnotationType.INSERTION:
      return 'Insert';
    case PlanAnnotationType.GLOBAL_COMMENT:
      return 'General';
    default:
      return 'Note';
  }
}

// typeBadgeClass returns CSS classes for the annotation type badge.
function typeBadgeClass(type: PlanAnnotationType): string {
  switch (type) {
    case PlanAnnotationType.COMMENT:
      return 'bg-yellow-100 text-yellow-800';
    case PlanAnnotationType.DELETION:
      return 'bg-red-100 text-red-800';
    case PlanAnnotationType.REPLACEMENT:
      return 'bg-blue-100 text-blue-800';
    case PlanAnnotationType.INSERTION:
      return 'bg-green-100 text-green-800';
    case PlanAnnotationType.GLOBAL_COMMENT:
      return 'bg-purple-100 text-purple-800';
    default:
      return 'bg-gray-100 text-gray-800';
  }
}

export function AnnotationSidebar({
  annotations,
  onScrollToBlock,
}: AnnotationSidebarProps) {
  const {
    selectedPlanAnnotationId,
    selectPlanAnnotation,
    deletePlanAnnotation,
    updatePlanAnnotation,
    addPlanAnnotation,
  } = useAnnotationStore();

  const [editingId, setEditingId] = useState<string | null>(null);
  const [editText, setEditText] = useState('');
  const [globalCommentText, setGlobalCommentText] = useState('');
  const [showGlobalInput, setShowGlobalInput] = useState(false);

  // Handle clicking an annotation card.
  const handleSelect = (ann: PlanAnnotation) => {
    selectPlanAnnotation(ann.id);
    if (ann.blockId && onScrollToBlock) {
      onScrollToBlock(ann.blockId);
    }
  };

  // Start editing an annotation.
  const handleEdit = (ann: PlanAnnotation) => {
    setEditingId(ann.id);
    setEditText(ann.text || '');
  };

  // Save edit.
  const handleSaveEdit = (ann: PlanAnnotation) => {
    updatePlanAnnotation(ann.id, { text: editText.trim() });
    setEditingId(null);
    setEditText('');
  };

  // Cancel edit.
  const handleCancelEdit = () => {
    setEditingId(null);
    setEditText('');
  };

  // Add global comment.
  const handleAddGlobalComment = () => {
    const trimmed = globalCommentText.trim();
    if (!trimmed) return;

    addPlanAnnotation({
      blockId: '',
      type: PlanAnnotationType.GLOBAL_COMMENT,
      text: trimmed,
      originalText: '',
      startOffset: 0,
      endOffset: 0,
    });

    setGlobalCommentText('');
    setShowGlobalInput(false);
  };

  return (
    <div className="flex h-full flex-col">
      {/* Header. */}
      <div className="flex items-center justify-between border-b border-gray-200 px-4 py-3">
        <h3 className="text-sm font-semibold text-gray-900">
          Annotations
          {annotations.length > 0 && (
            <span className="ml-1.5 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-normal text-gray-600">
              {annotations.length}
            </span>
          )}
        </h3>
        <button
          type="button"
          onClick={() => setShowGlobalInput(!showGlobalInput)}
          className="flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-gray-600 hover:bg-gray-100"
          title="Add global comment"
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
          Global
        </button>
      </div>

      {/* Global comment input. */}
      {showGlobalInput && (
        <div className="border-b border-gray-200 p-3">
          <textarea
            value={globalCommentText}
            onChange={(e) => setGlobalCommentText(e.target.value)}
            placeholder="Add a general comment..."
            rows={2}
            className="w-full resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
            onKeyDown={(e) => {
              if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                e.preventDefault();
                handleAddGlobalComment();
              }
            }}
          />
          <div className="mt-1.5 flex justify-end gap-2">
            <button
              type="button"
              onClick={() => {
                setShowGlobalInput(false);
                setGlobalCommentText('');
              }}
              className="rounded px-2 py-1 text-xs text-gray-500 hover:bg-gray-100"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleAddGlobalComment}
              disabled={!globalCommentText.trim()}
              className="rounded bg-blue-600 px-2 py-1 text-xs text-white hover:bg-blue-700 disabled:opacity-50"
            >
              Add
            </button>
          </div>
        </div>
      )}

      {/* Annotation list. */}
      <div className="flex-1 overflow-y-auto">
        {annotations.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-center">
            <svg
              className="mb-3 h-10 w-10 text-gray-300"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={1.5}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379.35.026.67.21.865.501L12 21l2.755-4.133a1.14 1.14 0 01.865-.501 48.172 48.172 0 003.423-.379c1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0012 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v6.018z"
              />
            </svg>
            <p className="text-sm text-gray-500">No annotations yet</p>
            <p className="mt-1 text-xs text-gray-400">
              Select text in the plan to annotate
            </p>
          </div>
        ) : (
          <div className="divide-y divide-gray-100">
            {annotations.map((ann, index) => (
              <div
                key={ann.id}
                className={`cursor-pointer px-4 py-3 transition-colors hover:bg-gray-50 ${
                  selectedPlanAnnotationId === ann.id
                    ? 'bg-blue-50 border-l-2 border-blue-500'
                    : ''
                }`}
                onClick={() => handleSelect(ann)}
              >
                {/* Type badge and index. */}
                <div className="flex items-center justify-between mb-1.5">
                  <div className="flex items-center gap-2">
                    <span className="text-[10px] font-medium text-gray-400">
                      #{index + 1}
                    </span>
                    <span
                      className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${typeBadgeClass(ann.type as PlanAnnotationType)}`}
                    >
                      {typeLabel(ann.type as PlanAnnotationType)}
                    </span>
                  </div>
                  <div className="flex gap-1">
                    {(ann.type === PlanAnnotationType.COMMENT ||
                      ann.type === PlanAnnotationType.REPLACEMENT ||
                      ann.type === PlanAnnotationType.INSERTION ||
                      ann.type === PlanAnnotationType.GLOBAL_COMMENT) && (
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleEdit(ann);
                        }}
                        className="rounded p-1 text-gray-400 hover:bg-gray-200 hover:text-gray-600"
                        title="Edit"
                      >
                        <svg
                          className="h-3 w-3"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                          strokeWidth={2}
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0115.75 21H5.25A2.25 2.25 0 013 18.75V8.25A2.25 2.25 0 015.25 6H10"
                          />
                        </svg>
                      </button>
                    )}
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        deletePlanAnnotation(ann.id);
                      }}
                      className="rounded p-1 text-gray-400 hover:bg-red-100 hover:text-red-600"
                      title="Delete"
                    >
                      <svg
                        className="h-3 w-3"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                        strokeWidth={2}
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          d="M6 18L18 6M6 6l12 12"
                        />
                      </svg>
                    </button>
                  </div>
                </div>

                {/* Original text snippet. */}
                {ann.originalText && (
                  <p className="text-xs text-gray-500 truncate mb-1">
                    &ldquo;{ann.originalText}&rdquo;
                  </p>
                )}

                {/* Annotation text (editable). */}
                {editingId === ann.id ? (
                  <div className="mt-1">
                    <textarea
                      value={editText}
                      onChange={(e) => setEditText(e.target.value)}
                      rows={2}
                      className="w-full resize-none rounded border border-gray-300 px-2 py-1 text-xs focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                      autoFocus
                      onKeyDown={(e) => {
                        if (
                          (e.metaKey || e.ctrlKey) &&
                          e.key === 'Enter'
                        ) {
                          e.preventDefault();
                          handleSaveEdit(ann);
                        }
                        if (e.key === 'Escape') {
                          handleCancelEdit();
                        }
                      }}
                    />
                    <div className="mt-1 flex justify-end gap-1">
                      <button
                        type="button"
                        onClick={handleCancelEdit}
                        className="rounded px-2 py-0.5 text-[10px] text-gray-500 hover:bg-gray-100"
                      >
                        Cancel
                      </button>
                      <button
                        type="button"
                        onClick={() => handleSaveEdit(ann)}
                        className="rounded bg-blue-600 px-2 py-0.5 text-[10px] text-white hover:bg-blue-700"
                      >
                        Save
                      </button>
                    </div>
                  </div>
                ) : ann.text ? (
                  <p className="text-xs text-gray-700 whitespace-pre-wrap">
                    {ann.text}
                  </p>
                ) : null}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
