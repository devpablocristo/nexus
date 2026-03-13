import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { createOrgAPIKey, getOrgAPIKeys, getUserMe, revokeOrgAPIKey, rotateOrgAPIKey } from '../lib/api';

function parseScopes(raw: string): string[] {
  return raw
    .split(',')
    .map((scope) => scope.trim())
    .filter(Boolean);
}

export default function APIKeysPage() {
  const qc = useQueryClient();
  const meQuery = useQuery({ queryKey: ['users', 'me'], queryFn: getUserMe });
  const orgID = meQuery.data?.org_id;

  const keysQuery = useQuery({
    queryKey: ['org-api-keys', orgID],
    queryFn: () => getOrgAPIKeys(orgID!),
    enabled: Boolean(orgID),
  });

  const [name, setName] = useState('tower-key');
  const [scopesRaw, setScopesRaw] = useState('admin:console:read,admin:console:write,audit:read');
  const [latestRawKey, setLatestRawKey] = useState('');

  const createMut = useMutation({
    mutationFn: () =>
      createOrgAPIKey(orgID!, {
        name: name.trim(),
        scopes: parseScopes(scopesRaw),
      }),
    onSuccess: (out) => {
      setLatestRawKey(out.api_key);
      qc.invalidateQueries({ queryKey: ['org-api-keys', orgID] });
    },
  });

  const rotateMut = useMutation({
    mutationFn: (keyID: string) => rotateOrgAPIKey(orgID!, keyID),
    onSuccess: (out) => {
      setLatestRawKey(out.api_key);
      qc.invalidateQueries({ queryKey: ['org-api-keys', orgID] });
    },
  });

  const revokeMut = useMutation({
    mutationFn: (keyID: string) => revokeOrgAPIKey(orgID!, keyID),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['org-api-keys', orgID] }),
  });

  const canCreate = useMemo(() => Boolean(orgID && name.trim()), [orgID, name]);

  return (
    <div className="panel-page">
      <h2>API Keys</h2>
      <p className="muted">Manage machine-to-machine keys per organization.</p>

      {!orgID && <p className="muted">Loading org context...</p>}

      {orgID && (
        <>
          <div className="inline-form">
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Key name" />
            <input
              value={scopesRaw}
              onChange={(e) => setScopesRaw(e.target.value)}
              placeholder="Scopes CSV"
            />
            <button onClick={() => createMut.mutate()} disabled={!canCreate || createMut.isPending}>
              {createMut.isPending ? 'Creating...' : 'Create key'}
            </button>
          </div>

          {latestRawKey && (
            <div className="secret-reveal">
              <strong>New key (show once):</strong>
              <code>{latestRawKey}</code>
            </div>
          )}

          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Scopes</th>
                <th>Created</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {(keysQuery.data?.items ?? []).map((key) => (
                <tr key={key.id}>
                  <td>{key.name}</td>
                  <td>{key.scopes.join(', ')}</td>
                  <td>{new Date(key.created_at).toLocaleString()}</td>
                  <td className="table-actions">
                    <button className="btn-secondary-sm" onClick={() => rotateMut.mutate(key.id)} disabled={rotateMut.isPending}>
                      Rotate
                    </button>
                    <button className="btn-danger-sm" onClick={() => revokeMut.mutate(key.id)} disabled={revokeMut.isPending}>
                      Revoke
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  );
}

