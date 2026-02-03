// Integration tests for the Spinner component.

import { describe, it, expect } from 'vitest';
import { render, screen } from '../../utils.js';
import {
  Spinner,
  LoadingOverlay,
  InlineLoading,
  Skeleton,
  SkeletonText,
} from '@/components/ui/Spinner';

describe('Spinner', () => {
  describe('rendering', () => {
    it('renders with loading role', () => {
      render(<Spinner />);
      expect(screen.getByRole('status')).toBeInTheDocument();
    });

    it('has default aria-label', () => {
      render(<Spinner />);
      expect(screen.getByLabelText('Loading')).toBeInTheDocument();
    });

    it('uses custom label', () => {
      render(<Spinner label="Processing" />);
      expect(screen.getByLabelText('Processing')).toBeInTheDocument();
    });

    it('renders with screen reader text', () => {
      render(<Spinner label="Loading data" />);
      expect(screen.getByText('Loading data')).toHaveClass('sr-only');
    });
  });

  describe('sizes', () => {
    it('renders xs size', () => {
      const { container } = render(<Spinner size="xs" />);
      const svg = container.querySelector('svg');
      expect(svg?.className).toContain('h-3');
    });

    it('renders sm size', () => {
      const { container } = render(<Spinner size="sm" />);
      const svg = container.querySelector('svg');
      expect(svg?.className).toContain('h-4');
    });

    it('renders md size by default', () => {
      const { container } = render(<Spinner />);
      const svg = container.querySelector('svg');
      expect(svg?.className).toContain('h-6');
    });

    it('renders lg size', () => {
      const { container } = render(<Spinner size="lg" />);
      const svg = container.querySelector('svg');
      expect(svg?.className).toContain('h-8');
    });

    it('renders xl size', () => {
      const { container } = render(<Spinner size="xl" />);
      const svg = container.querySelector('svg');
      expect(svg?.className).toContain('h-12');
    });
  });

  describe('variants', () => {
    it('renders default variant', () => {
      const { container } = render(<Spinner variant="default" />);
      const svg = container.querySelector('svg');
      expect(svg?.className).toContain('text-gray-400');
    });

    it('renders primary variant', () => {
      const { container } = render(<Spinner variant="primary" />);
      const svg = container.querySelector('svg');
      expect(svg?.className).toContain('text-blue-600');
    });

    it('renders white variant', () => {
      const { container } = render(<Spinner variant="white" />);
      const svg = container.querySelector('svg');
      expect(svg?.className).toContain('text-white');
    });
  });

  describe('custom className', () => {
    it('applies custom className', () => {
      const { container } = render(<Spinner className="custom-spinner" />);
      expect(container.firstChild).toHaveClass('custom-spinner');
    });
  });
});

describe('LoadingOverlay', () => {
  it('does not render when isLoading is false', () => {
    render(<LoadingOverlay isLoading={false} />);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('renders when isLoading is true', () => {
    render(<LoadingOverlay isLoading />);
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('shows default message', () => {
    render(<LoadingOverlay isLoading />);
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('shows custom message', () => {
    render(<LoadingOverlay isLoading message="Please wait" />);
    expect(screen.getByText('Please wait')).toBeInTheDocument();
  });

  it('applies fullScreen styling', () => {
    render(<LoadingOverlay isLoading fullScreen />);
    expect(screen.getByRole('alert').className).toContain('fixed');
  });

  it('applies non-fullScreen styling by default', () => {
    render(<LoadingOverlay isLoading />);
    expect(screen.getByRole('alert').className).toContain('absolute');
  });

  it('has aria-busy attribute', () => {
    render(<LoadingOverlay isLoading />);
    expect(screen.getByRole('alert')).toHaveAttribute('aria-busy', 'true');
  });
});

describe('InlineLoading', () => {
  it('renders spinner', () => {
    render(<InlineLoading />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('shows text when provided', () => {
    render(<InlineLoading text="Loading items" />);
    // Find the visible text span (not sr-only)
    const textElements = screen.getAllByText('Loading items');
    const visibleText = textElements.find((el) => !el.classList.contains('sr-only'));
    expect(visibleText).toBeInTheDocument();
  });

  it('does not show text when not provided', () => {
    const { container } = render(<InlineLoading />);
    const textElements = container.querySelectorAll('.text-sm');
    expect(textElements.length).toBe(0);
  });

  it('applies custom className', () => {
    const { container } = render(<InlineLoading className="custom-inline" />);
    expect(container.firstChild).toHaveClass('custom-inline');
  });
});

describe('Skeleton', () => {
  it('renders with default dimensions', () => {
    const { container } = render(<Skeleton />);
    const skeleton = container.firstChild;
    expect(skeleton).toHaveClass('w-full');
    expect(skeleton).toHaveClass('h-4');
  });

  it('applies custom width', () => {
    const { container } = render(<Skeleton width="w-1/2" />);
    expect(container.firstChild).toHaveClass('w-1/2');
  });

  it('applies custom height', () => {
    const { container } = render(<Skeleton height="h-8" />);
    expect(container.firstChild).toHaveClass('h-8');
  });

  it('renders circle variant', () => {
    const { container } = render(<Skeleton circle />);
    expect(container.firstChild).toHaveClass('rounded-full');
  });

  it('renders normal variant by default', () => {
    const { container } = render(<Skeleton />);
    expect(container.firstChild).toHaveClass('rounded');
  });

  it('has pulse animation', () => {
    const { container } = render(<Skeleton />);
    expect(container.firstChild).toHaveClass('animate-pulse');
  });

  it('is hidden from accessibility tree', () => {
    const { container } = render(<Skeleton />);
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true');
  });
});

describe('SkeletonText', () => {
  it('renders default 3 lines', () => {
    const { container } = render(<SkeletonText />);
    const lines = container.querySelectorAll('.animate-pulse');
    expect(lines.length).toBe(3);
  });

  it('renders custom number of lines', () => {
    const { container } = render(<SkeletonText lines={5} />);
    const lines = container.querySelectorAll('.animate-pulse');
    expect(lines.length).toBe(5);
  });

  it('last line is shorter', () => {
    const { container } = render(<SkeletonText lines={3} />);
    const lines = container.querySelectorAll('.animate-pulse');
    expect(lines[2]).toHaveClass('w-3/4');
  });

  it('non-last lines are full width', () => {
    const { container } = render(<SkeletonText lines={3} />);
    const lines = container.querySelectorAll('.animate-pulse');
    expect(lines[0]).toHaveClass('w-full');
    expect(lines[1]).toHaveClass('w-full');
  });
});
