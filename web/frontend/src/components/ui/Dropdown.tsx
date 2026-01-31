// Dropdown menu component using Headless UI.

import { Fragment, type ReactNode } from 'react';
import { Menu, MenuButton, MenuItem, MenuItems, Transition } from '@headlessui/react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

export type DropdownAlign = 'left' | 'right';

export interface DropdownItem {
  /** Unique identifier for the item. */
  id: string;
  /** Display label for the item. */
  label: string;
  /** Optional icon to display before the label. */
  icon?: ReactNode | undefined;
  /** Whether the item is disabled. */
  disabled?: boolean | undefined;
  /** Whether to show a divider before this item. */
  divider?: boolean | undefined;
  /** Optional description text. */
  description?: string | undefined;
  /** Click handler. */
  onClick?: () => void;
}

export interface DropdownProps {
  /** The trigger button content. */
  trigger: ReactNode;
  /** Menu items to display. */
  items: DropdownItem[];
  /** Alignment of the dropdown. */
  align?: DropdownAlign | undefined;
  /** Additional class name for the trigger button. */
  triggerClassName?: string | undefined;
  /** Additional class name for the menu. */
  menuClassName?: string | undefined;
  /** Width of the dropdown menu. */
  width?: 'auto' | 'sm' | 'md' | 'lg' | undefined;
}

// Width styles mapping.
const widthStyles: Record<string, string> = {
  auto: 'min-w-[8rem]',
  sm: 'w-40',
  md: 'w-56',
  lg: 'w-72',
};

// Chevron icon for dropdown triggers.
function ChevronDownIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-4 w-4', className)}
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z"
        clipRule="evenodd"
      />
    </svg>
  );
}

export function Dropdown({
  trigger,
  items,
  align = 'right',
  triggerClassName,
  menuClassName,
  width = 'auto',
}: DropdownProps) {
  return (
    <Menu as="div" className="relative inline-block text-left">
      <MenuButton
        className={cn(
          'inline-flex items-center justify-center gap-1 rounded-md px-3 py-2 text-sm font-medium',
          'text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
          triggerClassName,
        )}
      >
        {trigger}
        <ChevronDownIcon className="text-gray-400" />
      </MenuButton>

      <Transition
        as={Fragment}
        enter="transition ease-out duration-100"
        enterFrom="transform opacity-0 scale-95"
        enterTo="transform opacity-100 scale-100"
        leave="transition ease-in duration-75"
        leaveFrom="transform opacity-100 scale-100"
        leaveTo="transform opacity-0 scale-95"
      >
        <MenuItems
          className={cn(
            'absolute z-10 mt-2 origin-top-right rounded-md bg-white shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none',
            align === 'left' ? 'left-0' : 'right-0',
            widthStyles[width],
            menuClassName,
          )}
        >
          <div className="py-1">
            {items.map((item) => (
              <Fragment key={item.id}>
                {item.divider ? (
                  <div className="my-1 border-t border-gray-100" />
                ) : null}
                <MenuItem {...(item.disabled ? { disabled: true } : {})}>
                  {({ focus }) => (
                    <button
                      type="button"
                      onClick={item.onClick}
                      {...(item.disabled ? { disabled: true } : {})}
                      className={cn(
                        'flex w-full items-center gap-2 px-4 py-2 text-left text-sm',
                        focus ? 'bg-gray-100 text-gray-900' : 'text-gray-700',
                        item.disabled ? 'cursor-not-allowed opacity-50' : '',
                      )}
                    >
                      {item.icon ? (
                        <span className="flex-shrink-0 text-gray-400">
                          {item.icon}
                        </span>
                      ) : null}
                      <span className="flex flex-col">
                        <span>{item.label}</span>
                        {item.description ? (
                          <span className="text-xs text-gray-500">
                            {item.description}
                          </span>
                        ) : null}
                      </span>
                    </button>
                  )}
                </MenuItem>
              </Fragment>
            ))}
          </div>
        </MenuItems>
      </Transition>
    </Menu>
  );
}

// Simple dropdown button that looks like a regular button.
export interface DropdownButtonProps {
  /** Button label. */
  label: string;
  /** Menu items. */
  items: DropdownItem[];
  /** Button variant. */
  variant?: 'primary' | 'secondary' | 'outline' | undefined;
  /** Dropdown alignment. */
  align?: DropdownAlign | undefined;
  /** Additional class name. */
  className?: string | undefined;
}

// Variant styles for dropdown button.
const buttonVariantStyles: Record<string, string> = {
  primary: 'bg-blue-600 text-white hover:bg-blue-700 focus:ring-blue-500',
  secondary: 'bg-gray-100 text-gray-900 hover:bg-gray-200 focus:ring-gray-500',
  outline: 'border border-gray-300 text-gray-700 hover:bg-gray-50 focus:ring-gray-500',
};

export function DropdownButton({
  label,
  items,
  variant = 'secondary',
  align = 'right',
  className,
}: DropdownButtonProps) {
  return (
    <Menu as="div" className="relative inline-block text-left">
      <MenuButton
        className={cn(
          'inline-flex items-center justify-center gap-2 rounded-md px-4 py-2 text-sm font-medium',
          'focus:outline-none focus:ring-2 focus:ring-offset-2',
          buttonVariantStyles[variant],
          className,
        )}
      >
        {label}
        <ChevronDownIcon />
      </MenuButton>

      <Transition
        as={Fragment}
        enter="transition ease-out duration-100"
        enterFrom="transform opacity-0 scale-95"
        enterTo="transform opacity-100 scale-100"
        leave="transition ease-in duration-75"
        leaveFrom="transform opacity-100 scale-100"
        leaveTo="transform opacity-0 scale-95"
      >
        <MenuItems
          className={cn(
            'absolute z-10 mt-2 min-w-[8rem] origin-top-right rounded-md bg-white shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none',
            align === 'left' ? 'left-0' : 'right-0',
          )}
        >
          <div className="py-1">
            {items.map((item) => (
              <Fragment key={item.id}>
                {item.divider ? (
                  <div className="my-1 border-t border-gray-100" />
                ) : null}
                <MenuItem {...(item.disabled ? { disabled: true } : {})}>
                  {({ focus }) => (
                    <button
                      type="button"
                      onClick={item.onClick}
                      {...(item.disabled ? { disabled: true } : {})}
                      className={cn(
                        'flex w-full items-center gap-2 px-4 py-2 text-left text-sm',
                        focus ? 'bg-gray-100 text-gray-900' : 'text-gray-700',
                        item.disabled ? 'cursor-not-allowed opacity-50' : '',
                      )}
                    >
                      {item.icon ? (
                        <span className="flex-shrink-0">{item.icon}</span>
                      ) : null}
                      {item.label}
                    </button>
                  )}
                </MenuItem>
              </Fragment>
            ))}
          </div>
        </MenuItems>
      </Transition>
    </Menu>
  );
}
