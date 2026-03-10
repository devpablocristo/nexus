import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';

import { askAssistant } from '../lib/api';

export default function AssistantPage() {
  const [query, setQuery] = useState('Show the last security events and open incidents');
  const mut = useMutation({ mutationFn: (q: string) => askAssistant(q) });

  return (
    <div className="panel-page">
      <h2>Assistant</h2>
      <p className="muted">Query AI operators for posture and recommendations.</p>

      <div className="inline-form">
        <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Ask about incidents, policies, risks..." />
        <button onClick={() => mut.mutate(query)} disabled={!query.trim() || mut.isPending}>
          {mut.isPending ? 'Asking...' : 'Ask'}
        </button>
      </div>

      {mut.data && (
        <div className="assistant-response">
          <h3>Summary</h3>
          <p>{mut.data.summary}</p>

          {(mut.data.tables ?? []).map((table) => (
            <div key={table.title}>
              <h4>{table.title}</h4>
              <table className="table">
                <thead>
                  <tr>
                    {table.columns.map((col) => (
                      <th key={col}>{col}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {table.rows.map((row, idx) => (
                    <tr key={idx}>
                      {table.columns.map((col) => (
                        <td key={col}>{row[col]}</td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}

          {(mut.data.actions ?? []).length > 0 && (
            <div>
              <h4>Suggested Actions</h4>
              <ul>
                {(mut.data.actions ?? []).map((action, idx) => (
                  <li key={idx}>
                    <strong>{action.label}</strong> <span className="muted">({action.action_type})</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}

      {mut.error && <p className="field-error">{(mut.error as Error).message}</p>}
    </div>
  );
}

