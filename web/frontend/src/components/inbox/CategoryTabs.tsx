// CategoryTabs component - tabs for switching between inbox categories.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Category types.
export type InboxCategory = 'primary' | 'agents' | 'topics';

// Tab item interface.
export interface TabItem {
  id: InboxCategory;
  label: string;
  count?: number;
  icon?: React.ReactNode;
}

// Default category tabs.
const defaultTabs: TabItem[] = [
  { id: 'primary', label: 'Primary' },
  { id: 'agents', label: 'Agents' },
  { id: 'topics', label: 'Topics' },
];

// CategoryTabs props.
export interface CategoryTabsProps {
  /** Currently selected category. */
  selected: InboxCategory;
  /** Handler for category change. */
  onSelect: (category: InboxCategory) => void;
  /** Custom tabs (overrides defaults). */
  tabs?: TabItem[];
  /** Whether tabs are disabled. */
  disabled?: boolean;
  /** Additional class name. */
  className?: string;
  /** Variant style. */
  variant?: 'underline' | 'pills';
}

// Tab button component.
interface TabButtonProps {
  tab: TabItem;
  isSelected: boolean;
  onClick: () => void;
  disabled?: boolean;
  variant: 'underline' | 'pills';
}

function TabButton({ tab, isSelected, onClick, disabled, variant }: TabButtonProps) {
  const baseStyles = 'relative flex items-center gap-2 text-sm font-medium transition-colors';

  if (variant === 'pills') {
    return (
      <button
        type="button"
        onClick={onClick}
        disabled={disabled}
        className={cn(
          baseStyles,
          'rounded-full px-4 py-2',
          isSelected
            ? 'bg-blue-100 text-blue-700'
            : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900',
          disabled ? 'cursor-not-allowed opacity-50' : '',
        )}
        aria-current={isSelected ? 'true' : undefined}
      >
        {tab.icon}
        <span>{tab.label}</span>
        {tab.count !== undefined && tab.count > 0 ? (
          <span
            className={cn(
              'ml-1 rounded-full px-2 py-0.5 text-xs',
              isSelected ? 'bg-blue-200 text-blue-800' : 'bg-gray-200 text-gray-600',
            )}
          >
            {tab.count}
          </span>
        ) : null}
      </button>
    );
  }

  // Underline variant.
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={cn(
        baseStyles,
        'border-b-2 pb-3 pt-2 px-1',
        isSelected
          ? 'border-blue-500 text-blue-600'
          : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700',
        disabled ? 'cursor-not-allowed opacity-50' : '',
      )}
      aria-current={isSelected ? 'true' : undefined}
    >
      {tab.icon}
      <span>{tab.label}</span>
      {tab.count !== undefined && tab.count > 0 ? (
        <span
          className={cn(
            'ml-2 rounded-full px-2 py-0.5 text-xs',
            isSelected ? 'bg-blue-100 text-blue-600' : 'bg-gray-100 text-gray-600',
          )}
        >
          {tab.count}
        </span>
      ) : null}
    </button>
  );
}

export function CategoryTabs({
  selected,
  onSelect,
  tabs = defaultTabs,
  disabled = false,
  className,
  variant = 'underline',
}: CategoryTabsProps) {
  return (
    <nav
      className={cn(
        'flex',
        variant === 'underline' ? 'border-b border-gray-200 gap-6' : 'gap-2',
        className,
      )}
      aria-label="Category tabs"
    >
      {tabs.map((tab) => (
        <TabButton
          key={tab.id}
          tab={tab}
          isSelected={selected === tab.id}
          onClick={() => onSelect(tab.id)}
          disabled={disabled}
          variant={variant}
        />
      ))}
    </nav>
  );
}

// Simplified tabs for just All/Unread/Starred filtering.
export type FilterType = 'all' | 'unread' | 'starred';

export interface FilterTabsProps {
  /** Currently selected filter. */
  selected: FilterType;
  /** Handler for filter change. */
  onSelect: (filter: FilterType) => void;
  /** Whether tabs are disabled. */
  disabled?: boolean;
  /** Additional class name. */
  className?: string;
}

const filterTabs: Array<{ id: FilterType; label: string }> = [
  { id: 'all', label: 'All' },
  { id: 'unread', label: 'Unread' },
  { id: 'starred', label: 'Starred' },
];

export function FilterTabs({
  selected,
  onSelect,
  disabled = false,
  className,
}: FilterTabsProps) {
  return (
    <div className={cn('flex gap-1 rounded-lg bg-gray-100 p-1', className)}>
      {filterTabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          onClick={() => onSelect(tab.id)}
          disabled={disabled}
          className={cn(
            'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
            selected === tab.id
              ? 'bg-white text-gray-900 shadow-sm'
              : 'text-gray-600 hover:text-gray-900',
            disabled ? 'cursor-not-allowed opacity-50' : '',
          )}
          aria-current={selected === tab.id ? 'true' : undefined}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}
