import { Link, useLocation } from 'react-router-dom';

const navItems = [
  { to: '/', label: 'Overview' },
  { to: '/viewer', label: 'Viewer' },
  { to: '/run-explorer', label: 'Run Explorer' },
  { to: '/timeline', label: 'Timeline' },
  { to: '/policies', label: 'Policies' },
  { to: '/approvals', label: 'Approvals' },
  { to: '/alerts', label: 'Alerts' },
  { to: '/sessions', label: 'Sessions' },
  { to: '/ask-agent', label: 'Ask Agent' },
  { to: '/exports', label: 'Exports' },
];

export function Shell({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const shellClass = 'shell';
  return (
    <div className={shellClass}>
      <header className="shell-header">
        <div>
          <p className="eyebrow">Nexus Tower</p>
          <h1>Agent-Operated Supervision</h1>
        </div>
        <p className="header-note">Human review, deterministic backend, auditable actions.</p>
      </header>

      <nav className="shell-nav">
        {navItems.map((item) => (
          <Link key={item.to} to={item.to} className={location.pathname === item.to ? 'active' : ''}>
            {item.label}
          </Link>
        ))}
      </nav>

      <main className="shell-main">{children}</main>
    </div>
  );
}
