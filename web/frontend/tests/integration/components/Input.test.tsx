// Integration tests for the Input component.

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '../../utils.js';
import userEvent from '@testing-library/user-event';
import { Input, TextInput, Textarea } from '@/components/ui/Input';

describe('TextInput', () => {
  describe('rendering', () => {
    it('renders an input element', () => {
      render(<TextInput />);
      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });

    it('renders with placeholder', () => {
      render(<TextInput placeholder="Enter text" />);
      expect(screen.getByPlaceholderText('Enter text')).toBeInTheDocument();
    });

    it('renders with label', () => {
      render(<TextInput label="Username" />);
      expect(screen.getByLabelText('Username')).toBeInTheDocument();
    });

    it('renders required indicator with label', () => {
      render(<TextInput label="Username" required />);
      expect(screen.getByText('*')).toBeInTheDocument();
    });
  });

  describe('sizes', () => {
    it('renders medium size by default', () => {
      render(<TextInput />);
      const input = screen.getByRole('textbox');
      expect(input.className).toContain('py-2');
    });

    it('renders small size', () => {
      render(<TextInput size="sm" />);
      const input = screen.getByRole('textbox');
      expect(input.className).toContain('text-xs');
    });

    it('renders large size', () => {
      render(<TextInput size="lg" />);
      const input = screen.getByRole('textbox');
      expect(input.className).toContain('py-3');
    });
  });

  describe('validation', () => {
    it('displays error message', () => {
      render(<TextInput error="This field is required" />);
      expect(screen.getByText('This field is required')).toBeInTheDocument();
    });

    it('has error styling when error is present', () => {
      render(<TextInput error="Error" />);
      const input = screen.getByRole('textbox');
      expect(input.className).toContain('border-red-300');
    });

    it('sets aria-invalid when error is present', () => {
      render(<TextInput error="Error" />);
      expect(screen.getByRole('textbox')).toHaveAttribute('aria-invalid', 'true');
    });

    it('displays helper text', () => {
      render(<TextInput helperText="Enter your username" />);
      expect(screen.getByText('Enter your username')).toBeInTheDocument();
    });

    it('prioritizes error over helper text', () => {
      render(<TextInput helperText="Helper" error="Error" />);
      expect(screen.getByText('Error')).toBeInTheDocument();
      expect(screen.queryByText('Helper')).not.toBeInTheDocument();
    });
  });

  describe('icons', () => {
    it('renders left icon', () => {
      const icon = <span data-testid="left-icon">Icon</span>;
      render(<TextInput leftIcon={icon} />);
      expect(screen.getByTestId('left-icon')).toBeInTheDocument();
    });

    it('renders right icon', () => {
      const icon = <span data-testid="right-icon">Icon</span>;
      render(<TextInput rightIcon={icon} />);
      expect(screen.getByTestId('right-icon')).toBeInTheDocument();
    });

    it('adds left padding for left icon', () => {
      const icon = <span data-testid="left-icon">Icon</span>;
      render(<TextInput leftIcon={icon} />);
      const input = screen.getByRole('textbox');
      expect(input.className).toContain('pl-10');
    });

    it('adds right padding for right icon', () => {
      const icon = <span data-testid="right-icon">Icon</span>;
      render(<TextInput rightIcon={icon} />);
      const input = screen.getByRole('textbox');
      expect(input.className).toContain('pr-10');
    });
  });

  describe('disabled state', () => {
    it('is disabled when disabled prop is true', () => {
      render(<TextInput disabled />);
      expect(screen.getByRole('textbox')).toBeDisabled();
    });

    it('has disabled styling', () => {
      render(<TextInput disabled />);
      const input = screen.getByRole('textbox');
      expect(input.className).toContain('disabled:cursor-not-allowed');
    });
  });

  describe('interactions', () => {
    it('calls onChange when typing', async () => {
      const user = userEvent.setup();
      const onChange = vi.fn();
      render(<TextInput onChange={onChange} />);

      await user.type(screen.getByRole('textbox'), 'hello');
      expect(onChange).toHaveBeenCalled();
    });

    it('updates value when typing', async () => {
      const user = userEvent.setup();
      render(<TextInput />);

      const input = screen.getByRole('textbox');
      await user.type(input, 'hello');
      expect(input).toHaveValue('hello');
    });

    it('does not allow typing when disabled', async () => {
      const user = userEvent.setup();
      const onChange = vi.fn();
      render(<TextInput disabled onChange={onChange} />);

      await user.type(screen.getByRole('textbox'), 'hello');
      expect(onChange).not.toHaveBeenCalled();
    });

    it('calls onFocus when focused', async () => {
      const user = userEvent.setup();
      const onFocus = vi.fn();
      render(<TextInput onFocus={onFocus} />);

      await user.click(screen.getByRole('textbox'));
      expect(onFocus).toHaveBeenCalledTimes(1);
    });

    it('calls onBlur when blurred', async () => {
      const user = userEvent.setup();
      const onBlur = vi.fn();
      render(<TextInput onBlur={onBlur} />);

      const input = screen.getByRole('textbox');
      await user.click(input);
      await user.tab();
      expect(onBlur).toHaveBeenCalledTimes(1);
    });
  });

  describe('custom className', () => {
    it('applies custom className', () => {
      render(<TextInput className="custom-class" />);
      const input = screen.getByRole('textbox');
      expect(input.className).toContain('custom-class');
    });
  });
});

describe('Textarea', () => {
  describe('rendering', () => {
    it('renders a textarea element', () => {
      render(<Textarea />);
      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });

    it('renders with default rows', () => {
      render(<Textarea />);
      expect(screen.getByRole('textbox')).toHaveAttribute('rows', '4');
    });

    it('renders with custom rows', () => {
      render(<Textarea rows={6} />);
      expect(screen.getByRole('textbox')).toHaveAttribute('rows', '6');
    });

    it('renders with label', () => {
      render(<Textarea label="Description" />);
      expect(screen.getByLabelText('Description')).toBeInTheDocument();
    });

    it('renders with placeholder', () => {
      render(<Textarea placeholder="Enter description" />);
      expect(screen.getByPlaceholderText('Enter description')).toBeInTheDocument();
    });
  });

  describe('validation', () => {
    it('displays error message', () => {
      render(<Textarea error="Required" />);
      expect(screen.getByText('Required')).toBeInTheDocument();
    });

    it('has error styling when error is present', () => {
      render(<Textarea error="Error" />);
      const textarea = screen.getByRole('textbox');
      expect(textarea.className).toContain('border-red-300');
    });

    it('displays helper text', () => {
      render(<Textarea helperText="Maximum 500 characters" />);
      expect(screen.getByText('Maximum 500 characters')).toBeInTheDocument();
    });
  });

  describe('interactions', () => {
    it('allows typing', async () => {
      const user = userEvent.setup();
      render(<Textarea />);

      const textarea = screen.getByRole('textbox');
      await user.type(textarea, 'Hello\nWorld');
      expect(textarea).toHaveValue('Hello\nWorld');
    });

    it('is resizable', () => {
      render(<Textarea />);
      const textarea = screen.getByRole('textbox');
      expect(textarea.className).toContain('resize-y');
    });
  });

  describe('disabled state', () => {
    it('is disabled when disabled prop is true', () => {
      render(<Textarea disabled />);
      expect(screen.getByRole('textbox')).toBeDisabled();
    });
  });
});

describe('Input (combined)', () => {
  it('renders TextInput when multiline is false', () => {
    render(<Input multiline={false} placeholder="Text input" />);
    const input = screen.getByRole('textbox');
    expect(input.tagName).toBe('INPUT');
  });

  it('renders Textarea when multiline is true', () => {
    render(<Input multiline placeholder="Textarea" />);
    const input = screen.getByRole('textbox');
    expect(input.tagName).toBe('TEXTAREA');
  });

  it('renders TextInput by default', () => {
    render(<Input placeholder="Default" />);
    const input = screen.getByRole('textbox');
    expect(input.tagName).toBe('INPUT');
  });
});
