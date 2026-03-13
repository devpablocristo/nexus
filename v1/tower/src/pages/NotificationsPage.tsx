import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';

import type { InAppNotification } from '../lib/types';
import { getInAppNotifications, markInAppNotificationRead } from '../lib/api';

function destinationFor(type: string): string {
  switch (type) {
    case 'incident_opened':
    case 'incident_closed':
      return '/incidents';
    case 'plan_upgraded':
    case 'payment_failed':
    case 'subscription_canceled':
    case 'tenant_suspended':
    case 'tenant_reactivated':
    case 'usage_warning_80':
    case 'usage_warning_95':
    case 'usage_limit_reached':
      return '/billing';
    case 'welcome':
      return '/tools';
    default:
      return '/notifications';
  }
}

export default function NotificationsPage() {
  const [offset, setOffset] = useState(0);
  const limit = 25;
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const listQuery = useQuery({
    queryKey: ['notifications', 'page', limit, offset],
    queryFn: () => getInAppNotifications(limit, offset),
    refetchInterval: 30_000,
  });

  const markReadMutation = useMutation({
    mutationFn: (id: string) => markInAppNotificationRead(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
  });

  async function openNotification(item: InAppNotification) {
    if (!item.read_at) {
      await markReadMutation.mutateAsync(item.id);
    }
    navigate(destinationFor(item.type));
  }

  const items = listQuery.data?.items ?? [];

  return (
    <section className="panel-page notifications-page">
      <header className="notifications-page-header">
        <h2>Notifications</h2>
        <p className="muted">Recent in-app notifications across billing, incidents and admin actions.</p>
      </header>

      {listQuery.isLoading && <p className="muted">Loading notifications...</p>}
      {listQuery.error && <p className="field-error">{(listQuery.error as Error).message}</p>}

      {!listQuery.isLoading && !listQuery.error && (
        <>
          <div className="notifications-feed">
            {items.map((item) => (
              <article key={item.id} className={`notifications-feed-item ${item.read_at ? 'read' : 'unread'}`}>
                <div className="notifications-feed-main">
                  <p className="notifications-feed-title">{item.title}</p>
                  <p className="notifications-feed-body">{item.body}</p>
                  <p className="muted">{new Date(item.created_at).toLocaleString()}</p>
                </div>
                <div className="notifications-feed-actions">
                  {!item.read_at && (
                    <button onClick={() => markReadMutation.mutate(item.id)} disabled={markReadMutation.isPending}>
                      Mark as read
                    </button>
                  )}
                  <button onClick={() => void openNotification(item)}>Open</button>
                </div>
              </article>
            ))}
            {items.length === 0 && <p className="muted">No notifications found.</p>}
          </div>

          <div className="notifications-pagination">
            <button onClick={() => setOffset((prev) => Math.max(0, prev - limit))} disabled={offset === 0}>
              Previous
            </button>
            <span className="muted">
              Showing {offset + 1} to {offset + items.length}
            </span>
            <button onClick={() => setOffset((prev) => prev + limit)} disabled={items.length < limit}>
              Next
            </button>
          </div>
        </>
      )}
    </section>
  );
}
