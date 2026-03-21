// DiffAnnotationCard renders an inline card below annotated diff lines
// showing the annotation type, comment text, and suggested code (if any).

import { useState } from 'react';
import type { DiffAnnotation, DiffAnnotationType } from '@/types/annotations.js';

export interface DiffAnnotationCardProps {
  annotation: DiffAnnotation;
  onUpdate: (id: string, text: string, suggestedCode?: string | undefined) => void;
  onDelete: (id: string) => void;
}

// typeBadge returns styling for the annotation type badge.
function typeBadge(type: DiffAnnotationType): {
  label: string;
  className: string;
} {
  switch (type) {
    case 'comment':
      return {
        label: 'Comment',
        className: 'bg-blue-100 text-blue-800',
      };
    case 'suggestion':
      return {
        label: 'Suggestion',
        className: 'bg-green-100 text-green-800',
      };
    case 'concern':
      return {
        label: 'Concern',
        className: 'bg-amber-100 text-amber-800',
      };
    default:
      return { label: 'Note', className: 'bg-gray-100 text-gray-800' };
  }
}

export function DiffAnnotationCard({
  annotation,
  onUpdate,
  onDelete,
}: DiffAnnotationCardProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [editText, setEditText] = useState(annotation.text || '');
  const [editCode, setEditCode] = useState(
    annotation.suggestedCode || '',
  );

  const badge = typeBadge(annotation.type);

  const handleSave = () => {
    onUpdate(
      annotation.id,
      editText.trim(),
      annotation.type === 'suggestion' ? editCode.trim() || undefined : undefined,
    );
    setIsEditing(false);
  };

  const handleCancel = () => {
    setEditText(annotation.text || '');
    setEditCode(annotation.suggestedCode || '');
    setIsEditing(false);
  };

  return (
    <div className="mx-2 my-1 rounded-lg border border-gray-200 bg-white shadow-sm">
      {/* Header with type badge and actions. */}
      <div className="flex items-center justify-between px-3 py-1.5">
        <div className="flex items-center gap-2">
          <span
            className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${badge.className}`}
          >
            {badge.label}
          </span>
          {annotation.scope === 'file' ? (
            <span className="text-[10px] text-gray-400">
              File comment
            </span>
          ) : (
            <span className="text-[10px] text-gray-400">
              L{annotation.lineStart}
              {annotation.lineEnd !== annotation.lineStart &&
                `-${annotation.lineEnd}`}{' '}
              ({annotation.side})
            </span>
          )}
        </div>
        <div className="flex gap-1">
          <button
            type="button"
            onClick={() => setIsEditing(!isEditing)}
            className="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
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
          <button
            type="button"
            onClick={() => onDelete(annotation.id)}
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

      {/* Content. */}
      <div className="px-3 pb-2">
        {isEditing ? (
          <div>
            <textarea
              value={editText}
              onChange={(e) => setEditText(e.target.value)}
              rows={2}
              className="w-full resize-none rounded border border-gray-300 px-2 py-1 text-xs focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              autoFocus
              onKeyDown={(e) => {
                if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                  e.preventDefault();
                  handleSave();
                }
                if (e.key === 'Escape') handleCancel();
              }}
            />
            {annotation.type === 'suggestion' && (
              <textarea
                value={editCode}
                onChange={(e) => setEditCode(e.target.value)}
                rows={2}
                placeholder="Suggested code..."
                className="mt-1 w-full resize-none rounded border border-gray-300 bg-green-50/30 px-2 py-1 font-mono text-[11px] focus:border-green-500 focus:outline-none focus:ring-1 focus:ring-green-500"
              />
            )}
            <div className="mt-1 flex justify-end gap-1">
              <button
                type="button"
                onClick={handleCancel}
                className="rounded px-2 py-0.5 text-[10px] text-gray-500 hover:bg-gray-100"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleSave}
                className="rounded bg-blue-600 px-2 py-0.5 text-[10px] text-white hover:bg-blue-700"
              >
                Save
              </button>
            </div>
          </div>
        ) : (
          <>
            {annotation.text && (
              <p className="text-xs text-gray-700 whitespace-pre-wrap">
                {annotation.text}
              </p>
            )}
            {annotation.suggestedCode && (
              <pre className="mt-1.5 rounded bg-green-50 border border-green-200 px-2 py-1.5 text-[11px] font-mono text-green-900 overflow-x-auto">
                {annotation.suggestedCode}
              </pre>
            )}
          </>
        )}
      </div>
    </div>
  );
}
