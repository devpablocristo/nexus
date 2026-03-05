import { useEffect, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import type { InAppNotification } from '../lib/types';
import {
  getInAppNotificationUnreadCount,
  getInAppNotifications,
  markInAppNotificationRead,
} from '../lib/api';

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

export function NotificationBell() {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const unreadQuery = useQuery({
    queryKey: ['notifications', 'unread-count'],
    queryFn: getInAppNotificationUnreadCount,
    refetchInterval: 30_000,
  });

  const latestQuery = useQuery({
    queryKey: ['notifications', 'latest'],
    queryFn: () => getInAppNotifications(10, 0),
    enabled: open,
    refetchInterval: open ? 30_000 : false,
  });

  const markReadMutation = useMutation({
    mutationFn: (id: string) => markInAppNotificationRead(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['notifications', 'unread-count'] });
      void queryClient.invalidateQueries({ queryKey: ['notifications', 'latest'] });
      void queryClient.invalidateQueries({ queryKey: ['notifications', 'page'] });
    },
  });

  useEffect(() => {
    function onClickOutside(event: MouseEvent) {
      if (!rootRef.current) return;
      const target = event.target;
      if (target instanceof Node && !rootRef.current.contains(target)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', onClickOutside);
    return () => document.removeEventListener('mousedown', onClickOutside);
  }, []);

  async function onNotificationClick(item: InAppNotification) {
    if (!item.read_at) {
      await markReadMutation.mutateAsync(item.id);
    }
    setOpen(false);
    navigate(destinationFor(item.type));
  }

  const unread = unreadQuery.data?.count ?? 0;

  return (
    <div className="notification-bell" ref={rootRef}>
      <button
        type="button"
        className="notification-bell-button"
        onClick={() => setOpen((v) => !v)}
        aria-label="Open notifications"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M12 22a2.4 2.4 0 0 1-2.2-1.5h4.4A2.4 2.4 0 0 1 12 22Zm7.5-3H4.5a1 1 0 0 1-.8-1.6L6 14.1V10a6 6 0 0 1 12 0v4.1l2.3 3.3a1 1 0 0 1-.8 1.6Z" />
        </svg>
        {unread > 0 && <span className="notification-badge">{unread > 99 ? '99+' : unread}</span>}
      </button>

      {open && (
        <div className="notification-dropdown">
          <div className="notification-dropdown-header">
            <strong>Notifications</strong>
            <Link to="/notifications" onClick={() => setOpen(false)}>
              View all
            </Link>
          </div>

          {latestQuery.isLoading && <p className="muted">Loading notifications...</p>}
          {latestQuery.error && <p className="field-error">{(latestQuery.error as Error).message}</p>}

          {!latestQuery.isLoading && !latestQuery.error && (
            <div className="notification-dropdown-list">
              {(latestQuery.data?.items ?? []).map((item) => (
                <button
                  type="button"
                  key={item.id}
                  className={`notification-dropdown-item ${item.read_at ? 'read' : 'unread'}`}
                  onClick={() => void onNotificationClick(item)}
                >
                  <div>
                    <p className="notification-dropdown-title">{item.title}</p>
                    <p className="notification-dropdown-body">{item.body}</p>
                  </div>
                  <span className="notification-dropdown-time">
                    {new Date(item.created_at).toLocaleString()}
                  </span>
                </button>
              ))}
              {(latestQuery.data?.items ?? []).length === 0 && <p className="muted">No notifications yet.</p>}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
