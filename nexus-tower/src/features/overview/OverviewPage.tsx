import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

import { Card } from '../../components/Card';
import { QueryError } from '../../components/QueryError';
import { getActions, getEvents, getIncidents } from '../../lib/api';

export function OverviewPage() {
  const eventsQ = useQuery({ queryKey: ['events'], queryFn: () => getEvents(0, 200), refetchInterval: 8000 });
  const actionsQ = useQuery({ queryKey: ['actions'], queryFn: getActions, refetchInterval: 8000 });
  const incidentsQ = useQuery({ queryKey: ['incidents'], queryFn: getIncidents, refetchInterval: 8000 });

  const chartData = useMemo(() => {
    const counts = new Map<string, number>();
    (eventsQ.data?.items || []).forEach((event) => {
      counts.set(event.event_type, (counts.get(event.event_type) || 0) + 1);
    });
    return Array.from(counts.entries()).map(([event_type, count]) => ({ event_type, count }));
  }, [eventsQ.data]);

  const openIncidents = (incidentsQ.data?.items || []).filter((item) => item.status === 'open').length;
  const activeActions = (actionsQ.data?.items || []).filter((item) => item.status === 'active').length;
  const isTestMode = import.meta.env.MODE === 'test';

  return (
    <div className="grid two">
      <Card title="Control Status">
        <QueryError error={eventsQ.error} onRetry={() => eventsQ.refetch()} />
        <div className="stat-grid">
          <div>
            <p className="stat-label">Events</p>
            <strong>{eventsQ.data?.items.length || 0}</strong>
          </div>
          <div>
            <p className="stat-label">Active Actions</p>
            <strong>{activeActions}</strong>
          </div>
          <div>
            <p className="stat-label">Open Incidents</p>
            <strong>{openIncidents}</strong>
          </div>
        </div>
      </Card>

      <Card title="Event Mix">
        <QueryError error={eventsQ.error} onRetry={() => eventsQ.refetch()} />
        <div style={{ width: '100%', height: 260 }}>
          {isTestMode ? (
            <BarChart width={640} height={260} data={chartData}>
              <XAxis dataKey="event_type" hide />
              <YAxis allowDecimals={false} />
              <Tooltip />
              <Bar dataKey="count" fill="#ff5f2a" radius={[4, 4, 0, 0]} />
            </BarChart>
          ) : (
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={chartData}>
                <XAxis dataKey="event_type" hide />
                <YAxis allowDecimals={false} />
                <Tooltip />
                <Bar dataKey="count" fill="#ff5f2a" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </Card>

      <Card title="Latest Incidents">
        <QueryError error={incidentsQ.error} onRetry={() => incidentsQ.refetch()} />
        <table className="table">
          <thead>
            <tr>
              <th>Severity</th>
              <th>Title</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {(incidentsQ.data?.items || []).slice(0, 8).map((incident) => (
              <tr key={incident.id}>
                <td>{incident.severity}</td>
                <td>{incident.title}</td>
                <td>{incident.status}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>

      <Card title="Latest Actions">
        <QueryError error={actionsQ.error} onRetry={() => actionsQ.refetch()} />
        <table className="table">
          <thead>
            <tr>
              <th>Type</th>
              <th>Scope</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {(actionsQ.data?.items || []).slice(0, 8).map((action) => (
              <tr key={action.id}>
                <td>{action.action_type}</td>
                <td>{action.scope_type}</td>
                <td>{action.status}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}
