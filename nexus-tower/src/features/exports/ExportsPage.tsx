import { Card } from '../../components/Card';

const apiBase =
  import.meta.env.VITE_NEXUS_SAAS_URL || import.meta.env.VITE_NEXUS_CORE_URL || 'http://localhost:8082';

export function ExportsPage() {
  return (
    <Card title="Exports & Compliance">
      <p>
        Export append-only audit evidence through the Nexus SaaS control API. Use a browser with API key headers
        plugin or curl for authenticated access.
      </p>
      <ul className="exports-list">
        <li>
          <a href={`${apiBase}/v1/audit/export?format=jsonl`} target="_blank" rel="noreferrer">
            Audit Export JSONL
          </a>
        </li>
        <li>
          <a href={`${apiBase}/v1/audit/export?format=csv`} target="_blank" rel="noreferrer">
            Audit Export CSV
          </a>
        </li>
        <li>
          <a href={`${apiBase}/openapi.yaml`} target="_blank" rel="noreferrer">
            Control API OpenAPI
          </a>
        </li>
      </ul>
    </Card>
  );
}
