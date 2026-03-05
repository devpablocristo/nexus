import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';

import { getNotificationPreferences, updateNotificationPreferences } from '../lib/api';

type PreferenceDefinition = {
  key: string;
  title: string;
  description: string;
};

const PREFERENCE_TYPES: PreferenceDefinition[] = [
  { key: 'welcome', title: 'Welcome Email', description: 'Sent when you first join Nexus' },
  { key: 'plan_upgraded', title: 'Plan Upgraded', description: "Sent when your organization's plan changes" },
  { key: 'payment_failed', title: 'Payment Failed', description: 'Sent when a payment attempt fails' },
  { key: 'subscription_canceled', title: 'Subscription Canceled', description: 'Sent when your subscription is canceled' },
  { key: 'incident_opened', title: 'Incident Opened', description: 'Sent when a new incident is detected' },
  { key: 'incident_closed', title: 'Incident Resolved', description: 'Sent when an incident is closed' },
];

export default function NotificationPreferencesPage() {
  const [formValues, setFormValues] = useState<Record<string, boolean>>({});
  const [savedMessage, setSavedMessage] = useState('');

  const preferencesQuery = useQuery({
    queryKey: ['notification-preferences'],
    queryFn: getNotificationPreferences,
  });

  useEffect(() => {
    if (!preferencesQuery.data) return;
    const next: Record<string, boolean> = {};
    for (const pref of preferencesQuery.data.items) {
      next[pref.notification_type] = pref.enabled;
    }
    for (const pref of PREFERENCE_TYPES) {
      if (!(pref.key in next)) {
        next[pref.key] = true;
      }
    }
    setFormValues(next);
  }, [preferencesQuery.data]);

  const mutation = useMutation({
    mutationFn: (items: Array<{ notification_type: string; enabled: boolean }>) => updateNotificationPreferences(items),
    onSuccess: () => {
      setSavedMessage('Preferences saved.');
      preferencesQuery.refetch();
    },
  });

  const initialValues = useMemo(() => {
    if (!preferencesQuery.data) return {} as Record<string, boolean>;
    const out: Record<string, boolean> = {};
    for (const pref of preferencesQuery.data.items) {
      out[pref.notification_type] = pref.enabled;
    }
    for (const pref of PREFERENCE_TYPES) {
      if (!(pref.key in out)) {
        out[pref.key] = true;
      }
    }
    return out;
  }, [preferencesQuery.data]);

  const hasChanges = useMemo(() => {
    for (const pref of PREFERENCE_TYPES) {
      if ((formValues[pref.key] ?? true) !== (initialValues[pref.key] ?? true)) {
        return true;
      }
    }
    return false;
  }, [formValues, initialValues]);

  const onToggle = (notificationType: string, enabled: boolean) => {
    setSavedMessage('');
    setFormValues((prev) => ({ ...prev, [notificationType]: enabled }));
  };

  const onSave = () => {
    const payload = PREFERENCE_TYPES.map((item) => ({
      notification_type: item.key,
      enabled: formValues[item.key] ?? true,
    }));
    mutation.mutate(payload);
  };

  return (
    <div className="panel-page notifications-page">
      <div>
        <h2>Notification Preferences</h2>
        <p className="muted">Choose which emails you want to receive.</p>
      </div>

      {preferencesQuery.isLoading && <p className="muted">Loading preferences...</p>}
      {preferencesQuery.error && <p className="field-error">{(preferencesQuery.error as Error).message}</p>}

      {!preferencesQuery.isLoading && !preferencesQuery.error && (
        <div className="notifications-list">
          {PREFERENCE_TYPES.map((item) => {
            const enabled = formValues[item.key] ?? true;
            return (
              <article className="notification-card" key={item.key}>
                <div>
                  <p className="notification-title">{item.title}</p>
                  <p className="notification-description">{item.description}</p>
                </div>
                <label className="notification-toggle">
                  <input type="checkbox" checked={enabled} onChange={(event) => onToggle(item.key, event.target.checked)} />
                  <span>{enabled ? 'enabled' : 'disabled'}</span>
                </label>
              </article>
            );
          })}
        </div>
      )}

      {mutation.error && <p className="field-error">{(mutation.error as Error).message}</p>}
      {savedMessage && <p className="muted">{savedMessage}</p>}

      <div className="notifications-actions">
        <button onClick={onSave} disabled={!hasChanges || mutation.isPending || preferencesQuery.isLoading}>
          {mutation.isPending ? 'Saving...' : 'Save Preferences'}
        </button>
      </div>
    </div>
  );
}
