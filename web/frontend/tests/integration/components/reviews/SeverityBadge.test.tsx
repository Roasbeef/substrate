// Integration tests for SeverityBadge component.

import { describe, it, expect } from 'vitest';
import { render, screen } from '../../../utils.js';
import { SeverityBadge } from '@/components/reviews/SeverityBadge.js';
import type { IssueSeverity } from '@/types/api.js';

describe('SeverityBadge', () => {
  const severities: IssueSeverity[] = ['critical', 'major', 'minor', 'suggestion'];

  severities.forEach((severity) => {
    it(`renders badge for severity "${severity}"`, () => {
      render(<SeverityBadge severity={severity} />);

      expect(screen.getByText(severity)).toBeInTheDocument();
    });
  });

  it('shows "!!" icon for critical severity', () => {
    render(<SeverityBadge severity="critical" />);

    expect(screen.getByText('!!')).toBeInTheDocument();
  });

  it('shows "!" icon for major severity', () => {
    render(<SeverityBadge severity="major" />);

    expect(screen.getByText('!')).toBeInTheDocument();
  });

  it('shows "~" icon for minor severity', () => {
    render(<SeverityBadge severity="minor" />);

    expect(screen.getByText('~')).toBeInTheDocument();
  });

  it('shows "?" icon for suggestion severity', () => {
    render(<SeverityBadge severity="suggestion" />);

    expect(screen.getByText('?')).toBeInTheDocument();
  });

  it('uses red styling for critical severity', () => {
    const { container } = render(<SeverityBadge severity="critical" />);

    const badge = container.firstElementChild;
    expect(badge?.className).toContain('bg-red-100');
    expect(badge?.className).toContain('text-red-800');
  });

  it('uses blue styling for suggestion severity', () => {
    const { container } = render(<SeverityBadge severity="suggestion" />);

    const badge = container.firstElementChild;
    expect(badge?.className).toContain('bg-blue-100');
    expect(badge?.className).toContain('text-blue-800');
  });

  it('applies custom className', () => {
    const { container } = render(
      <SeverityBadge severity="minor" className="mt-2" />,
    );

    const badge = container.firstElementChild;
    expect(badge?.className).toContain('mt-2');
  });
});
