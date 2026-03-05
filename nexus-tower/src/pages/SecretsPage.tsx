import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { deleteToolSecret, getTools, listToolSecrets, upsertToolSecret } from '../lib/api';

export default function SecretsPage() {
  const qc = useQueryClient();
  const toolsQuery = useQuery({ queryKey: ['tools'], queryFn: getTools });
  const [toolName, setToolName] = useState('');
  const [keyName, setKeyName] = useState('');
  const [secretType, setSecretType] = useState('api_key');
  const [value, setValue] = useState('');

  useEffect(() => {
    if (!toolName && toolsQuery.data?.items?.[0]?.name) {
      setToolName(toolsQuery.data.items[0].name);
    }
  }, [toolName, toolsQuery.data]);

  const secretsQuery = useQuery({
    queryKey: ['secrets', toolName],
    queryFn: () => listToolSecrets(toolName),
    enabled: Boolean(toolName),
  });

  const upsertMut = useMutation({
    mutationFn: () =>
      upsertToolSecret(toolName, {
        secret_type: secretType,
        key_name: keyName.trim(),
        value: value.trim(),
        enabled: true,
      }),
    onSuccess: () => {
      setValue('');
      qc.invalidateQueries({ queryKey: ['secrets', toolName] });
    },
  });

  const deleteMut = useMutation({
    mutationFn: (name: string) => deleteToolSecret(toolName, name),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['secrets', toolName] }),
  });

  return (
    <div className="panel-page">
      <h2>Secrets</h2>
      <p className="muted">Store per-tool credentials for gateway runtime substitution.</p>

      <div className="inline-form">
        <select value={toolName} onChange={(e) => setToolName(e.target.value)}>
          {(toolsQuery.data?.items ?? []).map((tool) => (
            <option key={tool.id} value={tool.name}>
              {tool.name}
            </option>
          ))}
        </select>
        <input value={keyName} onChange={(e) => setKeyName(e.target.value)} placeholder="Key name" />
        <select value={secretType} onChange={(e) => setSecretType(e.target.value)}>
          <option value="api_key">api_key</option>
          <option value="bearer_token">bearer_token</option>
          <option value="password">password</option>
        </select>
        <input value={value} onChange={(e) => setValue(e.target.value)} placeholder="Secret value" />
        <button onClick={() => upsertMut.mutate()} disabled={!toolName || !keyName.trim() || !value.trim() || upsertMut.isPending}>
          {upsertMut.isPending ? 'Saving...' : 'Save'}
        </button>
      </div>

      <table className="table">
        <thead>
          <tr>
            <th>Key</th>
            <th>Type</th>
            <th>Enabled</th>
            <th>Updated</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {(secretsQuery.data?.items ?? []).map((secret) => (
            <tr key={secret.id}>
              <td>{secret.key_name}</td>
              <td>{secret.secret_type}</td>
              <td>{secret.enabled ? 'yes' : 'no'}</td>
              <td>{new Date(secret.updated_at).toLocaleString()}</td>
              <td>
                <button className="btn-danger-sm" onClick={() => deleteMut.mutate(secret.key_name)} disabled={deleteMut.isPending}>
                  Delete
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

