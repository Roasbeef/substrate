// PlanToc renders a left-side table of contents for plan navigation.
// Shows heading hierarchy (h1-h3) with annotation count badges per section.

import { useCallback } from 'react';
import type { TocItem } from '@/lib/annotation-helpers.js';

export interface PlanTocProps {
  items: TocItem[];
  onScrollToBlock: (blockId: string) => void;
}

export function PlanToc({ items, onScrollToBlock }: PlanTocProps) {
  if (items.length === 0) return null;

  return (
    <nav className="flex flex-col">
      <h4 className="mb-3 px-3 text-xs font-semibold uppercase tracking-wider text-gray-500">
        Contents
      </h4>
      <div className="space-y-0.5">
        {items.map((item) => (
          <TocEntry
            key={item.id}
            item={item}
            onScrollToBlock={onScrollToBlock}
          />
        ))}
      </div>
    </nav>
  );
}

// TocEntry renders a single TOC entry with its children recursively.
function TocEntry({
  item,
  onScrollToBlock,
}: {
  item: TocItem;
  onScrollToBlock: (blockId: string) => void;
}) {
  const handleClick = useCallback(() => {
    onScrollToBlock(item.id);
  }, [item.id, onScrollToBlock]);

  // Indent based on heading level.
  const paddingLeft = `${(item.level - 1) * 0.75 + 0.75}rem`;

  return (
    <>
      <button
        type="button"
        onClick={handleClick}
        className="group flex w-full items-center justify-between rounded-md px-3 py-1.5 text-left text-xs text-gray-600 hover:bg-gray-100 hover:text-gray-900"
        style={{ paddingLeft }}
        title={item.content}
      >
        <span className="truncate">{item.content}</span>
        {item.annotationCount > 0 && (
          <span className="ml-2 flex-shrink-0 rounded-full bg-yellow-100 px-1.5 py-0.5 text-[10px] font-medium text-yellow-800">
            {item.annotationCount}
          </span>
        )}
      </button>
      {item.children.map((child) => (
        <TocEntry
          key={child.id}
          item={child}
          onScrollToBlock={onScrollToBlock}
        />
      ))}
    </>
  );
}
