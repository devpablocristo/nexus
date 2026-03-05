import { useState } from 'react';
import { useActiveTool } from '../../lib/tool-context';

const grafanaBase =
  import.meta.env.VITE_NEXUS_GRAFANA_URL || 'http://localhost:3000';

type PanelDef = { id: number; title: string; h: number };
type DashboardKey = 'gateway' | 'saas' | 'operators';

type DashboardConfig = {
  uid: string;
  supportsToolFilter: boolean;
  kpis: PanelDef[];
  charts: PanelDef[];
};

const dashboards: Record<DashboardKey, DashboardConfig> = {
  gateway: {
    uid: 'nexus-gateway-overview',
    supportsToolFilter: true,
    kpis: [
      { id: 1, title: 'Total Runs', h: 120 },
      { id: 2, title: 'Allow Rate', h: 120 },
      { id: 3, title: 'Deny Rate', h: 120 },
      { id: 4, title: 'Error Rate', h: 120 },
      { id: 5, title: 'Latency p95', h: 120 },
      { id: 6, title: 'Latency p50', h: 120 },
    ],
    charts: [
      { id: 7, title: 'Run Throughput', h: 260 },
      { id: 8, title: 'Latency Percentiles', h: 260 },
      { id: 9, title: 'Decisions Over Time', h: 260 },
      { id: 10, title: 'Blocked by Tool', h: 260 },
      { id: 11, title: 'Top Tools by Volume', h: 260 },
      { id: 12, title: 'Latency by Tool (p95)', h: 260 },
      { id: 13, title: 'HTTP Requests/s', h: 260 },
      { id: 14, title: 'Error Runs by Tool', h: 260 },
    ],
  },
  saas: {
    uid: 'nexus-saas-overview',
    supportsToolFilter: false,
    kpis: [
      { id: 3, title: 'Billing Checkouts (24h)', h: 120 },
      { id: 7, title: 'HTTP Latency p95', h: 120 },
      { id: 8, title: 'Error Rate (5xx)', h: 120 },
    ],
    charts: [
      { id: 1, title: 'HTTP Requests/s', h: 260 },
      { id: 2, title: 'Webhooks Received', h: 260 },
      { id: 4, title: 'Notifications Sent', h: 260 },
      { id: 5, title: 'Alert Evaluations', h: 260 },
      { id: 6, title: 'Alerts Fired', h: 260 },
    ],
  },
  operators: {
    uid: 'nexus-operators-overview',
    supportsToolFilter: false,
    kpis: [
      { id: 3, title: 'Consumer Offset', h: 120 },
      { id: 6, title: 'Actions Applied (1h)', h: 120 },
      { id: 7, title: 'Incidents Opened (1h)', h: 120 },
      { id: 8, title: 'Proposals Created (1h)', h: 120 },
      { id: 9, title: 'AI Last Cursor', h: 120 },
    ],
    charts: [
      { id: 1, title: 'Events Processed (control)', h: 260 },
      { id: 2, title: 'Processing Duration p95', h: 260 },
      { id: 4, title: 'Core API Calls (control)', h: 260 },
      { id: 5, title: 'Events Consumed (AI)', h: 260 },
    ],
  },
};

function panelURL(
  dashboardUID: string,
  panelID: number,
  theme: string,
  from: string,
  to: string,
  toolName: string | null,
): string {
  const varParam = toolName ? `&var-tool_name=${encodeURIComponent(toolName)}` : '';
  return `${grafanaBase}/d-solo/${dashboardUID}?orgId=1&panelId=${panelID}&theme=${theme}&from=${from}&to=${to}${varParam}`;
}

export function MonitoringPage() {
  const [range, setRange] = useState('now-1h');
  const [dashboard, setDashboard] = useState<DashboardKey>('gateway');
  const { activeTool } = useActiveTool();

  const selected = dashboards[dashboard];
  const theme = 'dark';
  const from = range;
  const to = 'now';
  const toolFilter = selected.supportsToolFilter ? activeTool : null;

  return (
    <div className="monitoring-page">
      <div className="monitoring-toolbar">
        <label>
          Dashboard
          <select value={dashboard} onChange={(e) => setDashboard(e.target.value as DashboardKey)}>
            <option value="gateway">Gateway</option>
            <option value="saas">SaaS</option>
            <option value="operators">Operators</option>
          </select>
        </label>

        <label>
          Time range
          <select value={range} onChange={(e) => setRange(e.target.value)}>
            <option value="now-15m">Last 15 min</option>
            <option value="now-1h">Last 1 hour</option>
            <option value="now-6h">Last 6 hours</option>
            <option value="now-24h">Last 24 hours</option>
            <option value="now-7d">Last 7 days</option>
          </select>
        </label>

        <a
          href={`${grafanaBase}/d/${selected.uid}?orgId=1&from=${from}&to=${to}`}
          target="_blank"
          rel="noopener noreferrer"
          className="btn-action"
        >
          Open in Grafana
        </a>
      </div>

      {!selected.supportsToolFilter ? (
        <p className="monitoring-hint">Tool filter applies only to the Gateway dashboard.</p>
      ) : null}

      <div className="monitoring-kpis">
        {selected.kpis.map((panel) => (
          <div key={panel.id} className="monitoring-panel" style={{ height: panel.h }}>
            <iframe
              src={panelURL(selected.uid, panel.id, theme, from, to, toolFilter)}
              title={panel.title}
              frameBorder="0"
            />
          </div>
        ))}
      </div>

      <div className="monitoring-charts">
        {selected.charts.map((panel) => (
          <div key={panel.id} className="monitoring-panel" style={{ height: panel.h }}>
            <iframe
              src={panelURL(selected.uid, panel.id, theme, from, to, toolFilter)}
              title={panel.title}
              frameBorder="0"
            />
          </div>
        ))}
      </div>
    </div>
  );
}
