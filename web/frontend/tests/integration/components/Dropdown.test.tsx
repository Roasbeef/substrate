// Integration tests for the Dropdown component.

import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '../../utils.js';
import userEvent from '@testing-library/user-event';
import { Dropdown, DropdownButton } from '@/components/ui/Dropdown';

const mockItems = [
  { id: '1', label: 'Item 1', onClick: vi.fn() },
  { id: '2', label: 'Item 2', onClick: vi.fn() },
  { id: '3', label: 'Item 3', onClick: vi.fn(), disabled: true },
];

describe('Dropdown', () => {
  describe('rendering', () => {
    it('renders trigger button', () => {
      render(<Dropdown trigger="Options" items={mockItems} />);
      expect(screen.getByRole('button', { name: /options/i })).toBeInTheDocument();
    });

    it('menu is hidden initially', () => {
      render(<Dropdown trigger="Options" items={mockItems} />);
      expect(screen.queryByText('Item 1')).not.toBeInTheDocument();
    });
  });

  describe('interactions', () => {
    it('opens menu on click', async () => {
      const user = userEvent.setup();
      render(<Dropdown trigger="Options" items={mockItems} />);

      await user.click(screen.getByRole('button', { name: /options/i }));

      await waitFor(() => {
        expect(screen.getByText('Item 1')).toBeInTheDocument();
        expect(screen.getByText('Item 2')).toBeInTheDocument();
      });
    });

    it('closes menu on second click', async () => {
      const user = userEvent.setup();
      render(<Dropdown trigger="Options" items={mockItems} />);

      const trigger = screen.getByRole('button', { name: /options/i });
      await user.click(trigger);
      await waitFor(() => {
        expect(screen.getByText('Item 1')).toBeInTheDocument();
      });

      await user.click(trigger);
      await waitFor(() => {
        expect(screen.queryByText('Item 1')).not.toBeInTheDocument();
      });
    });

    it('calls onClick when item is clicked', async () => {
      const user = userEvent.setup();
      const onClick = vi.fn();
      const items = [{ id: '1', label: 'Click Me', onClick }];
      render(<Dropdown trigger="Options" items={items} />);

      await user.click(screen.getByRole('button', { name: /options/i }));
      await waitFor(() => {
        expect(screen.getByText('Click Me')).toBeInTheDocument();
      });

      await user.click(screen.getByText('Click Me'));
      expect(onClick).toHaveBeenCalledTimes(1);
    });

    it('does not call onClick for disabled items', async () => {
      const user = userEvent.setup();
      const onClick = vi.fn();
      const items = [{ id: '1', label: 'Disabled', onClick, disabled: true }];
      render(<Dropdown trigger="Options" items={items} />);

      await user.click(screen.getByRole('button', { name: /options/i }));
      await waitFor(() => {
        expect(screen.getByText('Disabled')).toBeInTheDocument();
      });

      await user.click(screen.getByText('Disabled'));
      expect(onClick).not.toHaveBeenCalled();
    });
  });

  describe('features', () => {
    it('renders icon when provided', async () => {
      const user = userEvent.setup();
      const icon = <span data-testid="item-icon">★</span>;
      const items = [{ id: '1', label: 'With Icon', icon }];
      render(<Dropdown trigger="Options" items={items} />);

      await user.click(screen.getByRole('button', { name: /options/i }));
      await waitFor(() => {
        expect(screen.getByTestId('item-icon')).toBeInTheDocument();
      });
    });

    it('renders description when provided', async () => {
      const user = userEvent.setup();
      const items = [{ id: '1', label: 'Item', description: 'Description text' }];
      render(<Dropdown trigger="Options" items={items} />);

      await user.click(screen.getByRole('button', { name: /options/i }));
      await waitFor(() => {
        expect(screen.getByText('Description text')).toBeInTheDocument();
      });
    });

    it('renders divider when specified', async () => {
      const user = userEvent.setup();
      const items = [
        { id: '1', label: 'Before' },
        { id: '2', label: 'After', divider: true },
      ];
      render(<Dropdown trigger="Options" items={items} />);

      await user.click(screen.getByRole('button', { name: /options/i }));
      await waitFor(() => {
        const menu = screen.getByRole('menu');
        const divider = menu.querySelector('.border-t');
        expect(divider).toBeInTheDocument();
      });
    });
  });

  describe('alignment', () => {
    it('aligns right by default', async () => {
      const user = userEvent.setup();
      render(<Dropdown trigger="Options" items={mockItems} />);

      await user.click(screen.getByRole('button', { name: /options/i }));
      await waitFor(() => {
        const menu = screen.getByRole('menu');
        expect(menu.className).toContain('right-0');
      });
    });

    it('aligns left when specified', async () => {
      const user = userEvent.setup();
      render(<Dropdown trigger="Options" items={mockItems} align="left" />);

      await user.click(screen.getByRole('button', { name: /options/i }));
      await waitFor(() => {
        const menu = screen.getByRole('menu');
        expect(menu.className).toContain('left-0');
      });
    });
  });

  describe('custom className', () => {
    it('applies trigger className', () => {
      render(
        <Dropdown
          trigger="Options"
          items={mockItems}
          triggerClassName="custom-trigger"
        />,
      );
      expect(screen.getByRole('button').className).toContain('custom-trigger');
    });

    it('applies menu className', async () => {
      const user = userEvent.setup();
      render(
        <Dropdown
          trigger="Options"
          items={mockItems}
          menuClassName="custom-menu"
        />,
      );

      await user.click(screen.getByRole('button', { name: /options/i }));
      await waitFor(() => {
        expect(screen.getByRole('menu').className).toContain('custom-menu');
      });
    });
  });
});

describe('DropdownButton', () => {
  it('renders with label', () => {
    render(<DropdownButton label="Actions" items={mockItems} />);
    expect(screen.getByRole('button', { name: /actions/i })).toBeInTheDocument();
  });

  it('opens menu on click', async () => {
    const user = userEvent.setup();
    render(<DropdownButton label="Actions" items={mockItems} />);

    await user.click(screen.getByRole('button', { name: /actions/i }));
    await waitFor(() => {
      expect(screen.getByText('Item 1')).toBeInTheDocument();
    });
  });

  describe('variants', () => {
    it('renders primary variant', () => {
      render(<DropdownButton label="Primary" items={mockItems} variant="primary" />);
      expect(screen.getByRole('button').className).toContain('bg-blue-600');
    });

    it('renders secondary variant by default', () => {
      render(<DropdownButton label="Secondary" items={mockItems} />);
      expect(screen.getByRole('button').className).toContain('bg-gray-100');
    });

    it('renders outline variant', () => {
      render(<DropdownButton label="Outline" items={mockItems} variant="outline" />);
      expect(screen.getByRole('button').className).toContain('border');
    });
  });
});
