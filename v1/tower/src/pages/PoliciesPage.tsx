import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { createToolPolicy, getToolPolicies, getTools, updatePolicy } from '../lib/api';

function parseConditions(raw: string): Record<string, unknown> {
  if (!raw.trim()) return {};
  return JSON.parse(raw) as Record<string, unknown>;
}

export default function PoliciesPage() {
  const qc = useQueryClient();
  const toolsQuery = useQuery({ queryKey: ['tools'], queryFn: getTools });
  const [toolName, setToolName] = useState('');
  const [effect, setEffect] = useState<'allow' | 'deny'>('allow');
  const [priority, setPriority] = useState('10');
  const [conditionsRaw, setConditionsRaw] = useState('{}');
  const [error, setError] = useState('');

  useEffect(() => {
    if (!toolName && toolsQuery.data?.items?.[0]?.name) {
      setToolName(toolsQuery.data.items[0].name);
    }
  }, [toolName, toolsQuery.data]);

  const policiesQuery = useQuery({
    queryKey: ['policies', toolName],
    queryFn: () => getToolPolicies(toolName),
    enabled: Boolean(toolName),
  });

  const addMut = useMutation({
    mutationFn: async () => {
      const conditions = parseConditions(conditionsRaw);
      return createToolPolicy(toolName, {
        effect,
        priority: parseInt(priority, 10),
        conditions,
        enabled: true,
      });
    },
    onSuccess: () => {
      setError('');
      qc.invalidateQueries({ queryKey: ['policies', toolName] });
    },
    onError: (err: Error) => setError(err.message),
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => updatePolicy(id, { enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['policies', toolName] }),
  });

  return (
    <div className="panel-page">
      <h2>Policies</h2>
      <p className="muted">Define allow/deny rules evaluated on each tool execution.</p>

      <div className="inline-form">
        <select value={toolName} onChange={(e) => setToolName(e.target.value)}>
          {(toolsQuery.data?.items ?? []).map((tool) => (
            <option key={tool.id} value={tool.name}>
              {tool.name}
            </option>
          ))}
        </select>
        <select value={effect} onChange={(e) => setEffect(e.target.value as 'allow' | 'deny')}>
          <option value="allow">allow</option>
          <option value="deny">deny</option>
        </select>
        <input value={priority} onChange={(e) => setPriority(e.target.value)} placeholder="priority" />
        <input value={conditionsRaw} onChange={(e) => setConditionsRaw(e.target.value)} placeholder='{"user.role":"admin"}' />
        <button onClick={() => addMut.mutate()} disabled={!toolName || addMut.isPending}>
          {addMut.isPending ? 'Adding...' : 'Add policy'}
        </button>
      </div>
      {error && <p className="field-error">{error}</p>}

      <table className="table">
        <thead>
          <tr>
            <th>Effect</th>
            <th>Priority</th>
            <th>Enabled</th>
            <th>Conditions</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {(policiesQuery.data?.items ?? []).map((policy) => (
            <tr key={policy.id}>
              <td>{policy.effect}</td>
              <td>{policy.priority}</td>
              <td>{policy.enabled ? 'yes' : 'no'}</td>
              <td>
                <code>{JSON.stringify(policy.conditions)}</code>
              </td>
              <td>
                <button
                  className="btn-secondary-sm"
                  onClick={() => toggleMut.mutate({ id: policy.id, enabled: !policy.enabled })}
                  disabled={toggleMut.isPending}
                >
                  {policy.enabled ? 'Disable' : 'Enable'}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

