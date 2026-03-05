import { OrganizationProfile } from '@clerk/clerk-react';

import { clerkEnabled } from '../lib/auth';

export default function OrganizationSettingsPage() {
  if (!clerkEnabled) {
    return (
      <div className="panel-page">
        <h2>Organization Settings</h2>
        <p className="muted">Clerk is disabled in this environment.</p>
      </div>
    );
  }
  return (
    <div className="panel-page">
      <OrganizationProfile path="/settings/org" routing="path" />
    </div>
  );
}

