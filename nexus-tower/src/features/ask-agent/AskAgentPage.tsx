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
          <button type="submit">Query Operator</button>
          <button type="button" className="ghost" onClick={() => tick.mutate()}>
            Trigger Tick
          </button>
        </div>
      </form>

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
