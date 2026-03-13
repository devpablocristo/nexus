import { Fragment, useState } from 'react';
import { useQuery } from '@tanstack/react-query';

import { getAdminActivity, getAdminBootstrap } from '../lib/api';

export default function AdminActivityPage() {
  const [limit, setLimit] = useState(50);
  const [expandedRowID, setExpandedRowID] = useState<string | null>(null);

  const bootstrapQuery = useQuery({
    queryKey: ['admin-bootstrap'],
    queryFn: getAdminBootstrap,
  });

  const canReadAdmin = bootstrapQuery.data?.can_read_admin ?? false;

  const activityQuery = useQuery({
    queryKey: ['admin-activity', limit],
    queryFn: () => getAdminActivity(limit),
    enabled: canReadAdmin,
  });

  if (bootstrapQuery.isLoading) {
    return (
      <div className="panel-page admin-activity-page">
        <h2>Admin Activity Log</h2>
        <p className="muted">Loading admin permissions...</p>
      </div>
    );
  }

  if (bootstrapQuery.error) {
    return (
      <div className="panel-page admin-activity-page">
        <h2>Admin Activity Log</h2>
        {isForbidden(bootstrapQuery.error) ? (
          <p className="muted">You don&apos;t have permission to view admin settings.</p>
        ) : (
          <p className="field-error">{(bootstrapQuery.error as Error).message}</p>
        )}
      </div>
    );
  }

  if (!bootstrapQuery.data || !canReadAdmin) {
    return (
      <div className="panel-page admin-activity-page">
        <h2>Admin Activity Log</h2>
        <p className="muted">You don&apos;t have permission to view admin settings.</p>
      </div>
    );
  }

  const items = activityQuery.data?.items ?? [];
  const canLoadMore = limit < 200 && items.length >= limit;

  return (
    <div className="panel-page admin-activity-page">
      <h2>Admin Activity Log</h2>
      <p className="muted">Track all administrative changes in your organization.</p>

      {activityQuery.isLoading && <p className="muted">Loading activity...</p>}
      {activityQuery.error && <p className="field-error">{(activityQuery.error as Error).message}</p>}

      <div className="admin-activity-table-wrap">
        <table className="table admin-activity-table">
          <thead>
            <tr>
              <th>When</th>
              <th>Actor</th>
              <th>Action</th>
              <th>Resource</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => {
              const expanded = expandedRowID === item.id;
              return (
                <Fragment key={item.id}>
                  <tr
                    className={expanded ? 'admin-activity-row expanded' : 'admin-activity-row'}
                    onClick={() => setExpandedRowID(expanded ? null : item.id)}
                  >
                    <td>{formatDateTime(item.created_at)}</td>
                    <td>{item.actor || 'system'}</td>
                    <td>{item.action}</td>
                    <td>{item.resource_type}</td>
                  </tr>
                  {expanded && (
                    <tr className="admin-activity-payload-row">
                      <td colSpan={4}>
                        <pre>{JSON.stringify(item.payload, null, 2)}</pre>
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
            {!activityQuery.isLoading && items.length === 0 && (
              <tr>
                <td colSpan={4} className="muted">
                  No activity found.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="admin-activity-footer">
        <span className="muted">
          Showing {items.length} of up to 200 entries (current limit {limit})
        </span>
        {canLoadMore && (
          <button className="btn-secondary" onClick={() => setLimit((prev) => Math.min(200, prev + 50))}>
            Load more
          </button>
        )}
      </div>
    </div>
  );
}

function isForbidden(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  return error.message.includes('API 403');
}

function formatDateTime(v?: string): string {
  if (!v) return '—';
  const date = new Date(v);
  return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;
}
