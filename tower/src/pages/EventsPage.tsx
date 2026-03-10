import { useQuery } from '@tanstack/react-query';

import { getEvents } from '../lib/api';

export default function EventsPage() {
  const query = useQuery({
    queryKey: ['events'],
    queryFn: () => getEvents(200),
    refetchInterval: 10000,
  });

  return (
    <div className="panel-page">
      <h2>Events</h2>
      <p className="muted">Event stream emitted by the SaaS control plane.</p>

      {query.isLoading && <p className="muted">Loading events...</p>}

      <table className="table">
        <thead>
          <tr>
            <th>Time</th>
            <th>Type</th>
            <th>Payload</th>
          </tr>
        </thead>
        <tbody>
          {(query.data?.items ?? []).map((event) => (
            <tr key={event.id}>
              <td>{new Date(event.created_at).toLocaleString()}</td>
              <td>{event.event_type}</td>
              <td>
                <code>{JSON.stringify(event.payload)}</code>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

