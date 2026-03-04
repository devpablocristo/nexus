import { useState } from 'react';
import { useActiveTool } from '../../lib/tool-context';

const grafanaBase =
  import.meta.env.VITE_NEXUS_GRAFANA_URL || 'http://localhost:3000';

const DASHBOARD_UID = 'nexus-gateway-overview';

type PanelDef = { id: number; title: string; h: number };

const kpis: PanelDef[] = [
  { id: 1, title: 'Total Runs', h: 120 },
  { id: 2, title: 'Allow Rate', h: 120 },
  { id: 3, title: 'Deny Rate', h: 120 },
  { id: 4, title: 'Error Rate', h: 120 },
  { id: 5, title: 'Latency p95', h: 120 },
  { id: 6, title: 'Latency p50', h: 120 },
];

const charts: PanelDef[] = [
  { id: 7, title: 'Run Throughput', h: 260 },
  { id: 8, title: 'Latency Percentiles', h: 260 },
  { id: 9, title: 'Decisions Over Time', h: 260 },
  { id: 10, title: 'Blocked by Tool', h: 260 },
  { id: 11, title: 'Top Tools by Volume', h: 260 },
  { id: 12, title: 'Latency by Tool (p95)', h: 260 },
  { id: 13, title: 'HTTP Requests/s', h: 260 },
  { id: 14, title: 'Error Runs by Tool', h: 260 },
];

function panelUrl(panelId: number, theme: string, from: string, to: string, toolName: string | null): string {
  const varParam = toolName ? `&var-tool_name=${encodeURIComponent(toolName)}` : '';
  return `${grafanaBase}/d-solo/${DASHBOARD_UID}?orgId=1&panelId=${panelId}&theme=${theme}&from=${from}&to=${to}${varParam}`;
}

export function MonitoringPage() {
  const [range, setRange] = useState('now-1h');
  const { activeTool } = useActiveTool();
  const theme = 'dark';
  const from = range;
  const to = 'now';

  return (
    <div className="monitoring-page">
      <div className="monitoring-toolbar">
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
          href={`${grafanaBase}/d/${DASHBOARD_UID}?orgId=1&from=${from}&to=${to}`}
          target="_blank"
          rel="noopener noreferrer"
          className="btn-action"
        >
          Open in Grafana
        </a>
      </div>

      <div className="monitoring-kpis">
        {kpis.map((p) => (
          <div key={p.id} className="monitoring-panel" style={{ height: p.h }}>
            <iframe src={panelUrl(p.id, theme, from, to, activeTool)} title={p.title} frameBorder="0" />
          </div>
        ))}
      </div>

      <div className="monitoring-charts">
        {charts.map((p) => (
          <div key={p.id} className="monitoring-panel" style={{ height: p.h }}>
            <iframe src={panelUrl(p.id, theme, from, to, activeTool)} title={p.title} frameBorder="0" />
          </div>
        ))}
      </div>
    </div>
  );
}
