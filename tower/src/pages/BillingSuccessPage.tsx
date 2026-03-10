import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

export default function BillingSuccessPage() {
  const navigate = useNavigate();

  useEffect(() => {
    const timer = window.setTimeout(() => {
      navigate('/billing?ok=1', { replace: true });
    }, 200);
    return () => window.clearTimeout(timer);
  }, [navigate]);

  return (
    <div className="panel-page">
      <h2>Processing Upgrade</h2>
      <p className="muted">Redirecting to billing dashboard...</p>
    </div>
  );
}
