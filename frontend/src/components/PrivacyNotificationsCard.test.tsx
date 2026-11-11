import { describe, expect, it, vi, beforeEach } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { renderWithQuery } from '@/test/render';
import type { SystemSettings } from '@/api/settings';

const api = vi.hoisted(() => ({ getSettings: vi.fn(), patchSettings: vi.fn() }));
vi.mock('@/api/settings', () => api);

import { PrivacyNotificationsCard } from './PrivacyNotificationsCard';

const settings: SystemSettings = {
  force_barcode_on_signout: false,
  con_end_date: '',
  lostfound_retention_mode: 'after_close',
  lostfound_retention_days: 30,
  lostfound_retention_hours: 720,
  user_inactive_days: 0,
  telegram_enabled: false,
  overdue_email_enabled: false,
  overdue_notify_emails: '',
  smtp_host: 'smtp.den.example',
  smtp_port: 587,
  smtp_username: '',
  smtp_from: '',
  smtp_tls: 'starttls',
  managed_by: 'den',
  managed_keys: ['organization_name', 'smtp_host'],
};

describe('PrivacyNotificationsCard', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    api.getSettings.mockResolvedValue(settings);
    api.patchSettings.mockImplementation(async (p: Partial<SystemSettings>) => ({ ...settings, ...p }));
  });

  it('shows Den-managed settings read-only', async () => {
    renderWithQuery(<PrivacyNotificationsCard />);
    const host = await screen.findByPlaceholderText('smtp.example.org');
    expect(host).toBeDisabled();
    expect(host).toHaveValue('smtp.den.example');
    expect(screen.getAllByText('Managed in Den').length).toBeGreaterThan(0);
  });

  it('saves only what changed', async () => {
    renderWithQuery(<PrivacyNotificationsCard />);
    const save = await screen.findByRole('button', { name: 'Save' });
    expect(save).toBeDisabled();
    fireEvent.click(screen.getByRole('checkbox', { name: /telegram bot/i }));
    fireEvent.change(screen.getByLabelText('Hours after closing'), { target: { value: '48' } });
    fireEvent.click(save);
    await waitFor(() =>
      expect(api.patchSettings).toHaveBeenCalledWith({ telegram_enabled: true, lostfound_retention_hours: 48 })
    );
  });

  it('switches the retention fields with the mode', async () => {
    renderWithQuery(<PrivacyNotificationsCard />);
    fireEvent.change(await screen.findByLabelText('Delete reports'), { target: { value: 'after_con' } });
    expect(screen.getByLabelText('Last day of the convention')).toBeInTheDocument();
    expect(screen.queryByLabelText('Hours after closing')).not.toBeInTheDocument();
  });
});
