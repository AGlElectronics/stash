import { apiRequest } from './client';

export type RetentionMode = 'off' | 'after_con' | 'after_close';
export type SmtpTls = 'starttls' | 'tls' | 'none';

export type SystemSettings = {
  force_barcode_on_signout: boolean;
  organization_name?: string;
  default_item_owner?: string;
  default_currency?: string;
  barcode_prefix?: string;
  barcode_width?: number;
  preserve_sacred_lostfound?: boolean;
  /** Last day of the convention, YYYY-MM-DD ('' = not set). */
  con_end_date: string;
  lostfound_retention_mode: RetentionMode;
  lostfound_retention_days: number;
  lostfound_retention_hours: number;
  user_inactive_days: number;
  telegram_enabled: boolean;
  overdue_email_enabled: boolean;
  overdue_notify_emails: string;
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_from: string;
  smtp_tls: SmtpTls;
  /** "den" while paired with Den; managed_keys are then read-only here. */
  managed_by?: string;
  managed_keys?: string[];
};

export const getSettings = () => apiRequest<SystemSettings>('/api/settings');

export const patchSettings = (patch: Partial<SystemSettings>) =>
  apiRequest<SystemSettings>('/api/settings', { method: 'PATCH', body: patch });
