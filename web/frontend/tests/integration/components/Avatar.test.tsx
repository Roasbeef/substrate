// Integration tests for the Avatar component.

import { describe, it, expect } from 'vitest';
import { render, screen } from '../../utils.js';
import { Avatar, AvatarGroup, getInitials } from '@/components/ui/Avatar';

describe('Avatar', () => {
  describe('with image', () => {
    it('renders image when src is provided', () => {
      render(<Avatar src="https://example.com/avatar.jpg" alt="User" />);
      const img = screen.getByRole('img');
      expect(img).toHaveAttribute('src', 'https://example.com/avatar.jpg');
      expect(img).toHaveAttribute('alt', 'User');
    });

    it('applies correct size classes', () => {
      render(<Avatar src="https://example.com/avatar.jpg" alt="User" size="lg" />);
      const img = screen.getByRole('img');
      expect(img.className).toContain('h-12');
      expect(img.className).toContain('w-12');
    });
  });

  describe('with initials', () => {
    it('renders initials when no src', () => {
      render(<Avatar initials="JD" alt="John Doe" />);
      expect(screen.getByText('JD')).toBeInTheDocument();
    });

    it('renders fallback when no src or initials', () => {
      render(<Avatar alt="Unknown" />);
      expect(screen.getByText('?')).toBeInTheDocument();
    });

    it('applies aria-label when using initials', () => {
      render(<Avatar initials="JD" alt="John Doe" />);
      expect(screen.getByLabelText('John Doe')).toBeInTheDocument();
    });
  });

  describe('sizes', () => {
    it('renders xs size', () => {
      render(<Avatar initials="XS" size="xs" />);
      expect(screen.getByText('XS').className).toContain('h-6');
    });

    it('renders sm size', () => {
      render(<Avatar initials="SM" size="sm" />);
      expect(screen.getByText('SM').className).toContain('h-8');
    });

    it('renders md size by default', () => {
      render(<Avatar initials="MD" />);
      expect(screen.getByText('MD').className).toContain('h-10');
    });

    it('renders lg size', () => {
      render(<Avatar initials="LG" size="lg" />);
      expect(screen.getByText('LG').className).toContain('h-12');
    });

    it('renders xl size', () => {
      render(<Avatar initials="XL" size="xl" />);
      expect(screen.getByText('XL').className).toContain('h-16');
    });
  });

  describe('status indicator', () => {
    it('shows online status', () => {
      render(<Avatar initials="ON" status="online" />);
      expect(screen.getByLabelText('Status: online')).toBeInTheDocument();
    });

    it('shows offline status', () => {
      render(<Avatar initials="OF" status="offline" />);
      expect(screen.getByLabelText('Status: offline')).toBeInTheDocument();
    });

    it('shows busy status', () => {
      render(<Avatar initials="BY" status="busy" />);
      expect(screen.getByLabelText('Status: busy')).toBeInTheDocument();
    });

    it('shows away status', () => {
      render(<Avatar initials="AW" status="away" />);
      expect(screen.getByLabelText('Status: away')).toBeInTheDocument();
    });

    it('does not show status when not provided', () => {
      render(<Avatar initials="NS" />);
      expect(screen.queryByLabelText(/Status:/)).not.toBeInTheDocument();
    });
  });

  describe('custom className', () => {
    it('applies custom className', () => {
      const { container } = render(<Avatar initials="CC" className="custom-class" />);
      expect(container.firstChild).toHaveClass('custom-class');
    });
  });
});

describe('AvatarGroup', () => {
  it('renders multiple avatars', () => {
    render(
      <AvatarGroup>
        <Avatar initials="A1" />
        <Avatar initials="A2" />
        <Avatar initials="A3" />
      </AvatarGroup>,
    );
    expect(screen.getByText('A1')).toBeInTheDocument();
    expect(screen.getByText('A2')).toBeInTheDocument();
    expect(screen.getByText('A3')).toBeInTheDocument();
  });

  it('limits visible avatars when max is set', () => {
    render(
      <AvatarGroup max={2}>
        <Avatar initials="A1" />
        <Avatar initials="A2" />
        <Avatar initials="A3" />
        <Avatar initials="A4" />
      </AvatarGroup>,
    );
    expect(screen.getByText('A1')).toBeInTheDocument();
    expect(screen.getByText('A2')).toBeInTheDocument();
    expect(screen.queryByText('A3')).not.toBeInTheDocument();
    expect(screen.queryByText('A4')).not.toBeInTheDocument();
    expect(screen.getByText('+2')).toBeInTheDocument();
  });

  it('does not show overflow count when all avatars fit', () => {
    render(
      <AvatarGroup max={5}>
        <Avatar initials="A1" />
        <Avatar initials="A2" />
      </AvatarGroup>,
    );
    expect(screen.queryByText(/^\+/)).not.toBeInTheDocument();
  });
});

describe('getInitials', () => {
  it('extracts initials from single word', () => {
    expect(getInitials('John')).toBe('J');
  });

  it('extracts initials from two words', () => {
    expect(getInitials('John Doe')).toBe('JD');
  });

  it('extracts first two initials from three words', () => {
    expect(getInitials('John David Doe')).toBe('JD');
  });

  it('handles lowercase input', () => {
    expect(getInitials('jane smith')).toBe('JS');
  });

  it('handles custom max length', () => {
    expect(getInitials('John David Doe', 3)).toBe('JDD');
  });
});
