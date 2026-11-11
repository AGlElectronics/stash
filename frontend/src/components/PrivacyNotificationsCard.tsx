import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@/api/client';
import { getSettings, patchSettings } from '@/api/settings';
import { Button } from '@/components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card';

import { pickPrivacySettings as pick, settingsDiff, type PrivacyDraft as Draft, type PrivacyKey as Key } from '@/lib/settingsDiff';

const inputCls = 'w-full rounded-md border border-border bg-background px-3 py-2 disabled:opacity-60';

export function PrivacyNotificationsCard() {
  const qc = useQueryClient();
  const settings = useQuery({ queryKey: ['system-settings'], queryFn: getSettings });
  const [draft, setDraft] = useState<Draft | null>(null);
  const [savedAt, setSavedAt] = useState<string | null>(null);

  useEffect(() => {
    if (settings.data) setDraft(pick(settings.data));
  }, [settings.data]);

  const managed = useMemo(() => settings.data?.managed_keys ?? [], [settings.data]);
  const diff = settings.data && draft ? settingsDiff(pick(settings.data), draft, managed) : {};
  const dirty = Object.keys(diff).length > 0;

  const save = useMutation({
    mutationFn: () => patchSettings(diff),
    onSuccess: (data) => {
      qc.setQueryData(['system-settings'], data);
      setSavedAt(new Date().toLocaleTimeString());
    },
  });

  if (!draft) return null;
  const set = <K extends Key>(k: K, v: Draft[K]) => setDraft({ ...draft, [k]: v });
  const fromDen = (k: Key) => managed.includes(k);
  const denHint = (k: Key) =>
    fromDen(k) ? <span className="block text-xs text-muted-foreground">Managed in Den</span> : null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>Privacy and notifications</CardTitle>
        <CardDescription>
          How long guest data in Lost &amp; Found is kept, and how Stash reaches people. Values set in
          Den (Admin → Module settings) override these while Stash is connected.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-5 text-sm">
        <fieldset className="space-y-3">
          <legend className="font-semibold">Lost &amp; Found retention</legend>
          <label className="block space-y-1">
            <span className="font-medium">Delete reports</span>
            <select
              className={inputCls}
              value={draft.lostfound_retention_mode}
              disabled={fromDen('lostfound_retention_mode')}
              onChange={(e) =>
                set('lostfound_retention_mode', e.target.value as Draft['lostfound_retention_mode'])
              }
            >
              <option value="after_close">A number of hours after they are returned or closed</option>
              <option value="after_con">A number of days after the convention</option>
              <option value="off">Never (not recommended)</option>
            </select>
            {denHint('lostfound_retention_mode')}
          </label>
          {draft.lostfound_retention_mode === 'after_close' ? (
            <label className="block space-y-1">
              <span className="font-medium">Hours after closing</span>
              <input
                type="number"
                min={1}
                className={inputCls}
                value={draft.lostfound_retention_hours}
                disabled={fromDen('lostfound_retention_hours')}
                onChange={(e) => set('lostfound_retention_hours', Number(e.target.value))}
              />
              {denHint('lostfound_retention_hours')}
            </label>
          ) : null}
          {draft.lostfound_retention_mode === 'after_con' ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <label className="block space-y-1">
                <span className="font-medium">Last day of the convention</span>
                <input
                  type="date"
                  className={inputCls}
                  value={draft.con_end_date}
                  disabled={fromDen('con_end_date')}
                  onChange={(e) => set('con_end_date', e.target.value)}
                />
                {denHint('con_end_date')}
              </label>
              <label className="block space-y-1">
                <span className="font-medium">Days after the convention</span>
                <input
                  type="number"
                  min={1}
                  className={inputCls}
                  value={draft.lostfound_retention_days}
                  disabled={fromDen('lostfound_retention_days')}
                  onChange={(e) => set('lostfound_retention_days', Number(e.target.value))}
                />
                {denHint('lostfound_retention_days')}
              </label>
            </div>
          ) : null}
          <label className="block space-y-1">
            <span className="font-medium">Anonymise staff who have not signed in for (days)</span>
            <input
              type="number"
              min={0}
              className={inputCls}
              value={draft.user_inactive_days}
              onChange={(e) => set('user_inactive_days', Number(e.target.value))}
            />
            <span className="block text-xs text-muted-foreground">0 = never.</span>
          </label>
        </fieldset>

        <fieldset className="space-y-3">
          <legend className="font-semibold">Notifications</legend>
          <label className="flex items-start gap-3">
            <input
              type="checkbox"
              className="mt-1 accent-primary"
              checked={draft.telegram_enabled}
              onChange={(e) => set('telegram_enabled', e.target.checked)}
            />
            <span>
              <span className="font-semibold">Telegram bot for Lost &amp; Found</span>
              <span className="block text-muted-foreground">
                Guests link their report by sending <code>/start LF0042</code> to the bot; the desk
                can then notify them. Also needs <code>TELEGRAM_BOT_TOKEN</code> on the backend.
              </span>
            </span>
          </label>
          <label className="flex items-start gap-3">
            <input
              type="checkbox"
              className="mt-1 accent-primary"
              checked={draft.overdue_email_enabled}
              onChange={(e) => set('overdue_email_enabled', e.target.checked)}
            />
            <span>
              <span className="font-semibold">Email overdue-return reminders</span>
              <span className="block text-muted-foreground">
                Once a day, to whoever received the gear and to the addresses below.
              </span>
            </span>
          </label>
          <label className="block space-y-1">
            <span className="font-medium">Also send the overdue digest to</span>
            <input
              className={inputCls}
              placeholder="logistics@example.org, lead@example.org"
              value={draft.overdue_notify_emails}
              onChange={(e) => set('overdue_notify_emails', e.target.value)}
            />
          </label>
        </fieldset>

        <fieldset className="space-y-3">
          <legend className="font-semibold">Email server (SMTP)</legend>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {(
              [
                ['smtp_host', 'Host', 'smtp.example.org'],
                ['smtp_username', 'Username', 'optional'],
                ['smtp_from', 'From address', 'Stash <stash@example.org>'],
              ] as const
            ).map(([k, label, placeholder]) => (
              <label key={k} className="block space-y-1">
                <span className="font-medium">{label}</span>
                <input
                  className={inputCls}
                  placeholder={placeholder}
                  value={draft[k]}
                  disabled={fromDen(k)}
                  onChange={(e) => set(k, e.target.value)}
                />
                {denHint(k)}
              </label>
            ))}
            <label className="block space-y-1">
              <span className="font-medium">Port</span>
              <input
                type="number"
                className={inputCls}
                value={draft.smtp_port}
                disabled={fromDen('smtp_port')}
                onChange={(e) => set('smtp_port', Number(e.target.value))}
              />
              {denHint('smtp_port')}
            </label>
            <label className="block space-y-1">
              <span className="font-medium">Security</span>
              <select
                className={inputCls}
                value={draft.smtp_tls}
                disabled={fromDen('smtp_tls')}
                onChange={(e) => set('smtp_tls', e.target.value as Draft['smtp_tls'])}
              >
                <option value="starttls">STARTTLS (port 587)</option>
                <option value="tls">TLS (port 465)</option>
                <option value="none">None (local test server only)</option>
              </select>
              {denHint('smtp_tls')}
            </label>
          </div>
          <p className="text-xs text-muted-foreground">
            The password is the <code>SMTP_PASSWORD</code> secret on the backend, never stored here.
          </p>
        </fieldset>

        <div className="flex items-center gap-3">
          <Button onClick={() => save.mutate()} disabled={!dirty || save.isPending} type="button">
            {save.isPending ? 'Saving…' : 'Save'}
          </Button>
          {save.isError ? (
            <span className="text-sm text-destructive">
              {save.error instanceof ApiError ? save.error.message : 'Failed to save settings.'}
            </span>
          ) : null}
          {!save.isError && savedAt ? (
            <span className="text-sm text-muted-foreground">Saved at {savedAt}.</span>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}
