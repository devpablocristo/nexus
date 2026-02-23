import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { AskAgentPage } from '../../features/ask-agent/AskAgentPage';

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
        <AskAgentPage />
      </BrowserRouter>
    </QueryClientProvider>,
  );
}

describe('AskAgentPage', () => {
  it('renders the form with default query text', () => {
    renderPage();

    expect(screen.getByText('Ask Agent')).toBeInTheDocument();
    expect(screen.getByRole('textbox')).toHaveValue('Summarize the latest risk posture.');
    expect(screen.getByRole('button', { name: 'Query Operator' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Trigger Tick' })).toBeInTheDocument();
  });

  it('submits the form and calls queryAssistant API', async () => {
    const user = userEvent.setup();
    const mockResponse = {
      summary: 'Risk is moderate. Two incidents are open.',
      tables: [],
      actions: [],
    };

    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
      text: () => Promise.resolve(''),
    });

    renderPage();

    await user.click(screen.getByRole('button', { name: 'Query Operator' }));

    await waitFor(() => {
      const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
      const queryCall = calls.find(
        (c: [string, RequestInit?]) => typeof c[0] === 'string' && c[0].includes('/v1/assistant/query'),
      );
      expect(queryCall).toBeDefined();
      expect(queryCall![1]?.method).toBe('POST');

      const body = JSON.parse(queryCall![1]!.body as string);
      expect(body.query).toBe('Summarize the latest risk posture.');
    });
  });

  it('displays summary from successful query response', async () => {
    const user = userEvent.setup();
    const mockResponse = {
      summary: 'All systems nominal. No open incidents.',
      tables: [],
      actions: [],
    };

    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
      text: () => Promise.resolve(''),
    });

    renderPage();

    await user.click(screen.getByRole('button', { name: 'Query Operator' }));

    await waitFor(() => {
      expect(screen.getByText('Summary')).toBeInTheDocument();
      expect(screen.getByText('All systems nominal. No open incidents.')).toBeInTheDocument();
    });
  });

  it('displays tables from query response', async () => {
    const user = userEvent.setup();
    const mockResponse = {
      summary: 'Here are the active incidents.',
      tables: [
        {
          title: 'Active Incidents',
          columns: ['Severity', 'Title'],
          rows: [
            { Severity: 'HIGH', Title: 'CPU spike' },
            { Severity: 'LOW', Title: 'Disk warning' },
          ],
        },
      ],
      actions: [],
    };

    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
      text: () => Promise.resolve(''),
    });

    renderPage();

    await user.click(screen.getByRole('button', { name: 'Query Operator' }));

    await waitFor(() => {
      expect(screen.getByText('Active Incidents')).toBeInTheDocument();
    });

    expect(screen.getByText('CPU spike')).toBeInTheDocument();
    expect(screen.getByText('Disk warning')).toBeInTheDocument();
  });

  it('shows error display on query failure', async () => {
    const user = userEvent.setup();

    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve('Internal server error'),
    });

    renderPage();

    await user.click(screen.getByRole('button', { name: 'Query Operator' }));

    await waitFor(() => {
      expect(screen.getByText('Query failed')).toBeInTheDocument();
    });

    expect(screen.getByText(/API 500/)).toBeInTheDocument();
  });

  it('allows editing the query text before submitting', async () => {
    const user = userEvent.setup();

    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ summary: 'Custom response', tables: [], actions: [] }),
      text: () => Promise.resolve(''),
    });

    renderPage();

    const textarea = screen.getByRole('textbox');
    await user.clear(textarea);
    await user.type(textarea, 'List all open incidents');
    await user.click(screen.getByRole('button', { name: 'Query Operator' }));

    await waitFor(() => {
      const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
      const queryCall = calls.find(
        (c: [string, RequestInit?]) => typeof c[0] === 'string' && c[0].includes('/v1/assistant/query'),
      );
      expect(queryCall).toBeDefined();

      const body = JSON.parse(queryCall![1]!.body as string);
      expect(body.query).toBe('List all open incidents');
    });
  });

  it('shows button text as "Querying..." while request is pending', async () => {
    const user = userEvent.setup();

    // Never resolve so the mutation stays pending
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockReturnValue(new Promise(() => {}));

    renderPage();

    await user.click(screen.getByRole('button', { name: 'Query Operator' }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Querying...' })).toBeInTheDocument();
    });
  });

  it('shows error when Trigger Tick fails', async () => {
    const user = userEvent.setup();

    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      status: 502,
      text: () => Promise.resolve('Bad gateway'),
    });

    renderPage();

    await user.click(screen.getByRole('button', { name: 'Trigger Tick' }));

    await waitFor(() => {
      expect(screen.getByText('Tick failed')).toBeInTheDocument();
    });
  });

  it('shows success message when Trigger Tick succeeds', async () => {
    const user = userEvent.setup();

    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'ok' }),
      text: () => Promise.resolve(''),
    });

    renderPage();

    await user.click(screen.getByRole('button', { name: 'Trigger Tick' }));

    await waitFor(() => {
      expect(screen.getByText('Operator tick completed.')).toBeInTheDocument();
    });
  });
});
