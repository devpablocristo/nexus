import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { QueryError } from '../../components/QueryError';

describe('QueryError', () => {
  it('renders nothing when error is null', () => {
    const { container } = render(<QueryError error={null} />);
    expect(container.innerHTML).toBe('');
  });

  it('shows error message when error is provided', () => {
    const error = new Error('Network timeout');
    render(<QueryError error={error} />);

    expect(screen.getByText('Failed to load data')).toBeInTheDocument();
    expect(screen.getByText('Network timeout')).toBeInTheDocument();
  });

  it('does not show retry button when onRetry is not provided', () => {
    const error = new Error('Something broke');
    render(<QueryError error={error} />);

    expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument();
  });

  it('shows retry button when onRetry callback is given', () => {
    const error = new Error('Server error');
    const onRetry = vi.fn();
    render(<QueryError error={error} onRetry={onRetry} />);

    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  });

  it('calls onRetry when retry button is clicked', async () => {
    const user = userEvent.setup();
    const error = new Error('Server error');
    const onRetry = vi.fn();
    render(<QueryError error={error} onRetry={onRetry} />);

    await user.click(screen.getByRole('button', { name: 'Retry' }));

    expect(onRetry).toHaveBeenCalledOnce();
  });
});
