import { useEffect, useState } from 'react';

import type { AdminTenantSettings } from '../lib/types';

type PlanPresetCode = 'starter' | 'growth' | 'enterprise';

const PLAN_DEFAULT_LIMITS: Record<PlanPresetCode, AdminTenantSettings['hard_limits']> = {
  starter: { tools_max: 20, run_rpm: 300, audit_retention_days: 30 },
  growth: { tools_max: 75, run_rpm: 1200, audit_retention_days: 90 },
  enterprise: { tools_max: 250, run_rpm: 5000, audit_retention_days: 365 },
};

type Props = {
  settings: AdminTenantSettings;
  isSaving: boolean;
  error?: string;
  onClose: () => void;
  onSave: (req: {
    plan_code: string;
    hard_limits: { tools_max: number; run_rpm: number; audit_retention_days: number };
  }) => void;
};

export function EditLimitsModal({ settings, isSaving, error, onClose, onSave }: Props) {
  const [planCode, setPlanCode] = useState(settings.plan_code);
  const [toolsMax, setToolsMax] = useState(String(settings.hard_limits.tools_max));
  const [runRPM, setRunRPM] = useState(String(settings.hard_limits.run_rpm));
  const [auditRetentionDays, setAuditRetentionDays] = useState(String(settings.hard_limits.audit_retention_days));
  const [formError, setFormError] = useState('');

  useEffect(() => {
    setPlanCode(settings.plan_code);
    setToolsMax(String(settings.hard_limits.tools_max));
    setRunRPM(String(settings.hard_limits.run_rpm));
    setAuditRetentionDays(String(settings.hard_limits.audit_retention_days));
    setFormError('');
  }, [settings]);

  useEffect(() => {
    const onKeyDown = (evt: KeyboardEvent) => {
      if (evt.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onClose]);

  const handlePlanChange = (nextPlanCode: string) => {
    setPlanCode(nextPlanCode);
    if (isPlanPresetCode(nextPlanCode)) {
      const defaults = PLAN_DEFAULT_LIMITS[nextPlanCode];
      setToolsMax(String(defaults.tools_max));
      setRunRPM(String(defaults.run_rpm));
      setAuditRetentionDays(String(defaults.audit_retention_days));
    }
  };

  const submit = () => {
    setFormError('');
    const parsedToolsMax = parsePositiveInt(toolsMax);
    const parsedRunRPM = parsePositiveInt(runRPM);
    const parsedAuditRetentionDays = parsePositiveInt(auditRetentionDays);
    if (parsedToolsMax === null || parsedRunRPM === null || parsedAuditRetentionDays === null) {
      setFormError('All limits must be positive integers.');
      return;
    }
    onSave({
      plan_code: planCode.trim().toLowerCase(),
      hard_limits: {
        tools_max: parsedToolsMax,
        run_rpm: parsedRunRPM,
        audit_retention_days: parsedAuditRetentionDays,
      },
    });
  };

  return (
    <div className="tool-form-overlay" role="dialog" aria-modal="true" aria-label="Edit Plan Limits">
      <div className="tool-form-card admin-modal-card">
        <div className="tool-form-header">
          <h2>Edit Plan Limits</h2>
          <button className="icon-btn" onClick={onClose} aria-label="Close">
            ✕
          </button>
        </div>

        <p className="tool-form-hint">
          When you select a plan, default limits are auto-filled. You can override each value before saving.
        </p>

        <div className="tool-form-grid">
          <label className="tool-form-label full">
            Plan Code
            <select value={planCode} onChange={(event) => handlePlanChange(event.target.value)}>
              <option value="starter">starter</option>
              <option value="growth">growth</option>
              <option value="enterprise">enterprise</option>
            </select>
          </label>

          <label className="tool-form-label">
            Max Tools
            <input
              type="number"
              min={1}
              value={toolsMax}
              onChange={(event) => setToolsMax(event.target.value)}
              placeholder="75"
            />
          </label>

          <label className="tool-form-label">
            Rate Limit (rpm)
            <input
              type="number"
              min={1}
              value={runRPM}
              onChange={(event) => setRunRPM(event.target.value)}
              placeholder="1200"
            />
          </label>

          <label className="tool-form-label">
            Audit Retention (days)
            <input
              type="number"
              min={1}
              value={auditRetentionDays}
              onChange={(event) => setAuditRetentionDays(event.target.value)}
              placeholder="90"
            />
          </label>
        </div>

        {formError && <p className="field-error">{formError}</p>}
        {error && <p className="form-server-error">{error}</p>}

        <div className="tool-form-actions">
          <button className="btn-secondary" onClick={onClose} disabled={isSaving}>
            Cancel
          </button>
          <button onClick={submit} disabled={isSaving}>
            {isSaving ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </div>
    </div>
  );
}

function parsePositiveInt(raw: string): number | null {
  const value = Number.parseInt(raw, 10);
  if (Number.isNaN(value) || value <= 0) {
    return null;
  }
  return value;
}

function isPlanPresetCode(v: string): v is PlanPresetCode {
  return v === 'starter' || v === 'growth' || v === 'enterprise';
}
