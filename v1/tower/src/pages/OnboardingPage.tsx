import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';

import { createTool, getBillingStatus, getTools, getUserMe, runTool } from '../lib/api';

type Step = 1 | 2 | 3;

export default function OnboardingPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [step, setStep] = useState<Step>(1);
  const [toolName, setToolName] = useState('my-first-tool');
  const [toolURL, setToolURL] = useState('http://mock-tools:8081/echo');
  const [toolMethod, setToolMethod] = useState<'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'>('POST');
  const [createdToolName, setCreatedToolName] = useState('');
  const [runOutput, setRunOutput] = useState('');

  const toolsQuery = useQuery({ queryKey: ['tools'], queryFn: getTools });
  const billingQuery = useQuery({ queryKey: ['billing-status'], queryFn: getBillingStatus });
  const meQuery = useQuery({ queryKey: ['user-me'], queryFn: getUserMe });

  useEffect(() => {
    if ((toolsQuery.data?.items.length ?? 0) > 0) {
      navigate('/tools', { replace: true });
    }
  }, [toolsQuery.data, navigate]);

  const createToolMutation = useMutation({
    mutationFn: () =>
      createTool({
        name: toolName.trim(),
        kind: 'http',
        description: 'First tool created from onboarding',
        method: toolMethod,
        url: toolURL.trim(),
        input_schema: {
          type: 'object',
          properties: {
            message: { type: 'string' },
          },
          required: ['message'],
        },
        action_type: 'read',
        classification: 'internal',
        sensitivity: 'low',
        risk_level: 1,
        enabled: true,
      }),
    onSuccess: (tool) => {
      setCreatedToolName(tool.name);
      queryClient.invalidateQueries({ queryKey: ['tools'] });
      setStep(3);
    },
  });

  const runMutation = useMutation({
    mutationFn: async () => {
      const name = createdToolName || toolName.trim();
      const result = await runTool(name, { message: 'hello from onboarding' });
      return JSON.stringify(result, null, 2);
    },
    onSuccess: (text) => setRunOutput(text),
  });

  const planLabel = useMemo(() => {
    const plan = billingQuery.data?.plan_code;
    if (!plan) return 'Starter';
    return plan.charAt(0).toUpperCase() + plan.slice(1);
  }, [billingQuery.data]);

  if (toolsQuery.isLoading) {
    return (
      <div className="panel-page onboarding-page">
        <h2>Onboarding</h2>
        <p className="muted">Checking your workspace...</p>
      </div>
    );
  }

  return (
    <div className="panel-page onboarding-page">
      <div className="onboarding-header">
        <div>
          <h2>Welcome to Nexus</h2>
          <p className="muted">Let&apos;s get your tenant ready in three quick steps.</p>
        </div>
        <button className="btn-secondary" onClick={() => navigate('/tools')}>
          Skip
        </button>
      </div>

      <div className="onboarding-steps">
        <span className={step === 1 ? 'active' : ''}>1. Welcome</span>
        <span className={step === 2 ? 'active' : ''}>2. Register tool</span>
        <span className={step === 3 ? 'active' : ''}>3. Test run</span>
      </div>

      {step === 1 && (
        <section className="billing-section onboarding-section">
          <h3>Step 1 — Confirm plan</h3>
          <p className="muted">Organization: {meQuery.data?.org_id || 'Current tenant'}</p>
          <p className="muted">Current plan: {planLabel}</p>
          {billingQuery.error && <p className="field-error">{(billingQuery.error as Error).message}</p>}
          <div className="onboarding-actions">
            <button onClick={() => setStep(2)}>Continue</button>
          </div>
        </section>
      )}

      {step === 2 && (
        <section className="billing-section onboarding-section">
          <h3>Step 2 — Register your first tool</h3>
          <div className="onboarding-form-grid">
            <label>
              Tool name
              <input value={toolName} onChange={(e) => setToolName(e.target.value)} />
            </label>
            <label>
              Method
              <select value={toolMethod} onChange={(e) => setToolMethod(e.target.value as typeof toolMethod)}>
                {['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map((method) => (
                  <option key={method} value={method}>
                    {method}
                  </option>
                ))}
              </select>
            </label>
            <label className="full">
              Endpoint URL
              <input value={toolURL} onChange={(e) => setToolURL(e.target.value)} />
            </label>
          </div>
          {createToolMutation.error && <p className="field-error">{(createToolMutation.error as Error).message}</p>}
          <div className="onboarding-actions">
            <button className="btn-secondary" onClick={() => setStep(1)}>
              Back
            </button>
            <button
              onClick={() => createToolMutation.mutate()}
              disabled={createToolMutation.isPending || !toolName.trim() || !toolURL.trim()}
            >
              {createToolMutation.isPending ? 'Creating...' : 'Create tool'}
            </button>
          </div>
        </section>
      )}

      {step === 3 && (
        <section className="billing-section onboarding-section">
          <h3>Step 3 — Test it</h3>
          <p className="muted">
            Tool ready: <code>{createdToolName || toolName}</code>
          </p>
          <div className="onboarding-actions">
            <button className="btn-secondary" onClick={() => setStep(2)}>
              Back
            </button>
            <button onClick={() => runMutation.mutate()} disabled={runMutation.isPending}>
              {runMutation.isPending ? 'Running...' : 'Run test'}
            </button>
          </div>
          {runMutation.error && <p className="field-error">{(runMutation.error as Error).message}</p>}
          {runOutput && (
            <pre className="onboarding-output" aria-label="onboarding-run-output">
              {runOutput}
            </pre>
          )}
          <div className="onboarding-actions">
            <button onClick={() => navigate('/tools')}>Finish onboarding</button>
          </div>
        </section>
      )}
    </div>
  );
}
