import { useQuery } from '@tanstack/react-query';

import { Card } from '../../components/Card';
import { QueryError } from '../../components/QueryError';
import { getEvents } from '../../lib/api';

export function TimelinePage() {
  const query = useQuery({ queryKey: ['timeline'], queryFn: () => getEvents(0, 200), refetchInterval: 8000 });

  return (
    <Card title="Operational Timeline">
      <QueryError error={query.error} onRetry={() => query.refetch()} />
      {query.isLoading && <p className="muted">Loading events...</p>}
      <ul className="timeline">
        {(query.data?.items || []).map((event) => (
          <li key={event.id}>
            <p>
              <strong>{event.event_type}</strong> <span>{event.created_at}</span>
            </p>
            <pre>{JSON.stringify(event.payload, null, 2)}</pre>
          </li>
        ))}
      </ul>
    </Card>
  );
}
