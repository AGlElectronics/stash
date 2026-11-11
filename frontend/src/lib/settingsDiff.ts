import type { SystemSettings } from '@/api/settings';

/** The settings the privacy and notifications card edits. */
export const PRIVACY_KEYS = [
  'con_end_date',
  'lostfound_retention_mode',
  'lostfound_retention_days',
  'lostfound_retention_hours',
  'user_inactive_days',
  'telegram_enabled',
  'overdue_email_enabled',
  'overdue_notify_emails',
  'smtp_host',
  'smtp_port',
  'smtp_username',
  'smtp_from',
  'smtp_tls',
] as const;
export type PrivacyKey = (typeof PRIVACY_KEYS)[number];
export type PrivacyDraft = Pick<SystemSettings, PrivacyKey>;

export function pickPrivacySettings(s: SystemSettings): PrivacyDraft {
  return Object.fromEntries(PRIVACY_KEYS.map((k) => [k, s[k]])) as PrivacyDraft;
}

/** Only the keys that changed and are not managed in Den. */
export function settingsDiff(
  server: PrivacyDraft,
  draft: PrivacyDraft,
  managed: string[]
): Partial<PrivacyDraft> {
  const out: Record<string, unknown> = {};
  for (const k of PRIVACY_KEYS) {
    if (!managed.includes(k) && draft[k] !== server[k]) out[k] = draft[k];
  }
  return out as Partial<PrivacyDraft>;
}
