import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { Card } from '../../components/Card';
import { approveProposal, getPolicyProposals, rejectProposal, shadowProposal } from '../../lib/api';

export function PoliciesPage() {
  const qc = useQueryClient();
  const query = useQuery({ queryKey: ['proposals'], queryFn: getPolicyProposals, refetchInterval: 8000 });

  const mutate = useMutation({
    mutationFn: async ({ id, action }: { id: string; action: 'approve' | 'reject' | 'shadow' }) => {
      if (action === 'approve') return approveProposal(id);
      if (action === 'reject') return rejectProposal(id);
      return shadowProposal(id);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['proposals'] }),
  });

  return (
    <Card title="Policy Proposals">
      <div className="proposal-list">
        {(query.data?.items || []).map((proposal) => (
          <article key={proposal.id} className="proposal-item">
            <header>
              <p>{proposal.status}</p>
              <small>{proposal.created_at}</small>
            </header>
            <p>{proposal.rationale}</p>
            <pre>{JSON.stringify(proposal.diff, null, 2)}</pre>
            <div className="button-row">
              <button onClick={() => mutate.mutate({ id: proposal.id, action: 'approve' })}>Approve</button>
              <button onClick={() => mutate.mutate({ id: proposal.id, action: 'shadow' })}>Shadow</button>
              <button className="ghost" onClick={() => mutate.mutate({ id: proposal.id, action: 'reject' })}>
                Reject
              </button>
            </div>
          </article>
        ))}
      </div>
    </Card>
  );
}
