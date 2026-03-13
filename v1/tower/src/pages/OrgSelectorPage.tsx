import { OrganizationSwitcher } from '@clerk/clerk-react';

import { clerkEnabled } from '../lib/auth';

export default function OrgSelectorPage() {
  if (!clerkEnabled) {
    return (
      <div className="panel-page">
        <h2>Organization Selector</h2>
        <p className="muted">Clerk is disabled in this environment.</p>
      </div>
    );
  }
  return (
    <div className="panel-page">
      <h2>Active Organization</h2>
      <OrganizationSwitcher
        afterCreateOrganizationUrl="/org-selector"
        afterLeaveOrganizationUrl="/org-selector"
        afterSelectOrganizationUrl="/org-selector"
        hidePersonal={false}
      />
    </div>
  );
}

