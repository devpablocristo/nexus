import { FormEvent, useState } from 'react';
import { useMutation } from '@tanstack/react-query';

import { Card } from '../../components/Card';
import { operatorTick, queryAssistant } from '../../lib/api';

export function AskAgentPage() {
  const [query, setQuery] = useState('Summarize the latest risk posture.');

  const ask = useMutation({
    mutationFn: (q: string) => queryAssistant(q),
  });

  const tick = useMutation({
    mutationFn: operatorTick,
  });

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    ask.mutate(query);
  };

  return (
    <Card title="Ask Agent">
      <form className="ask-form" onSubmit={onSubmit}>
        <textarea value={query} onChange={(event) => setQuery(event.target.value)} rows={5} />
        <div className="button-row">
          <button type="submit" disabled={ask.isPending}>
            {ask.isPending ? 'Querying...' : 'Query Operator'}
          </button>
          <button type="button" className="ghost" disabled={tick.isPending} onClick={() => tick.mutate()}>
            {tick.isPending ? 'Running...' : 'Trigger Tick'}
          </button>
        </div>
      </form>

      {ask.error && (
        <div className="error-banner">
          <strong>Query failed</strong>
          <p>{ask.error.message}</p>
        </div>
      )}

      {tick.error && (
        <div className="error-banner">
          <strong>Tick failed</strong>
          <p>{tick.error.message}</p>
        </div>
      )}

      {tick.isSuccess && <p className="success-msg">Operator tick completed.</p>}

      {ask.data && (
        <section className="agent-answer">
          <h3>Summary</h3>
          <p>{ask.data.summary}</p>
          {(ask.data.tables || []).map((table) => (
            <div key={table.title}>
              <h4>{table.title}</h4>
              <table className="table">
                <thead>
                  <tr>{table.columns.map((col) => <th key={col}>{col}</th>)}</tr>
                </thead>
                <tbody>
                  {table.rows.map((row, idx) => (
                    <tr key={idx}>
                      {table.columns.map((col) => <td key={col}>{row[col]}</td>)}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}
        </section>
      )}
    </Card>
  );
}
