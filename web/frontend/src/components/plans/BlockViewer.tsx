// BlockViewer renders parsed markdown blocks as selectable DOM elements with
// inline annotation highlighting. Handles text selection for annotation
// creation via the AnnotationToolbar popover.

import { useRef, useState, useCallback, useEffect, useMemo, memo } from 'react';
import type { Block, PlanAnnotation } from '@/types/annotations.js';
import { PlanAnnotationType } from '@/types/annotations.js';
import { AnnotationToolbar } from './AnnotationToolbar.js';
import { CommentPopover, type CommentMode } from './CommentPopover.js';
import { useAnnotationStore } from '@/stores/annotations.js';

export interface BlockViewerProps {
  blocks: Block[];
  annotations: PlanAnnotation[];
  readOnly?: boolean | undefined;
}

export function BlockViewer({
  blocks,
  annotations,
  readOnly = false,
}: BlockViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [toolbarState, setToolbarState] = useState<{
    rect: DOMRect;
    blockId: string;
    selectedText: string;
  } | null>(null);
  const [commentState, setCommentState] = useState<{
    rect: DOMRect;
    mode: CommentMode;
    blockId: string;
    selectedText: string;
  } | null>(null);

  const {
    addPlanAnnotation,
    selectPlanAnnotation,
    selectedPlanAnnotationId,
  } = useAnnotationStore();

  // Handle text selection within blocks.
  const handleMouseUp = useCallback(() => {
    if (readOnly) return;

    const selection = window.getSelection();
    if (!selection || selection.isCollapsed || !selection.rangeCount) {
      return;
    }

    const range = selection.getRangeAt(0);
    const selectedText = selection.toString().trim();
    if (!selectedText) return;

    // Find the block element containing the selection. The
    // startContainer is usually a text node, so walk up to the nearest
    // element before calling closest().
    const startNode = range.startContainer;
    const startEl = startNode.nodeType === Node.TEXT_NODE
      ? startNode.parentElement
      : startNode as HTMLElement;
    const blockEl = startEl?.closest?.('[data-block-id]');
    if (!blockEl) return;

    const blockId = blockEl.getAttribute('data-block-id');
    if (!blockId) return;

    const rect = range.getBoundingClientRect();
    setToolbarState({ rect, blockId, selectedText });
  }, [readOnly]);

  // Clear toolbar when selection changes. Uses a mounted guard so the
  // delayed setState cannot fire after the component unmounts.
  useEffect(() => {
    let mounted = true;
    let timerId: ReturnType<typeof setTimeout> | null = null;

    const handleSelectionChange = () => {
      const selection = window.getSelection();
      if (!selection || selection.isCollapsed) {
        // Delay to allow toolbar clicks to register.
        timerId = setTimeout(() => {
          if (!mounted) return;
          const sel = window.getSelection();
          if (!sel || sel.isCollapsed) {
            setToolbarState(null);
          }
        }, 200);
      }
    };
    document.addEventListener('selectionchange', handleSelectionChange);
    return () => {
      mounted = false;
      if (timerId) clearTimeout(timerId);
      document.removeEventListener(
        'selectionchange',
        handleSelectionChange,
      );
    };
  }, []);

  // Handle annotation creation from toolbar.
  const handleAnnotate = useCallback(
    (type: PlanAnnotationType) => {
      if (!toolbarState) return;

      if (
        type === PlanAnnotationType.COMMENT ||
        type === PlanAnnotationType.REPLACEMENT ||
        type === PlanAnnotationType.INSERTION
      ) {
        // These need text input — open the comment popover.
        const mode: CommentMode =
          type === PlanAnnotationType.COMMENT
            ? 'comment'
            : type === PlanAnnotationType.REPLACEMENT
              ? 'replace'
              : 'insert';

        setCommentState({
          rect: toolbarState.rect,
          mode,
          blockId: toolbarState.blockId,
          selectedText: toolbarState.selectedText,
        });
        setToolbarState(null);
        return;
      }

      // Deletion — no text input needed.
      addPlanAnnotation({
        blockId: toolbarState.blockId,
        type,
        originalText: toolbarState.selectedText,
        startOffset: 0,
        endOffset: toolbarState.selectedText.length,
      });

      setToolbarState(null);
      window.getSelection()?.removeAllRanges();
    },
    [toolbarState, addPlanAnnotation],
  );

  // Handle comment popover requesting to open.
  const handleRequestComment = useCallback(() => {
    if (!toolbarState) return;
    setCommentState({
      rect: toolbarState.rect,
      mode: 'comment',
      blockId: toolbarState.blockId,
      selectedText: toolbarState.selectedText,
    });
    setToolbarState(null);
  }, [toolbarState]);

  // Handle comment submission.
  const handleCommentSubmit = useCallback(
    (text: string) => {
      if (!commentState) return;

      const type =
        commentState.mode === 'comment'
          ? PlanAnnotationType.COMMENT
          : commentState.mode === 'replace'
            ? PlanAnnotationType.REPLACEMENT
            : PlanAnnotationType.INSERTION;

      addPlanAnnotation({
        blockId: commentState.blockId,
        type,
        text,
        originalText: commentState.selectedText,
        startOffset: 0,
        endOffset: commentState.selectedText.length,
      });

      setCommentState(null);
      window.getSelection()?.removeAllRanges();
    },
    [commentState, addPlanAnnotation],
  );

  // Build a map of block ID → annotations for O(1) lookup per block.
  const blockAnnotationMap = useMemo(() => {
    const map = new Map<string, PlanAnnotation[]>();
    for (const ann of annotations) {
      const existing = map.get(ann.blockId);
      if (existing) {
        existing.push(ann);
      } else {
        map.set(ann.blockId, [ann]);
      }
    }
    return map;
  }, [annotations]);

  return (
    <div
      ref={containerRef}
      className="relative"
      onMouseUp={handleMouseUp}
    >
      {blocks.map((block) => (
        <BlockRenderer
          key={block.id}
          block={block}
          annotations={blockAnnotationMap.get(block.id) ?? []}
          selectedAnnotationId={selectedPlanAnnotationId}
          onSelectAnnotation={selectPlanAnnotation}
        />
      ))}

      {/* Text selection toolbar. */}
      {toolbarState && (
        <AnnotationToolbar
          selectionRect={toolbarState.rect}
          onAnnotate={handleAnnotate}
          onRequestComment={handleRequestComment}
          onClose={() => setToolbarState(null)}
        />
      )}

      {/* Comment popover. */}
      {commentState && (
        <CommentPopover
          anchorRect={commentState.rect}
          mode={commentState.mode}
          contextText={commentState.selectedText}
          onSubmit={handleCommentSubmit}
          onClose={() => setCommentState(null)}
        />
      )}
    </div>
  );
}

// =============================================================================
// Block Renderer
// =============================================================================

interface BlockRendererProps {
  block: Block;
  annotations: PlanAnnotation[];
  selectedAnnotationId: string | null;
  onSelectAnnotation: (id: string | null) => void;
}

const BlockRenderer = memo(function BlockRenderer({
  block,
  annotations,
  selectedAnnotationId,
  onSelectAnnotation,
}: BlockRendererProps) {
  const hasAnnotations = annotations.length > 0;
  const isSelected = annotations.some(
    (a) => a.id === selectedAnnotationId,
  );

  // Base classes for annotated blocks.
  const annotatedClass = hasAnnotations
    ? 'relative ring-1 ring-yellow-200 rounded-sm'
    : '';
  const selectedClass = isSelected ? 'ring-2 ring-blue-400' : '';

  const handleClick = () => {
    if (hasAnnotations) {
      onSelectAnnotation(annotations[0]!.id);
    }
  };

  switch (block.type) {
    case 'heading': {
      const Tag = (
        `h${block.level || 1}` as 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6'
      );
      const styles: Record<number, string> = {
        1: 'text-2xl font-bold mb-4 mt-6 first:mt-0 tracking-tight',
        2: 'text-xl font-semibold mb-3 mt-8 text-gray-800',
        3: 'text-base font-semibold mb-2 mt-6 text-gray-700',
      };
      const className =
        styles[block.level || 1] ||
        'text-base font-semibold mb-2 mt-4';

      return (
        <Tag
          className={`${className} ${annotatedClass} ${selectedClass}`}
          data-block-id={block.id}
          onClick={handleClick}
        >
          <InlineContent content={block.content} />
          <AnnotationMarkers annotations={annotations} />
        </Tag>
      );
    }

    case 'paragraph':
      return (
        <p
          className={`mb-4 leading-relaxed text-gray-700 text-[15px] ${annotatedClass} ${selectedClass}`}
          data-block-id={block.id}
          onClick={handleClick}
        >
          <InlineContent content={block.content} />
          <AnnotationMarkers annotations={annotations} />
        </p>
      );

    case 'list-item': {
      const indent = (block.level || 0) * 1.25;
      const isCheckbox = block.checked !== undefined;
      const bullet =
        (block.level || 0) === 0
          ? '\u2022'
          : (block.level || 0) === 1
            ? '\u25E6'
            : '\u25AA';

      return (
        <div
          className={`flex gap-3 my-1.5 ${annotatedClass} ${selectedClass}`}
          data-block-id={block.id}
          style={{ marginLeft: `${indent}rem` }}
          onClick={handleClick}
        >
          <span className="select-none shrink-0 flex items-center text-gray-400">
            {isCheckbox ? (
              block.checked ? (
                <span className="text-green-600">&#x2713;</span>
              ) : (
                <span className="text-gray-400">&#x25CB;</span>
              )
            ) : (
              <span>{bullet}</span>
            )}
          </span>
          <span
            className={`text-sm leading-relaxed ${
              isCheckbox && block.checked
                ? 'text-gray-400 line-through'
                : 'text-gray-700'
            }`}
          >
            <InlineContent content={block.content} />
            <AnnotationMarkers annotations={annotations} />
          </span>
        </div>
      );
    }

    case 'blockquote':
      return (
        <blockquote
          className={`border-l-2 border-blue-300 pl-4 my-4 text-gray-500 italic ${annotatedClass} ${selectedClass}`}
          data-block-id={block.id}
          onClick={handleClick}
        >
          <InlineContent content={block.content} />
          <AnnotationMarkers annotations={annotations} />
        </blockquote>
      );

    case 'code':
      return (
        <div
          className={`my-5 relative group ${annotatedClass} ${selectedClass}`}
          data-block-id={block.id}
          onClick={handleClick}
        >
          {block.language && (
            <div className="absolute top-0 right-0 rounded-bl-lg rounded-tr-lg bg-gray-200 px-2 py-0.5 text-[10px] font-mono text-gray-500">
              {block.language}
            </div>
          )}
          <pre className="rounded-lg text-[13px] overflow-x-auto bg-gray-50 border border-gray-200 p-4">
            <code className="font-mono text-gray-800 whitespace-pre">
              {block.content}
            </code>
          </pre>
          <AnnotationMarkers annotations={annotations} />
        </div>
      );

    case 'table':
      return (
        <div
          className={`my-4 overflow-x-auto ${annotatedClass} ${selectedClass}`}
          data-block-id={block.id}
          onClick={handleClick}
        >
          <TableContent content={block.content} />
          <AnnotationMarkers annotations={annotations} />
        </div>
      );

    case 'hr':
      return (
        <hr
          className="border-gray-200 my-8"
          data-block-id={block.id}
        />
      );

    default:
      return (
        <p
          className={`mb-4 leading-relaxed text-gray-700 ${annotatedClass} ${selectedClass}`}
          data-block-id={block.id}
          onClick={handleClick}
        >
          <InlineContent content={block.content} />
          <AnnotationMarkers annotations={annotations} />
        </p>
      );
  }
});

// =============================================================================
// Inline Content Renderer
// =============================================================================

// InlineContent renders inline markdown: **bold**, *italic*, `code`, [links].
function InlineContent({ content }: { content: string }) {
  const parts: React.ReactNode[] = [];
  let remaining = content;
  let key = 0;

  while (remaining.length > 0) {
    // Bold: **text**.
    let match = remaining.match(/^\*\*(.+?)\*\*/);
    if (match) {
      parts.push(
        <strong key={key++} className="font-semibold">
          <InlineContent content={match[1]!} />
        </strong>,
      );
      remaining = remaining.slice(match[0].length);
      continue;
    }

    // Italic: *text*.
    match = remaining.match(/^\*(.+?)\*/);
    if (match) {
      parts.push(
        <em key={key++}>
          <InlineContent content={match[1]!} />
        </em>,
      );
      remaining = remaining.slice(match[0].length);
      continue;
    }

    // Inline code: `code`.
    match = remaining.match(/^`([^`]+)`/);
    if (match) {
      parts.push(
        <code
          key={key++}
          className="px-1.5 py-0.5 rounded bg-gray-100 text-sm font-mono text-gray-800"
        >
          {match[1]}
        </code>,
      );
      remaining = remaining.slice(match[0].length);
      continue;
    }

    // Links: [text](url) — only allow safe protocols.
    match = remaining.match(/^\[([^\]]+)\]\(([^)]+)\)/);
    if (match) {
      const linkUrl = match[2]!;
      const isSafe =
        linkUrl.startsWith('http://') ||
        linkUrl.startsWith('https://') ||
        linkUrl.startsWith('mailto:') ||
        linkUrl.startsWith('/') ||
        linkUrl.startsWith('#');

      if (isSafe) {
        parts.push(
          <a
            key={key++}
            href={linkUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="text-blue-600 underline underline-offset-2 hover:text-blue-800"
          >
            {match[1]}
          </a>,
        );
      } else {
        // Unsafe protocol (e.g., javascript:) — render as plain text.
        parts.push(
          <span key={key++} className="text-blue-600">
            {match[1]}
          </span>,
        );
      }
      remaining = remaining.slice(match[0].length);
      continue;
    }

    // Consume one character or until next special character.
    const nextSpecial = remaining.slice(1).search(/[*`[]/);
    if (nextSpecial === -1) {
      parts.push(remaining);
      break;
    } else {
      parts.push(remaining.slice(0, nextSpecial + 1));
      remaining = remaining.slice(nextSpecial + 1);
    }
  }

  return <>{parts}</>;
}

// =============================================================================
// Table Renderer
// =============================================================================

function TableContent({ content }: { content: string }) {
  const lines = content.split('\n').filter((l) => l.trim());
  if (lines.length === 0) return null;

  const parseRow = (line: string): string[] =>
    line
      .replace(/^\|/, '')
      .replace(/\|$/, '')
      .split('|')
      .map((cell) => cell.trim());

  const headers = parseRow(lines[0]!);
  const rows: string[][] = [];

  for (let i = 1; i < lines.length; i++) {
    const line = lines[i]!.trim();
    if (/^[|\-:\s]+$/.test(line)) continue;
    rows.push(parseRow(line));
  }

  return (
    <table className="min-w-full border-collapse text-sm">
      <thead>
        <tr className="border-b border-gray-200">
          {headers.map((header, i) => (
            <th
              key={i}
              className="px-3 py-2 text-left font-semibold text-gray-800 bg-gray-50"
            >
              <InlineContent content={header} />
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, rowIdx) => (
          <tr
            key={rowIdx}
            className="border-b border-gray-100 hover:bg-gray-50/50"
          >
            {row.map((cell, cellIdx) => (
              <td key={cellIdx} className="px-3 py-2 text-gray-600">
                <InlineContent content={cell} />
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// =============================================================================
// Annotation Markers
// =============================================================================

// AnnotationMarkers renders small colored indicators on annotated blocks.
function AnnotationMarkers({
  annotations,
}: {
  annotations: PlanAnnotation[];
}) {
  if (annotations.length === 0) return null;

  return (
    <span className="absolute -right-6 top-0 flex flex-col gap-0.5">
      {annotations.map((ann) => (
        <span
          key={ann.id}
          className={`inline-block h-2 w-2 rounded-full ${
            ann.type === PlanAnnotationType.COMMENT
              ? 'bg-yellow-400'
              : ann.type === PlanAnnotationType.DELETION
                ? 'bg-red-400'
                : ann.type === PlanAnnotationType.REPLACEMENT
                  ? 'bg-blue-400'
                  : ann.type === PlanAnnotationType.INSERTION
                    ? 'bg-green-400'
                    : 'bg-gray-400'
          }`}
          title={ann.text || ann.type}
        />
      ))}
    </span>
  );
}
