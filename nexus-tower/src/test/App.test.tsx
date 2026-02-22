import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { App } from '../app/App';

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
}

function renderApp(initialRoute = '/') {
  window.history.pushState({}, '', initialRoute);
  const qc = createTestQueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>,
  );
}

function mockFetchSuccess(data: unknown = { items: [], next_cursor: 0 }) {
  (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
    ok: true,
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(''),
  });
}

describe('App', () => {
  it('renders all navigation links', () => {
    mockFetchSuccess();
    renderApp('/');

    expect(screen.getByRole('link', { name: 'Overview' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Acuario' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Timeline' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Policies' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Ask Agent' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Exports' })).toBeInTheDocument();
  });

  it('renders header branding', () => {
    mockFetchSuccess();
    renderApp('/');

    expect(screen.getByText('Nexus Tower')).toBeInTheDocument();
    expect(screen.getByText('Agent-Operated Supervision')).toBeInTheDocument();
  });

  it('shows OverviewPage on root route', () => {
    mockFetchSuccess();
    renderApp('/');

    expect(screen.getByText('Control Status')).toBeInTheDocument();
  });

  it('shows TimelinePage when navigating to /timeline', async () => {
    mockFetchSuccess();
    renderApp('/');
    const user = userEvent.setup();

    await user.click(screen.getByRole('link', { name: 'Timeline' }));

    expect(screen.getByText('Operational Timeline')).toBeInTheDocument();
  });

  it('shows AcuarioPage when navigating to /acuario', async () => {
    mockFetchSuccess();
    renderApp('/');
    const user = userEvent.setup();

    await user.click(screen.getByRole('link', { name: 'Acuario' }));

    expect(screen.getByText('Acuario 3D')).toBeInTheDocument();
  });

  it('shows PoliciesPage when navigating to /policies', async () => {
    mockFetchSuccess();
    renderApp('/');
    const user = userEvent.setup();

    await user.click(screen.getByRole('link', { name: 'Policies' }));

    expect(screen.getByText('Policy Proposals')).toBeInTheDocument();
  });

  it('shows AskAgentPage when navigating to /ask-agent', async () => {
    mockFetchSuccess();
    renderApp('/');
    const user = userEvent.setup();

    await user.click(screen.getByRole('link', { name: 'Ask Agent' }));

    expect(screen.getByRole('heading', { name: 'Ask Agent' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Query Operator' })).toBeInTheDocument();
  });

  it('shows ExportsPage when navigating to /exports', async () => {
    mockFetchSuccess();
    renderApp('/');
    const user = userEvent.setup();

    await user.click(screen.getByRole('link', { name: 'Exports' }));

    expect(screen.getByText('Exports & Compliance')).toBeInTheDocument();
  });
});
