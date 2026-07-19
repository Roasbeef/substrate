// CommentPopover displays a positioned popover for entering annotation text.
// Supports comment, replacement, and insertion modes with context snippet
// display and keyboard shortcuts (Cmd+Enter to submit).

import { useState, useEffect, useRef, useCallback } from 'react';
import { createPortal } from 'react-dom';
export type CommentMode = 'comment' | 'replace' | 'insert';

export interface CommentPopoverProps {
  anchorRect: DOMRect;
  mode: CommentMode;
  contextText: string;
  onSubmit: (text: string) => void;
  onClose: () => void;
}

// modeLabel returns a human-readable label for the annotation mode.
function modeLabel(mode: CommentMode): string {
  switch (mode) {
    case 'comment':
      return 'Add Comment';
    case 'replace':
      return 'Replace With';
    case 'insert':
      return 'Insert Text';
  }
}

// modePlaceholder returns placeholder text for the textarea.
function modePlaceholder(mode: CommentMode): string {
  switch (mode) {
    case 'comment':
      return 'Enter your comment...';
    case 'replace':
      return 'Enter replacement text...';
    case 'insert':
      return 'Enter text to insert...';
  }
}

// modeColor returns the accent color class for the mode.
function modeColorClass(mode: CommentMode): string {
  switch (mode) {
    case 'comment':
      return 'text-[#33608D] bg-[#33608D]/8 border-[#33608D]/20';
    case 'replace':
      return 'text-[#92610E] bg-[#C98A1B]/8 border-[#C98A1B]/25';
    case 'insert':
      return 'text-[#178A5B] bg-[#178A5B]/8 border-[#178A5B]/20';
  }
}

export function CommentPopover({
  anchorRect,
  mode,
  contextText,
  onSubmit,
  onClose,
}: CommentPopoverProps) {
  const [text, setText] = useState('');
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
    // Delay to avoid immediate close from the toolbar click.
    const timer = setTimeout(() => {
      document.addEventListener('mousedown', handleMouseDown);
    }, 100);
    return () => {
      clearTimeout(timer);
      document.removeEventListener('mousedown', handleMouseDown);
    };
  }, [handleMouseDown]);

  // Handle keyboard shortcuts.
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault();
      handleSubmit();
    }
  };

  const handleSubmit = () => {
    const trimmed = text.trim();
    if (trimmed === '') return;
    onSubmit(trimmed);
  };

  // Position below the anchor rect.
  const top = anchorRect.bottom + window.scrollY + 8;
  const left = anchorRect.left + anchorRect.width / 2;

  // Truncate context text for display.
  const displayContext =
    contextText.length > 80
      ? contextText.slice(0, 77) + '...'
      : contextText;

  return createPortal(
    <div
      ref={popoverRef}
      className="fixed z-[110] w-80 rounded-lg border border-[#E6E4DD] bg-white shadow-xl"
      style={{
        top: `${top}px`,
        left: `${left}px`,
        transform: 'translateX(-50%)',
      }}
    >
      {/* Mode header. */}
      <div
        className={`flex items-center gap-2 rounded-t-lg border-b px-3 py-2 text-xs font-semibold ${modeColorClass(mode)}`}
      >
        {modeLabel(mode)}
      </div>

      <div className="p-3">
        {/* Context snippet. */}
        {contextText && mode === 'comment' && (
          <div className="mb-2 rounded border border-[#F1EFE9] bg-[#F4F3EE] px-2 py-1.5 text-xs text-[#6B7280]">
            Re: &ldquo;{displayContext}&rdquo;
          </div>
        )}

        {/* Text input. */}
        <textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={modePlaceholder(mode)}
          rows={3}
          className="w-full resize-none rounded-lg border border-[#E6E4DD] px-3 py-2 text-sm placeholder-[#9BA0A6] focus:border-[#22262A] focus:outline-none focus:ring-1 focus:ring-[#22262A]/30"
        />

        {/* Action buttons. */}
        <div className="mt-2 flex items-center justify-between">
          <span className="text-[10px] text-[#9BA0A6]">
            {navigator.platform.includes('Mac') ? '⌘' : 'Ctrl'}+Enter
            to submit
          </span>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md px-3 py-1.5 text-xs font-medium text-[#6B7280] hover:bg-[#F1EFE9]"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleSubmit}
              disabled={text.trim() === ''}
              className="rounded-md bg-[#22262A] px-3 py-1.5 text-xs font-medium text-white hover:bg-[#3A4046] disabled:cursor-not-allowed disabled:opacity-50"
            >
              Submit
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
