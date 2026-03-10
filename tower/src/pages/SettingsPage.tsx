import { UserProfile } from '@clerk/clerk-react';

import { clerkEnabled } from '../lib/auth';

export default function SettingsPage() {
  if (!clerkEnabled) {
    return (
      <div className="panel-page">
        <h2>User Settings</h2>
        <p className="muted">Clerk is disabled in this environment.</p>
      </div>
    );
  }
  return (
    <div className="panel-page">
      <UserProfile path="/settings" routing="path" />
    </div>
  );
}

