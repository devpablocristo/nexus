import { Card } from '../../components/Card';

const coreBase = import.meta.env.VITE_NEXUS_CORE_URL || 'http://localhost:8080';

export function ExportsPage() {
  return (
    <Card title="Exports & Compliance">
      <p>
        Export append-only audit evidence from Nexus Core. Use a browser with API key headers plugin or curl for
        authenticated access.
      </p>
      <ul className="exports-list">
        <li>
          <a href={`${coreBase}/v1/audit/export?format=jsonl`} target="_blank" rel="noreferrer">
            Audit Export JSONL
          </a>
        </li>
        <li>
          <a href={`${coreBase}/v1/audit/export?format=csv`} target="_blank" rel="noreferrer">
            Audit Export CSV
          </a>
        </li>
        <li>
          <a href={`${coreBase}/openapi.yaml`} target="_blank" rel="noreferrer">
            Nexus Core OpenAPI
          </a>
        </li>
      </ul>
    </Card>
  );
}
