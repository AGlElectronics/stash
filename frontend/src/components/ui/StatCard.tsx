import { Link } from '@tanstack/react-router';
import { Card, CardContent } from '@/components/ui/Card';
import { cn } from '@/lib/utils';

type Tone = 'default' | 'good' | 'warn' | 'bad';

const TONE_VALUE: Record<Tone, string> = {
  default: 'text-foreground',
  good: 'text-success',
  warn: 'text-warning',
  bad: 'text-destructive',
};

const TONE_ICON: Record<Tone, string> = {
  default: 'bg-primary-soft text-primary',
  good: 'bg-success/15 text-success',
  warn: 'bg-warning/15 text-warning',
  bad: 'bg-destructive/15 text-destructive',
};

export function StatCard({
  label,
  value,
  icon: Icon,
  tone = 'default',
  to,
  params,
  search,
  hint,
}: {
  label: string;
  value: number | string;
  icon: React.ComponentType<{ className?: string }>;
  tone?: Tone;
  /** When set, the whole card becomes a link (e.g. to a pre-filtered list). */
  to?: string;
  params?: Record<string, string>;
  search?: Record<string, unknown>;
  hint?: string;
}) {
  const body = (
    <Card clickable={!!to} className="h-full">
      <CardContent className="p-4 sm:p-5">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="text-xs uppercase text-muted-foreground tracking-wide mb-1 truncate">{label}</p>
            <p className={cn('text-3xl font-heading font-extrabold tabular-nums leading-none', TONE_VALUE[tone])}>
              {value}
            </p>
            {hint ? <p className="text-xs text-muted-foreground mt-1.5">{hint}</p> : null}
          </div>
          <div className={cn('rounded-pill p-2 shrink-0', TONE_ICON[tone])}>
            <Icon className="w-4 h-4" />
          </div>
        </div>
      </CardContent>
    </Card>
  );
  if (to) {
    return (
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      <Link to={to as any} params={params as any} search={search as any} className="block focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-card">
        {body}
      </Link>
    );
  }
  return body;
}
