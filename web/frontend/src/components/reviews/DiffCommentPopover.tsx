// DiffCommentPopover displays a popover for creating diff annotations.
// Supports comment, suggestion, and concern types with optional suggested
// code input and file-level comment toggle.

import { useState, useEffect, useRef, useCallback } from 'react';
import { createPortal } from 'react-dom';
import type { DiffAnnotationType, DiffAnnotationScope } from '@/types/annotations.js';

export interface DiffCommentPopoverProps {
  anchorRect: DOMRect;
  filePath: string;
  lineStart: number;
  lineEnd: number;
  side: 'old' | 'new';
  originalCode?: string | undefined;
  onSubmit: (params: {
    type: DiffAnnotationType;
    scope: DiffAnnotationScope;
    text: string;
    suggestedCode?: string | undefined;
  }) => void;
  onClose: () => void;
}

export function DiffCommentPopover({
  anchorRect,
  filePath,
  lineStart,
  lineEnd,
  side,
  originalCode: _originalCode,
  onSubmit,
  onClose,
}: DiffCommentPopoverProps) {
  const [type, setType] = useState<DiffAnnotationType>('comment');
  const [scope, setScope] = useState<DiffAnnotationScope>('line');
  const [text, setText] = useState('');
  const [suggestedCode, setSuggestedCode] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);

  // Auto-focus textarea on mount.
  useEffect(() => {
    setTimeout(() => textareaRef.current?.focus(), 50);
  }, []);

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
        popoverRef.current &&
        !popoverRef.current.contains(e.target as Node)
      ) {
        onClose();
      }
    },
    [onClose],
  );

  useEffect(() => {
    const timer = setTimeout(() => {
      document.addEventListener('mousedown', handleMouseDown);
    }, 100);
    return () => {
      clearTimeout(timer);
      document.removeEventListener('mousedown', handleMouseDown);
    };
  }, [handleMouseDown]);

  const handleSubmit = () => {
    const trimmed = text.trim();
    if (trimmed === '' && type !== 'suggestion') return;

    onSubmit({
      type,
      scope,
      text: trimmed,
      suggestedCode:
        type === 'suggestion' ? suggestedCode.trim() || undefined : undefined,
    });
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault();
      handleSubmit();
    }
  };

  // Position below the anchor rect.
  const top = anchorRect.bottom + window.scrollY + 8;
  const left = Math.min(anchorRect.left + 20, window.innerWidth - 400);

  const lineRange =
    lineStart === lineEnd
      ? `Line ${lineStart}`
      : `Lines ${lineStart}-${lineEnd}`;

  // Type button styles.
  const typeBtn = (t: DiffAnnotationType, label: string, color: string) => (
    <button
      type="button"
      onClick={() => setType(t)}
      className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
        type === t
          ? `${color} ring-1 ring-current`
          : 'text-gray-500 hover:bg-gray-100'
      }`}
    >
      {label}
    </button>
  );

  return createPortal(
    <div
      ref={popoverRef}
      className="fixed z-[110] w-96 rounded-lg border border-gray-200 bg-white shadow-xl"
      style={{
        top: `${top}px`,
        left: `${left}px`,
      }}
    >
      {/* Header with file and line info. */}
      <div className="flex items-center justify-between border-b border-gray-200 px-3 py-2">
        <div className="flex items-center gap-2 text-xs text-gray-500">
          <code className="rounded bg-gray-100 px-1.5 py-0.5 text-[10px]">
            {filePath.split('/').pop()}
          </code>
          <span>
            {scope === 'file' ? 'File comment' : `${lineRange} (${side})`}
          </span>
        </div>
      </div>

      <div className="p-3">
        {/* Type selector. */}
        <div className="mb-3 flex items-center gap-1">
          {typeBtn('comment', 'Comment', 'text-blue-700 bg-blue-50')}
          {typeBtn('suggestion', 'Suggestion', 'text-green-700 bg-green-50')}
          {typeBtn('concern', 'Concern', 'text-amber-700 bg-amber-50')}
          <div className="flex-1" />
          <label className="flex items-center gap-1.5 text-[10px] text-gray-400">
            <input
              type="checkbox"
              checked={scope === 'file'}
              onChange={(e) =>
                setScope(e.target.checked ? 'file' : 'line')
              }
              className="h-3 w-3 rounded border-gray-300"
            />
            File
          </label>
        </div>

        {/* Comment text. */}
        <textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={
            type === 'concern'
              ? 'Describe the concern...'
              : type === 'suggestion'
                ? 'Explain the suggestion...'
                : 'Add your comment...'
          }
          rows={3}
          className="w-full resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />

        {/* Suggested code input (only for suggestion type). */}
        {type === 'suggestion' && (
          <div className="mt-2">
            <label className="mb-1 block text-[10px] font-medium text-gray-500">
              Suggested replacement code
            </label>
            <textarea
              value={suggestedCode}
              onChange={(e) => setSuggestedCode(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Enter suggested code..."
              rows={3}
              className="w-full resize-none rounded-lg border border-gray-300 bg-green-50/30 px-3 py-2 font-mono text-xs placeholder-gray-400 focus:border-green-500 focus:outline-none focus:ring-1 focus:ring-green-500"
            />
          </div>
        )}

        {/* Action buttons. */}
        <div className="mt-2 flex items-center justify-between">
          <span className="text-[10px] text-gray-400">
            {navigator.platform.includes('Mac') ? '⌘' : 'Ctrl'}+Enter
          </span>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md px-3 py-1.5 text-xs font-medium text-gray-600 hover:bg-gray-100"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleSubmit}
              disabled={text.trim() === '' && type !== 'suggestion'}
              className="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              Add {type === 'concern' ? 'Concern' : type === 'suggestion' ? 'Suggestion' : 'Comment'}
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
