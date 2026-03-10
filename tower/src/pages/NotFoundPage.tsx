import { Link } from 'react-router-dom';

export default function NotFoundPage() {
  return (
    <div className="panel-page not-found-page">
      <p className="not-found-code">404</p>
      <h2>Page not found</h2>
      <p className="muted">The route you requested does not exist or was moved.</p>
      <Link to="/tools" className="btn-secondary not-found-link">
        Go to Dashboard
      </Link>
    </div>
  );
}
