from __future__ import annotations

from app.domain.models import Signal


def throttle_tenant_playbook(signal: Signal, ttl_seconds: int) -> dict[str, object]:
    return {
        'scope_type': 'tenant',
        'action_type': 'throttle_tenant_rpm',
        'params': {'per_minute': 10},
        'ttl_seconds': ttl_seconds,
        'evidence_refs': signal.evidence_refs,
    }


def incident_from_signal(signal: Signal, severity: str) -> dict[str, object]:
    return {
        'severity': severity,
        'title': 'Elevated deny ratio detected by operator',
        'summary': (
            f'Deny ratio={signal.deny_ratio:.2f}, denied={signal.denied_events}, '
            f'total={signal.total_events}. Temporary throttle action applied.'
        ),
        'evidence_refs': signal.evidence_refs,
    }


def proposal_from_signal(signal: Signal) -> dict[str, object]:
    return {
        'status': 'pending',
        'diff': {
            'suggested_policy': {
                'effect': 'deny',
                'conditions': {'path': 'context.risk.signal', 'op': 'eq', 'value': 'high'},
                'reason_template': 'Operator suggested temporary deny under high risk signal'
            }
        },
        'rationale': (
            'Deterministic operator signal observed high deny ratio. '
            'Proposal created for human review; no automatic permanent enforcement.'
        ),
        'tests_suggested': [
            'deny_ratio_spike_blocks_sensitive_writes',
            'normal_traffic_keeps_allow_path'
        ],
        'rollback_plan': 'Reject proposal or keep in shadow mode if false positives occur.',
    }
