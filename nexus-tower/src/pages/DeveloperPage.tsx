const coreBaseURL = import.meta.env.VITE_NEXUS_CORE_URL || 'http://localhost:8080';
const saasBaseURL = import.meta.env.VITE_NEXUS_SAAS_URL || 'http://localhost:8082';

export default function DeveloperPage() {
  const downloads = [
    {
      label: 'Postman Collection (Core)',
      href: '/downloads/nexus-core.postman_collection.json',
      text: 'Download .json',
    },
    {
      label: 'Postman Collection (SaaS)',
      href: '/downloads/nexus-saas.postman_collection.json',
      text: 'Download .json',
    },
    {
      label: 'OpenAPI Spec (Core)',
      href: `${coreBaseURL}/openapi.yaml`,
      text: 'Download .yaml',
    },
    {
      label: 'OpenAPI Spec (SaaS)',
      href: `${saasBaseURL}/openapi.yaml`,
      text: 'Download .yaml',
    },
  ];

  return (
    <section className="panel-page developer-page">
      <header className="developer-header">
        <h2>Developer Portal</h2>
        <p className="muted">Everything needed to integrate with Nexus quickly.</p>
      </header>

      <article className="developer-section">
        <h3>Getting Started</h3>
        <ol className="developer-steps">
          <li>Create an API key in Settings &rarr; API Keys.</li>
          <li>Register your first tool via <code>POST /v1/tools</code>.</li>
          <li>Run your first request via <code>POST /v1/run</code>.</li>
        </ol>
        <a className="developer-link" href="/settings/keys">
          View full guide &rarr;
        </a>
      </article>

      <article className="developer-section">
        <h3>API Reference</h3>
        <div className="developer-cards">
          <div className="developer-card">
            <h4>Nexus Core API</h4>
            <p className="muted">Gateway, tools, policies, audit, approvals.</p>
            <a className="developer-link" href={`${coreBaseURL}/docs`} target="_blank" rel="noreferrer">
              Open Docs &rarr;
            </a>
          </div>
          <div className="developer-card">
            <h4>Nexus SaaS API</h4>
            <p className="muted">Billing, admin, incidents, users, notifications.</p>
            <a className="developer-link" href={`${saasBaseURL}/docs`} target="_blank" rel="noreferrer">
              Open Docs &rarr;
            </a>
          </div>
        </div>
      </article>

      <article className="developer-section">
        <h3>SDKs</h3>
        <div className="developer-cards">
          <div className="developer-card">
            <h4>Python SDK</h4>
            <p className="muted">
              <code>pip install nexus-sdk</code>
            </p>
            <pre className="developer-code">
{`from nexus_sdk import NexusClient

client = NexusClient(base_url="...", api_key="...")
result = client.run(tool="my-tool", payload={"prompt": "hello"})
print(result)`}
            </pre>
          </div>
          <div className="developer-card">
            <h4>TypeScript SDK</h4>
            <p className="muted">
              <code>npm install @nexus/sdk</code>
            </p>
            <pre className="developer-code">
{`import { NexusClient } from '@nexus/sdk';

const client = new NexusClient({ baseUrl: '...', apiKey: '...' });
const result = await client.run('my-tool', { prompt: 'hello' });
console.log(result);`}
            </pre>
          </div>
        </div>
      </article>

      <article className="developer-section">
        <h3>Quick Reference</h3>
        <div className="developer-reference">
          <p>
            <strong>Core API:</strong> <code>{coreBaseURL}</code>
          </p>
          <p>
            <strong>SaaS API:</strong> <code>{saasBaseURL}</code>
          </p>
          <p>
            <strong>Auth header:</strong> <code>X-NEXUS-CORE-KEY: &lt;your-api-key&gt;</code>
          </p>
          <p>
            <strong>JWT alternative:</strong> <code>Authorization: Bearer &lt;jwt&gt;</code>
          </p>
          <p>
            <strong>Key endpoints:</strong> <code>POST /v1/run</code>, <code>GET /v1/tools</code>, <code>POST /v1/tools</code>,{' '}
            <code>GET /v1/audit</code>
          </p>
        </div>
      </article>

      <article className="developer-section">
        <h3>Downloads</h3>
        <div className="developer-downloads">
          {downloads.map((item) => (
            <div key={item.label} className="developer-download-item">
              <span>{item.label}</span>
              <a className="developer-link" href={item.href} target="_blank" rel="noreferrer">
                {item.text}
              </a>
            </div>
          ))}
        </div>
      </article>
    </section>
  );
}
