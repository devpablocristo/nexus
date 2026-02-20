import { render, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { OverviewPage } from '../../features/overview/OverviewPage';

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
    },
  });
}

function renderPage() {
  const qc = createTestQueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <OverviewPage />
      </BrowserRouter>
    </QueryClientProvider>,
  );
}

const mockEvents = {
  items: [
    { id: 1, event_type: 'scope.create', created_at: '2025-01-01T00:00:00Z', payload: {} },
    { id: 2, event_type: 'scope.create', created_at: '2025-01-01T01:00:00Z', payload: {} },
    { id: 3, event_type: 'action.execute', created_at: '2025-01-01T02:00:00Z', payload: {} },
  ],
  next_cursor: 3,
};

const mockActions = {
  items: [
    { id: 'a1', scope_type: 'repo', action_type: 'deploy', status: 'active', ttl_seconds: 300, created_at: '2025-01-01T00:00:00Z' },
    { id: 'a2', scope_type: 'infra', action_type: 'scale', status: 'completed', ttl_seconds: 600, created_at: '2025-01-01T01:00:00Z' },
  ],
};

const mockIncidents = {
  items: [
    { id: 'i1', severity: 'HIGH', status: 'open', title: 'CPU spike', summary: 'High CPU usage detected', opened_at: '2025-01-01T00:00:00Z' },
    { id: 'i2', severity: 'LOW', status: 'closed', title: 'Disk warning', summary: 'Disk usage above threshold', opened_at: '2025-01-01T01:00:00Z', closed_at: '2025-01-01T02:00:00Z' },
  ],
};

function mockFetchResponses(overrides: { events?: unknown; actions?: unknown; incidents?: unknown } = {}) {
  const events = overrides.events ?? mockEvents;
  const actions = overrides.actions ?? mockActions;
  const incidents = overrides.incidents ?? mockIncidents;

  (globalThis.fetch as ReturnType<typeof vi.fn>).mockImplementation((url: string) => {
    if (url.includes('/v1/events')) {
      return Promise.resolve({ ok: true, json: () => Promise.resolve(events), text: () => Promise.resolve('') });
    }
    if (url.includes('/v1/actions')) {
      return Promise.resolve({ ok: true, json: () => Promise.resolve(actions), text: () => Promise.resolve('') });
    }
    if (url.includes('/v1/incidents')) {
      return Promise.resolve({ ok: true, json: () => Promise.resolve(incidents), text: () => Promise.resolve('') });
    }
    return Promise.resolve({ ok: true, json: () => Promise.resolve({ items: [] }), text: () => Promise.resolve('') });
  });
}

describe('OverviewPage', () => {
  it('renders card titles', () => {
    mockFetchResponses();
    renderPage();

    expect(screen.getByText('Control Status')).toBeInTheDocument();
    expect(screen.getByText('Event Mix')).toBeInTheDocument();
    expect(screen.getByText('Latest Incidents')).toBeInTheDocument();
    expect(screen.getByText('Latest Actions')).toBeInTheDocument();
  });

  it('displays stats correctly with mock data', async () => {
    mockFetchResponses();
    renderPage();

    // Wait for the events count to appear (3 events)
    await waitFor(() => {
      expect(screen.getByText('3')).toBeInTheDocument();
    });

    // Stat labels
    expect(screen.getByText('Events')).toBeInTheDocument();
    expect(screen.getByText('Active Actions')).toBeInTheDocument();
    expect(screen.getByText('Open Incidents')).toBeInTheDocument();

    // Active actions: 1 (only a1 is 'active') — scoped to its stat block
    const actionsLabel = screen.getByText('Active Actions');
    const actionsBlock = actionsLabel.closest('div')!;
    expect(within(actionsBlock).getByText('1')).toBeInTheDocument();

    // Open incidents: 1 (only i1 is 'open') — scoped to its stat block
    const incidentsLabel = screen.getByText('Open Incidents');
    const incidentsBlock = incidentsLabel.closest('div')!;
    expect(within(incidentsBlock).getByText('1')).toBeInTheDocument();
  });

  it('displays incidents in the table', async () => {
    mockFetchResponses();
    renderPage();

    await waitFor(() => {
      expect(screen.getByText('CPU spike')).toBeInTheDocument();
    });

    expect(screen.getByText('HIGH')).toBeInTheDocument();
    expect(screen.getByText('Disk warning')).toBeInTheDocument();
  });

  it('displays actions in the table', async () => {
    mockFetchResponses();
    renderPage();

    await waitFor(() => {
      expect(screen.getByText('deploy')).toBeInTheDocument();
    });

    expect(screen.getByText('repo')).toBeInTheDocument();
    expect(screen.getByText('scale')).toBeInTheDocument();
    expect(screen.getByText('infra')).toBeInTheDocument();
  });

  it('displays error state when events API fails', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockImplementation((url: string) => {
      if (url.includes('/v1/events')) {
        return Promise.resolve({ ok: false, status: 500, text: () => Promise.resolve('Internal server error') });
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ items: [] }), text: () => Promise.resolve('') });
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getAllByText('Failed to load data').length).toBeGreaterThanOrEqual(1);
    });

    expect(screen.getAllByText(/API 500/).length).toBeGreaterThanOrEqual(1);
  });

  it('displays error state when incidents API fails', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockImplementation((url: string) => {
      if (url.includes('/v1/incidents')) {
        return Promise.resolve({ ok: false, status: 503, text: () => Promise.resolve('Service unavailable') });
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ items: [], next_cursor: 0 }), text: () => Promise.resolve('') });
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/API 503/)).toBeInTheDocument();
    });
  });

  it('shows zero counts when API returns empty lists', async () => {
    mockFetchResponses({
      events: { items: [], next_cursor: 0 },
      actions: { items: [] },
      incidents: { items: [] },
    });

    renderPage();

    // All three stat values should be 0
    await waitFor(() => {
      const zeros = screen.getAllByText('0');
      expect(zeros.length).toBe(3);
    });
  });
});
