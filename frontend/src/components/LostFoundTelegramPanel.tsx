import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Send } from 'lucide-react';
import { ApiError } from '@/api/client';
import { getTelegramStatus, notifyLostFoundGuest, type LostFoundReport } from '@/api/lostfound';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Textarea } from '@/components/ui/Input';
import { PERMS, useMe } from '@/hooks/useMe';

/** "LF-0042" → "LF0042", the form guests type after /start. */
function telegramStartCode(reportNumber: string): string {
  return reportNumber.replace('-', '');
}

/**
 * Telegram on a Lost & Found report: how the guest links the report, whether
 * they did, and the "Notify guest" button (lostfound.notify).
 */
export function LostFoundTelegramPanel({ report }: { report: LostFoundReport }) {
  const { can } = useMe();
  const status = useQuery({ queryKey: ['lostfound-telegram'], queryFn: getTelegramStatus, retry: false });
  const [open, setOpen] = useState(false);
  const [message, setMessage] = useState('');
  const [sentAt, setSentAt] = useState<string | null>(null);

  const notify = useMutation({
    mutationFn: () => notifyLostFoundGuest(report.id, message),
    onSuccess: () => {
      setOpen(false);
      setMessage('');
      setSentAt(new Date().toLocaleTimeString());
    },
  });

  if (!status.data?.enabled || !status.data.configured) return null;
  const code = telegramStartCode(report.report_number);
  const bot = status.data.bot_username;

  return (
    <div className="rounded-md border border-border p-3 text-sm space-y-2">
      <div className="flex items-center gap-2 flex-wrap">
        <span className="font-semibold">Telegram</span>
        {report.telegram_linked ? (
          <Badge variant="success">Guest linked</Badge>
        ) : (
          <Badge variant="outline">Not linked</Badge>
        )}
        {sentAt ? <span className="text-muted-foreground">Message sent at {sentAt}.</span> : null}
      </div>
      {!report.telegram_linked ? (
        <p className="text-muted-foreground">
          Ask the guest to send <code className="rounded bg-muted px-1">/start {code}</code> to{' '}
          {bot ? (
            <a className="text-primary underline" href={`https://t.me/${bot}?start=${code}`} target="_blank" rel="noreferrer">
              @{bot}
            </a>
          ) : (
            'our Telegram bot'
          )}
          . They can send <code className="rounded bg-muted px-1">/stop</code> at any time.
        </p>
      ) : null}
      {report.telegram_linked && can(PERMS.lostfoundNotify) ? (
        <Button variant="outline" onClick={() => setOpen(true)}>
          <Send className="w-4 h-4" />
          Notify guest
        </Button>
      ) : null}
      <Dialog open={open} onClose={() => setOpen(false)} title="Notify guest on Telegram">
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            notify.mutate();
          }}
        >
          <Field
            label="Message"
            htmlFor="notify-message"
            hint="Leave empty to send: “News about your Lost & Found report … please come to the Lost & Found desk.”"
          >
            <Textarea
              id="notify-message"
              rows={4}
              maxLength={1000}
              value={message}
              onChange={(e) => setMessage(e.target.value)}
            />
          </Field>
          {notify.isError ? (
            <p role="alert" className="text-sm text-destructive">
              {notify.error instanceof ApiError ? notify.error.message : 'Sending failed.'}
            </p>
          ) : null}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={notify.isPending}>
              {notify.isPending ? 'Sending…' : 'Send'}
            </Button>
          </div>
        </form>
      </Dialog>
    </div>
  );
}
