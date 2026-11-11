import { describe, expect, it } from 'vitest';
import type { SystemSettings } from '@/api/settings';
import { pickPrivacySettings, settingsDiff } from './settingsDiff';

const server: SystemSettings = {
  force_barcode_on_signout: false,
  con_end_date: '',
  lostfound_retention_mode: 'after_close',
  lostfound_retention_days: 30,
  lostfound_retention_hours: 720,
  user_inactive_days: 0,
  telegram_enabled: false,
  overdue_email_enabled: false,
  overdue_notify_emails: '',
  smtp_host: '',
  smtp_port: 587,
  smtp_username: '',
  smtp_from: '',
  smtp_tls: 'starttls',
};

describe('settingsDiff', () => {
  it('sends only changed keys', () => {
    const base = pickPrivacySettings(server);
    const draft = { ...base, telegram_enabled: true, smtp_host: 'smtp.example.org' };
    expect(settingsDiff(base, draft, [])).toEqual({ telegram_enabled: true, smtp_host: 'smtp.example.org' });
  });

  it('never sends keys managed in Den', () => {
    const base = pickPrivacySettings(server);
    const draft = { ...base, smtp_host: 'other', lostfound_retention_hours: 24 };
    expect(settingsDiff(base, draft, ['smtp_host'])).toEqual({ lostfound_retention_hours: 24 });
  });

  it('is empty when nothing changed', () => {
    const base = pickPrivacySettings(server);
    expect(settingsDiff(base, { ...base }, [])).toEqual({});
  });
});
