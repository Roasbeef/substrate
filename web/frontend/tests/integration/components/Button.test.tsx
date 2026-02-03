// Integration tests for the Button component.

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '../../utils.js';
import userEvent from '@testing-library/user-event';
import { Button } from '@/components/ui/Button';

describe('Button', () => {
  describe('rendering', () => {
    it('renders children text', () => {
      render(<Button>Click me</Button>);
      expect(screen.getByRole('button', { name: 'Click me' })).toBeInTheDocument();
    });

    it('renders with default type="button"', () => {
      render(<Button>Click me</Button>);
      expect(screen.getByRole('button')).toHaveAttribute('type', 'button');
    });
  });

  describe('variants', () => {
    it('renders primary variant by default', () => {
      render(<Button>Primary</Button>);
      const button = screen.getByRole('button');
      expect(button.className).toContain('bg-blue-600');
    });

    it('renders secondary variant', () => {
      render(<Button variant="secondary">Secondary</Button>);
      const button = screen.getByRole('button');
      expect(button.className).toContain('bg-gray-100');
    });

    it('renders outline variant', () => {
      render(<Button variant="outline">Outline</Button>);
      const button = screen.getByRole('button');
      expect(button.className).toContain('border');
    });

    it('renders ghost variant', () => {
      render(<Button variant="ghost">Ghost</Button>);
      const button = screen.getByRole('button');
      expect(button.className).toContain('hover:bg-gray-100');
    });

    it('renders danger variant', () => {
      render(<Button variant="danger">Danger</Button>);
      const button = screen.getByRole('button');
      expect(button.className).toContain('bg-red-600');
    });
  });

  describe('sizes', () => {
    it('renders medium size by default', () => {
      render(<Button>Medium</Button>);
      const button = screen.getByRole('button');
      expect(button.className).toContain('px-4');
      expect(button.className).toContain('py-2');
    });

    it('renders small size', () => {
      render(<Button size="sm">Small</Button>);
      const button = screen.getByRole('button');
      expect(button.className).toContain('text-xs');
    });

    it('renders large size', () => {
      render(<Button size="lg">Large</Button>);
      const button = screen.getByRole('button');
      expect(button.className).toContain('px-6');
      expect(button.className).toContain('py-3');
    });
  });

  describe('loading state', () => {
    it('shows spinner when loading', () => {
      render(<Button isLoading>Loading</Button>);
      const spinner = screen.getByRole('button').querySelector('svg');
      expect(spinner).toBeInTheDocument();
      expect(spinner?.classList.contains('animate-spin')).toBe(true);
    });

    it('disables button when loading', () => {
      render(<Button isLoading>Loading</Button>);
      expect(screen.getByRole('button')).toBeDisabled();
    });

    it('hides left icon when loading', () => {
      const icon = <span data-testid="left-icon">Icon</span>;
      render(
        <Button isLoading leftIcon={icon}>
          Loading
        </Button>,
      );
      expect(screen.queryByTestId('left-icon')).not.toBeInTheDocument();
    });
  });

  describe('icons', () => {
    it('renders left icon', () => {
      const icon = <span data-testid="left-icon">Icon</span>;
      render(<Button leftIcon={icon}>With Icon</Button>);
      expect(screen.getByTestId('left-icon')).toBeInTheDocument();
    });

    it('renders right icon', () => {
      const icon = <span data-testid="right-icon">Icon</span>;
      render(<Button rightIcon={icon}>With Icon</Button>);
      expect(screen.getByTestId('right-icon')).toBeInTheDocument();
    });

    it('renders both icons', () => {
      const leftIcon = <span data-testid="left-icon">Left</span>;
      const rightIcon = <span data-testid="right-icon">Right</span>;
      render(
        <Button leftIcon={leftIcon} rightIcon={rightIcon}>
          Both
        </Button>,
      );
      expect(screen.getByTestId('left-icon')).toBeInTheDocument();
      expect(screen.getByTestId('right-icon')).toBeInTheDocument();
    });
  });

  describe('disabled state', () => {
    it('is disabled when disabled prop is true', () => {
      render(<Button disabled>Disabled</Button>);
      expect(screen.getByRole('button')).toBeDisabled();
    });

    it('has disabled cursor style', () => {
      render(<Button disabled>Disabled</Button>);
      expect(screen.getByRole('button').className).toContain(
        'disabled:cursor-not-allowed',
      );
    });
  });

  describe('interactions', () => {
    it('calls onClick when clicked', async () => {
      const user = userEvent.setup();
      const onClick = vi.fn();
      render(<Button onClick={onClick}>Click me</Button>);

      await user.click(screen.getByRole('button'));
      expect(onClick).toHaveBeenCalledTimes(1);
    });

    it('does not call onClick when disabled', async () => {
      const user = userEvent.setup();
      const onClick = vi.fn();
      render(
        <Button disabled onClick={onClick}>
          Click me
        </Button>,
      );

      await user.click(screen.getByRole('button'));
      expect(onClick).not.toHaveBeenCalled();
    });

    it('does not call onClick when loading', async () => {
      const user = userEvent.setup();
      const onClick = vi.fn();
      render(
        <Button isLoading onClick={onClick}>
          Click me
        </Button>,
      );

      await user.click(screen.getByRole('button'));
      expect(onClick).not.toHaveBeenCalled();
    });
  });

  describe('custom className', () => {
    it('applies custom className', () => {
      render(<Button className="custom-class">Custom</Button>);
      expect(screen.getByRole('button').className).toContain('custom-class');
    });

    it('merges custom className with defaults', () => {
      render(<Button className="w-full">Full Width</Button>);
      const button = screen.getByRole('button');
      expect(button.className).toContain('w-full');
      expect(button.className).toContain('inline-flex');
    });
  });
});
