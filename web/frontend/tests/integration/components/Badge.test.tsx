// Integration tests for the Badge component.

import { describe, it, expect } from 'vitest';
import { render, screen } from '../../utils.js';
import { Badge, StatusBadge, PriorityBadge } from '@/components/ui/Badge';

describe('Badge', () => {
  describe('rendering', () => {
    it('renders children text', () => {
      render(<Badge>Label</Badge>);
      expect(screen.getByText('Label')).toBeInTheDocument();
    });

    it('renders as a span element', () => {
      render(<Badge>Label</Badge>);
      expect(screen.getByText('Label').tagName).toBe('SPAN');
    });
  });

  describe('variants', () => {
    it('renders default variant', () => {
      render(<Badge variant="default">Default</Badge>);
      expect(screen.getByText('Default').className).toContain('bg-gray-100');
    });

    it('renders success variant', () => {
      render(<Badge variant="success">Success</Badge>);
      expect(screen.getByText('Success').className).toContain('bg-green-100');
    });

    it('renders warning variant', () => {
      render(<Badge variant="warning">Warning</Badge>);
      expect(screen.getByText('Warning').className).toContain('bg-yellow-100');
    });

    it('renders error variant', () => {
      render(<Badge variant="error">Error</Badge>);
      expect(screen.getByText('Error').className).toContain('bg-red-100');
    });

    it('renders info variant', () => {
      render(<Badge variant="info">Info</Badge>);
      expect(screen.getByText('Info').className).toContain('bg-blue-100');
    });

    it('renders outline variant', () => {
      render(<Badge variant="outline">Outline</Badge>);
      expect(screen.getByText('Outline').className).toContain('border');
    });
  });

  describe('sizes', () => {
    it('renders small size', () => {
      render(<Badge size="sm">Small</Badge>);
      expect(screen.getByText('Small').className).toContain('px-1.5');
    });

    it('renders medium size by default', () => {
      render(<Badge>Medium</Badge>);
      expect(screen.getByText('Medium').className).toContain('px-2');
    });

    it('renders large size', () => {
      render(<Badge size="lg">Large</Badge>);
      expect(screen.getByText('Large').className).toContain('px-2.5');
    });
  });

  describe('dot indicator', () => {
    it('shows dot when withDot is true', () => {
      render(<Badge withDot>With Dot</Badge>);
      const badge = screen.getByText('With Dot');
      const dot = badge.querySelector('[aria-hidden="true"]');
      expect(dot).toBeInTheDocument();
    });

    it('does not show dot by default', () => {
      render(<Badge>No Dot</Badge>);
      const badge = screen.getByText('No Dot');
      const dot = badge.querySelector('[aria-hidden="true"]');
      expect(dot).not.toBeInTheDocument();
    });

    it('uses custom dot color', () => {
      render(<Badge withDot dotColor="bg-purple-500">Custom Dot</Badge>);
      const badge = screen.getByText('Custom Dot');
      const dot = badge.querySelector('[aria-hidden="true"]');
      expect(dot?.className).toContain('bg-purple-500');
    });
  });

  describe('custom className', () => {
    it('applies custom className', () => {
      render(<Badge className="custom-class">Custom</Badge>);
      expect(screen.getByText('Custom').className).toContain('custom-class');
    });
  });
});

describe('StatusBadge', () => {
  it('renders active status', () => {
    render(<StatusBadge status="active" />);
    expect(screen.getByText('Active')).toBeInTheDocument();
  });

  it('renders busy status', () => {
    render(<StatusBadge status="busy" />);
    expect(screen.getByText('Busy')).toBeInTheDocument();
  });

  it('renders idle status', () => {
    render(<StatusBadge status="idle" />);
    expect(screen.getByText('Idle')).toBeInTheDocument();
  });

  it('renders offline status', () => {
    render(<StatusBadge status="offline" />);
    expect(screen.getByText('Offline')).toBeInTheDocument();
  });

  it('hides label when showLabel is false', () => {
    render(<StatusBadge status="active" showLabel={false} />);
    expect(screen.queryByText('Active')).not.toBeInTheDocument();
  });

  it('applies custom className', () => {
    render(<StatusBadge status="active" className="custom-status" />);
    expect(screen.getByText('Active').className).toContain('custom-status');
  });
});

describe('PriorityBadge', () => {
  it('renders low priority', () => {
    render(<PriorityBadge priority="low" />);
    expect(screen.getByText('Low')).toBeInTheDocument();
  });

  it('renders normal priority', () => {
    render(<PriorityBadge priority="normal" />);
    expect(screen.getByText('Normal')).toBeInTheDocument();
  });

  it('renders high priority', () => {
    render(<PriorityBadge priority="high" />);
    expect(screen.getByText('High')).toBeInTheDocument();
  });

  it('renders urgent priority', () => {
    render(<PriorityBadge priority="urgent" />);
    expect(screen.getByText('Urgent')).toBeInTheDocument();
  });

  it('applies correct variant for urgent', () => {
    render(<PriorityBadge priority="urgent" />);
    expect(screen.getByText('Urgent').className).toContain('bg-red-100');
  });
});
