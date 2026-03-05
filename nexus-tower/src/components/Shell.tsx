import { Link, useLocation } from 'react-router-dom';
import { UserButton } from '@clerk/clerk-react';
import { useActiveTool } from '../lib/tool-context';
import { clerkEnabled } from '../lib/auth';

const navItems = [
  { to: '/tools', label: 'Tools' },
  { to: '/audit', label: 'Audit Log' },
  { to: '/monitoring', label: 'Monitoring' },
  { to: '/secrets', label: 'Secrets' },
  { to: '/policies', label: 'Policies' },
  { to: '/incidents', label: 'Incidents' },
  { to: '/events', label: 'Events' },
  { to: '/assistant', label: 'Assistant' },
  { to: '/settings/keys', label: 'API Keys' },
  { to: '/admin', label: 'Admin' },
  { to: '/billing', label: 'Billing' },
  { to: '/org-selector', label: 'Organizations' },
  { to: '/settings', label: 'Profile' },
];

export function Shell({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { activeTool } = useActiveTool();

  return (
    <div className="shell">
      <header className="shell-header">
        <div>
          <p className="eyebrow">Nexus Tower</p>
          <h1>Control Panel</h1>
        </div>
        <div className="header-actions">
          <p className="header-note">Manage tools, inspect requests, and monitor Nexus.</p>
          {clerkEnabled && (
            <div className="user-controls">
              <UserButton afterSignOutUrl="/login" />
            </div>
          )}
        </div>
      </header>

      <nav className="shell-nav">
        {navItems.map((item) => (
          <Link key={item.to} to={item.to} className={location.pathname.startsWith(item.to) ? 'active' : ''}>
            {item.label}
          </Link>
        ))}
      </nav>

      {activeTool && (
        <div className="active-tool-banner">
          <span className="active-tool-label">Monitoring</span>
          <code className="active-tool-name">{activeTool}</code>
        </div>
      )}

      <main className="shell-main">{children}</main>
    </div>
  );
}
