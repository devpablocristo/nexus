import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

import { Card } from '../../components/Card';
import { QueryError } from '../../components/QueryError';
import { getAuditEvents, type AuditQueryParams } from '../../lib/api';
import type { AuditEventItem } from '../../lib/types';

const DECISION_OPTIONS = ['', 'allow', 'deny'] as const;
const STATUS_OPTIONS = ['', 'success', 'error', 'blocked'] as const;

function formatTime(iso: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  } catch {
    return iso;
  }
}

function decisionPillClass(decision: string, status: string): string {
  if (status === 'error') return 'run-pill-error';
  if (status === 'blocked') return 'run-pill-blocked';
  if (decision === 'deny') return 'run-pill-deny';
  return 'run-pill-allow';
}

function toRFC3339(value: string): string | undefined {
  if (!value.trim()) return undefined;
  const d = new Date(value);
  return isNaN(d.getTime()) ? undefined : d.toISOString();
}

export function RunExplorerPage() {
  const [toolFilter, setToolFilter] = useState('');
  const [decisionFilter, setDecisionFilter] = useState<'' | 'allow' | 'deny'>('');
  const [statusFilter, setStatusFilter] = useState<'' | 'success' | 'error' | 'blocked'>('');
  const [fromFilter, setFromFilter] = useState('');
  const [toFilter, setToFilter] = useState('');
  const [limitFilter, setLimitFilter] = useState(300);
  const [selectedEvent, setSelectedEvent] = useState<AuditEventItem | null>(null);

  const queryParams: AuditQueryParams = useMemo(() => {
    const p: AuditQueryParams = { limit: limitFilter };
    if (toolFilter.trim()) p.tool_name = toolFilter.trim();
    if (decisionFilter) p.decision = decisionFilter;
    if (statusFilter) p.status = statusFilter;
    const from = toRFC3339(fromFilter);
    const to = toRFC3339(toFilter);
    if (from) p.from = from;
    if (to) p.to = to;
    return p;
  }, [toolFilter, decisionFilter, statusFilter, fromFilter, toFilter, limitFilter]);

  const query = useQuery({
    queryKey: ['audit', queryParams],
    queryFn: () => getAuditEvents(queryParams),
    refetchInterval: 10000,
  });

  const items = query.data?.items ?? [];
  const uniqueTools = useMemo(() => {
    const set = new Set<string>();
    items.forEach((e) => set.add(e.tool_name));
    return Array.from(set).sort();
  }, [items]);

  const summaryByTool = useMemo(() => {
    const byTool = new Map<string, { allow: number; deny: number; error: number; blocked: number }>();
    items.forEach((e) => {
      const cur = byTool.get(e.tool_name) ?? { allow: 0, deny: 0, error: 0, blocked: 0 };
      if (e.status === 'error') cur.error++;
      else if (e.status === 'blocked') cur.blocked++;
      else if (e.decision === 'deny') cur.deny++;
      else cur.allow++;
      byTool.set(e.tool_name, cur);
    });
    return Array.from(byTool.entries()).map(([tool, counts]) => ({
      tool,
      ...counts,
      total: counts.allow + counts.deny + counts.error + counts.blocked,
    }));
  }, [items]);

  const chartData = useMemo(
    () =>
      summaryByTool.slice(0, 12).map((s) => ({
        tool: s.tool.length > 14 ? s.tool.slice(0, 12) + '…' : s.tool,
        allow: s.allow,
        deny: s.deny,
        error: s.error,
        blocked: s.blocked,
      })),
    [summaryByTool]
  );

  const isTestMode = import.meta.env.MODE === 'test';

  return (
    <div className="run-explorer">
      <Card title="Run Explorer">
        <p className="muted run-explorer-desc">
          Timeline de ejecuciones (tool call → decisión → resultado). Filtros por tool, decisión y estado.
        </p>

        <div className="run-explorer-filters">
          <label>
            <span>Tool</span>
            <select
              value={toolFilter}
              onChange={(e) => setToolFilter(e.target.value)}
              className="run-filter-select"
            >
              <option value="">Todos</option>
              {uniqueTools.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>Decisión</span>
            <select
              value={decisionFilter}
              onChange={(e) => setDecisionFilter((e.target.value || '') as '' | 'allow' | 'deny')}
              className="run-filter-select"
            >
              <option value="">Todas</option>
              {DECISION_OPTIONS.filter(Boolean).map((d) => (
                <option key={d} value={d}>
                  {d}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>Estado</span>
            <select
              value={statusFilter}
              onChange={(e) =>
                setStatusFilter((e.target.value || '') as '' | 'success' | 'error' | 'blocked')
              }
              className="run-filter-select"
            >
              <option value="">Todos</option>
              {STATUS_OPTIONS.filter(Boolean).map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>Desde</span>
            <input
              type="datetime-local"
              value={fromFilter}
              onChange={(e) => setFromFilter(e.target.value)}
              className="run-filter-input"
            />
          </label>
          <label>
            <span>Hasta</span>
            <input
              type="datetime-local"
              value={toFilter}
              onChange={(e) => setToFilter(e.target.value)}
              className="run-filter-input"
            />
          </label>
          <label>
            <span>Límite</span>
            <select
              value={limitFilter}
              onChange={(e) => setLimitFilter(Number(e.target.value))}
              className="run-filter-select"
            >
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={200}>200</option>
              <option value={300}>300</option>
              <option value={500}>500</option>
            </select>
          </label>
        </div>

        <QueryError error={query.error} onRetry={() => query.refetch()} />

        {query.isLoading && <p className="muted">Cargando eventos...</p>}

        {!query.isLoading && items.length > 0 && (
          <>
            <div className="run-explorer-summary">
              <h3>Resumen por tool</h3>
              <div style={{ width: '100%', height: 220 }}>
                {isTestMode ? (
                  <BarChart width={640} height={220} data={chartData} layout="vertical" margin={{ left: 80 }}>
                    <XAxis type="number" />
                    <YAxis type="category" dataKey="tool" width={75} />
                    <Tooltip />
                    <Bar dataKey="allow" stackId="a" fill="#5a9e7a" radius={[0, 0, 0, 0]} />
                    <Bar dataKey="deny" stackId="a" fill="#8b6b9e" radius={[0, 0, 0, 0]} />
                    <Bar dataKey="error" stackId="a" fill="#c45c4a" radius={[0, 0, 0, 0]} />
                    <Bar dataKey="blocked" stackId="a" fill="#7a8ba8" radius={[0, 4, 4, 0]} />
                  </BarChart>
                ) : (
                  <ResponsiveContainer width="100%" height={220}>
                    <BarChart data={chartData} layout="vertical" margin={{ left: 80 }}>
                      <XAxis type="number" />
                      <YAxis type="category" dataKey="tool" width={75} />
                      <Tooltip />
                      <Bar dataKey="allow" stackId="a" fill="#5a9e7a" radius={[0, 0, 0, 0]} />
                      <Bar dataKey="deny" stackId="a" fill="#8b6b9e" radius={[0, 0, 0, 0]} />
                      <Bar dataKey="error" stackId="a" fill="#c45c4a" radius={[0, 0, 0, 0]} />
                      <Bar dataKey="blocked" stackId="a" fill="#7a8ba8" radius={[0, 4, 4, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                )}
              </div>
              <div className="run-legend">
                <span className="run-legend-item allow">allow</span>
                <span className="run-legend-item deny">deny</span>
                <span className="run-legend-item error">error</span>
                <span className="run-legend-item blocked">blocked</span>
              </div>
            </div>

            <div className="run-explorer-timeline-wrap">
              <h3>Timeline de eventos</h3>
              <ul className="run-timeline">
                {items.map((ev) => (
                  <li
                    key={ev.request_id}
                    className={`run-timeline-item ${selectedEvent?.request_id === ev.request_id ? 'selected' : ''}`}
                    onClick={() => setSelectedEvent(ev)}
                  >
                    <span className={`run-pill ${decisionPillClass(ev.decision, ev.status)}`}>
                      {ev.decision}
                    </span>
                    <span className="run-tool">{ev.tool_name}</span>
                    <span className="run-time">{formatTime(ev.created_at)}</span>
                    <span className="run-latency">{ev.latency_ms}ms</span>
                  </li>
                ))}
              </ul>
            </div>

            {selectedEvent && (
              <div className="run-detail-panel">
                <h3>Detalle</h3>
                <div className="run-detail-grid">
                  <p>
                    <strong>Request ID</strong> {selectedEvent.request_id}
                  </p>
                  <p>
                    <strong>Tool</strong> {selectedEvent.tool_name}
                  </p>
                  <p>
                    <strong>Decisión</strong>{' '}
                    <span className={`run-pill ${decisionPillClass(selectedEvent.decision, selectedEvent.status)}`}>
                      {selectedEvent.decision}
                    </span>
                  </p>
                  <p>
                    <strong>Estado</strong> {selectedEvent.status}
                  </p>
                  <p>
                    <strong>Latencia</strong> {selectedEvent.latency_ms}ms
                  </p>
                  {selectedEvent.reason && (
                    <p>
                      <strong>Razón</strong> {selectedEvent.reason}
                    </p>
                  )}
                  {selectedEvent.error && (
                    <p>
                      <strong>Error</strong> {selectedEvent.error.code}: {selectedEvent.error.message}
                    </p>
                  )}
                </div>
                {(selectedEvent.input || selectedEvent.output || selectedEvent.context) && (
                  <div className="run-detail-payload">
                    {selectedEvent.input && (
                      <div>
                        <strong>Input</strong>
                        <pre>{JSON.stringify(selectedEvent.input, null, 2)}</pre>
                      </div>
                    )}
                    {selectedEvent.context && (
                      <div>
                        <strong>Context</strong>
                        <pre>{JSON.stringify(selectedEvent.context, null, 2)}</pre>
                      </div>
                    )}
                    {selectedEvent.output && (
                      <div>
                        <strong>Output</strong>
                        <pre>{JSON.stringify(selectedEvent.output, null, 2)}</pre>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </>
        )}

        {!query.isLoading && items.length === 0 && !query.error && (
          <p className="muted">No hay eventos de auditoría con los filtros actuales.</p>
        )}
      </Card>
    </div>
  );
}
