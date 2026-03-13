import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { EditLimitsModal } from '../../components/EditLimitsModal';

const baseSettings = {
  plan_code: 'growth',
  hard_limits: {
    tools_max: 75,
    run_rpm: 1200,
    audit_retention_days: 90,
  },
};

describe('EditLimitsModal', () => {
  it('auto-fills limits when plan changes', async () => {
    const user = userEvent.setup();
    render(
      <EditLimitsModal
        settings={baseSettings}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    await user.selectOptions(screen.getByLabelText('Plan Code'), 'enterprise');

    expect(screen.getByLabelText('Max Tools')).toHaveValue(250);
    expect(screen.getByLabelText('Rate Limit (rpm)')).toHaveValue(5000);
    expect(screen.getByLabelText('Audit Retention (days)')).toHaveValue(365);
  });

  it('shows validation error when limits are invalid', async () => {
    const user = userEvent.setup();
    render(
      <EditLimitsModal
        settings={baseSettings}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    await user.clear(screen.getByLabelText('Max Tools'));
    await user.type(screen.getByLabelText('Max Tools'), '0');
    await user.click(screen.getByRole('button', { name: 'Save Changes' }));

    expect(screen.getByText('All limits must be positive integers.')).toBeInTheDocument();
  });

  it('submits normalized payload', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(
      <EditLimitsModal
        settings={baseSettings}
        isSaving={false}
        onClose={vi.fn()}
        onSave={onSave}
      />,
    );

    await user.selectOptions(screen.getByLabelText('Plan Code'), 'starter');
    await user.click(screen.getByRole('button', { name: 'Save Changes' }));

    expect(onSave).toHaveBeenCalledOnce();
    expect(onSave).toHaveBeenCalledWith({
      plan_code: 'starter',
      hard_limits: {
        tools_max: 20,
        run_rpm: 300,
        audit_retention_days: 30,
      },
    });
  });
});
