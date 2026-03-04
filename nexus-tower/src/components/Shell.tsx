import { Link, useLocation } from 'react-router-dom';

const navItems = [
  { to: '/tools', label: 'Tools' },
  { to: '/audit', label: 'Audit Log' },
  { to: '/monitoring', label: 'Monitoring' },
];

export function Shell({ children }: { children: React.ReactNode }) {
  const location = useLocation();
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

      <main className="shell-main">{children}</main>
    </div>
  );
}
