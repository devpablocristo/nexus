import { Link, useLocation } from 'react-router-dom';
import { useActiveTool } from '../lib/tool-context';

const navItems = [
  { to: '/tools', label: 'Tools' },
  { to: '/audit', label: 'Audit Log' },
  { to: '/monitoring', label: 'Monitoring' },
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
        <p className="header-note">Manage tools, inspect requests, and monitor Nexus.</p>
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
