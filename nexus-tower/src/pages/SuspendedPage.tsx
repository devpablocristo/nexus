import { Link } from 'react-router-dom';

export default function SuspendedPage() {
  return (
    <div className="panel-page suspended-page">
      <h2>Your account has been suspended</h2>
      <p className="muted">
        Access to core API actions is temporarily disabled for this tenant. Contact support or resolve billing to
        reactivate the account.
      </p>
      <div className="suspended-actions">
        <a className="btn-secondary" href="mailto:support@nexus.io?subject=Tenant%20suspension%20support">
          Contact support
        </a>
        <Link className="btn-secondary" to="/billing">
          Open billing
        </Link>
      </div>
    </div>
  );
}
