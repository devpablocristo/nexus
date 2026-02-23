import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { PoliciesPage } from '../../features/policies/PoliciesPage';

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
}

function renderPage() {
  const qc = createTestQueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <PoliciesPage />
      </BrowserRouter>
    </QueryClientProvider>,
  );
}

const mockProposals = {
  items: [
    {
      id: 'p1',
      status: 'pending' as const,
      rationale: 'Improve rate limiting for external APIs',
      diff: { rate_limit: { old: 100, new: 200 } },
      tests_suggested: ['test-rate-limit'],
      rollback_plan: 'Revert config change',
      created_at: '2025-01-15T10:00:00Z',
    },
    {
      id: 'p2',
      status: 'draft' as const,
      rationale: 'Add IP allowlist for admin endpoints',
      diff: { ip_allowlist: { added: ['10.0.0.0/8'] } },
      tests_suggested: ['test-ip-filter'],
      rollback_plan: 'Remove allowlist entries',
      created_at: '2025-01-14T08:00:00Z',
    },
  ],
};

function mockFetchForProposals() {
  (globalThis.fetch as ReturnType<typeof vi.fn>).mockImplementation((url: string, init?: RequestInit) => {
    // GET proposals list
    if (url.includes('/v1/policy-proposals') && (!init?.method || init.method === 'GET')) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(mockProposals),
        text: () => Promise.resolve(''),
      });
    }
    // POST approve/reject/shadow
    if (url.includes('/approve') || url.includes('/reject') || url.includes('/shadow')) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ ...mockProposals.items[0], status: 'approved' }),
        text: () => Promise.resolve(''),
      });
    }
    return Promise.resolve({
      ok: true,
      json: () => Promise.resolve({ items: [] }),
      text: () => Promise.resolve(''),
    });
  });
}

describe('PoliciesPage', () => {
  it('renders proposals from API', async () => {
    mockFetchForProposals();
    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Improve rate limiting for external APIs')).toBeInTheDocument();
    });

    expect(screen.getByText('Add IP allowlist for admin endpoints')).toBeInTheDocument();
    expect(screen.getByText('pending')).toBeInTheDocument();
    expect(screen.getByText('draft')).toBeInTheDocument();
  });

  it('renders approve, shadow, and reject buttons for each proposal', async () => {
    mockFetchForProposals();
    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Improve rate limiting for external APIs')).toBeInTheDocument();
    });

    const approveButtons = screen.getAllByRole('button', { name: 'Approve' });
    const shadowButtons = screen.getAllByRole('button', { name: 'Shadow' });
    const rejectButtons = screen.getAllByRole('button', { name: 'Reject' });

    expect(approveButtons).toHaveLength(2);
    expect(shadowButtons).toHaveLength(2);
    expect(rejectButtons).toHaveLength(2);
  });

  it('calls approve API when Approve button is clicked', async () => {
    const user = userEvent.setup();
    mockFetchForProposals();
    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Improve rate limiting for external APIs')).toBeInTheDocument();
    });

    const approveButtons = screen.getAllByRole('button', { name: 'Approve' });
    await user.click(approveButtons[0]);

    await waitFor(() => {
      const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
      const approveCall = calls.find(
        (c: [string, RequestInit?]) => typeof c[0] === 'string' && c[0].includes('/v1/policy-proposals/p1/approve'),
      );
      expect(approveCall).toBeDefined();
      expect(approveCall![1]?.method).toBe('POST');
    });
  });

  it('calls reject API when Reject button is clicked', async () => {
    const user = userEvent.setup();
    mockFetchForProposals();
    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Improve rate limiting for external APIs')).toBeInTheDocument();
    });

    const rejectButtons = screen.getAllByRole('button', { name: 'Reject' });
    await user.click(rejectButtons[0]);

    await waitFor(() => {
      const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
      const rejectCall = calls.find(
        (c: [string, RequestInit?]) => typeof c[0] === 'string' && c[0].includes('/v1/policy-proposals/p1/reject'),
      );
      expect(rejectCall).toBeDefined();
      expect(rejectCall![1]?.method).toBe('POST');
    });
  });

  it('calls shadow API when Shadow button is clicked', async () => {
    const user = userEvent.setup();
    mockFetchForProposals();
    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Improve rate limiting for external APIs')).toBeInTheDocument();
    });

    const shadowButtons = screen.getAllByRole('button', { name: 'Shadow' });
    await user.click(shadowButtons[0]);

    await waitFor(() => {
      const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
      const shadowCall = calls.find(
        (c: [string, RequestInit?]) => typeof c[0] === 'string' && c[0].includes('/v1/policy-proposals/p1/shadow'),
      );
      expect(shadowCall).toBeDefined();
      expect(shadowCall![1]?.method).toBe('POST');
    });
  });

  it('shows error when proposals API fails', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve('Internal server error'),
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Failed to load data')).toBeInTheDocument();
    });

    expect(screen.getByText(/API 500/)).toBeInTheDocument();
  });

  it('shows mutation error when action fails', async () => {
    const user = userEvent.setup();

    // First call succeeds (get proposals), subsequent POST calls fail
    let callCount = 0;
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes('/v1/policy-proposals') && (!init?.method || init.method === 'GET')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockProposals),
          text: () => Promise.resolve(''),
        });
      }
      if (init?.method === 'POST') {
        callCount++;
        return Promise.resolve({
          ok: false,
          status: 403,
          text: () => Promise.resolve('Forbidden'),
        });
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ items: [] }), text: () => Promise.resolve('') });
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Improve rate limiting for external APIs')).toBeInTheDocument();
    });

    const approveButtons = screen.getAllByRole('button', { name: 'Approve' });
    await user.click(approveButtons[0]);

    await waitFor(() => {
      expect(screen.getByText('Action failed')).toBeInTheDocument();
    });

    expect(screen.getByText(/API 403/)).toBeInTheDocument();
  });

  it('shows loading text while proposals are loading', () => {
    // Never resolve the fetch so the query stays in loading state
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockReturnValue(new Promise(() => {}));
    renderPage();

    expect(screen.getByText('Loading proposals...')).toBeInTheDocument();
  });
});
