import { useQuery } from '@tanstack/react-query';
import {
  Box,
  Hammer,
  HelpCircle,
  Move,
  PackagePlus,
  PackageX,
  RotateCcw,
  Search,
  Send,
  ShieldX,
  Sparkles,
  TrendingDown,
  TrendingUp,
  Undo2,
} from 'lucide-react';
import { getItemHistory } from '@/api/items';
import type { ItemEvent, ItemEventType } from '@/types';
import { Badge } from '@/components/ui/Badge';
import { LoadingRow, ErrorRow, EmptyRow } from '@/components/ui/States';
import { formatDateTime } from '@/lib/utils';

type IconAndLabel = { icon: React.ComponentType<{ className?: string }>; label: string };

const EVENT_META: Record<ItemEventType, IconAndLabel> = {
  created: { icon: PackagePlus, label: 'Created' },
  moved: { icon: Move, label: 'Moved' },
  quantity_adjusted: { icon: TrendingUp, label: 'Quantity adjusted' },
  signed_out: { icon: Send, label: 'Signed out' },
  returned: { icon: Undo2, label: 'Returned' },
  consumed: { icon: TrendingDown, label: 'Consumed' },
  marked_damaged: { icon: ShieldX, label: 'Marked damaged' },
  repaired: { icon: Hammer, label: 'Repaired' },
  marked_lost: { icon: PackageX, label: 'Marked lost' },
  marked_found: { icon: Search, label: 'Marked found' },
  condition_changed: { icon: Sparkles, label: 'Condition changed' },
  retired: { icon: Box, label: 'Retired' },
  restored: { icon: RotateCcw, label: 'Restored' },
};

function describe(e: ItemEvent): string {
  const meta = EVENT_META[e.event_type] ?? { label: e.event_type };
  const parts: string[] = [meta.label];
  if (e.quantity !== 0 && e.event_type !== 'created') {
    parts.push(`qty ${e.quantity > 0 ? '+' : ''}${e.quantity}`);
  } else if (e.event_type === 'created') {
    parts.push(`qty ${e.quantity}`);
  }
  if (e.condition_before && e.condition_after) {
    parts.push(`${e.condition_before} → ${e.condition_after}`);
  }
  return parts.join(' · ');
}

export function ItemHistoryTimeline({ itemId }: { itemId: string }) {
  const history = useQuery({
    queryKey: ['item-history', itemId],
    queryFn: () => getItemHistory(itemId, { limit: 200 }),
  });

  if (history.isLoading) return <LoadingRow />;
  if (history.isError) return <ErrorRow message="Failed to load history." />;
  if (!history.data || history.data.events.length === 0) {
    return <EmptyRow message="No history yet." />;
  }

  return (
    <ol className="relative border-l border-border pl-6 space-y-4">
      {history.data.events.map((e) => {
        const meta = EVENT_META[e.event_type] ?? { icon: HelpCircle, label: e.event_type };
        const Icon = meta.icon;
        return (
          <li key={e.id} className="relative">
            <span className="absolute -left-9 top-0.5 inline-flex items-center justify-center w-6 h-6 rounded-full border border-border bg-background">
              <Icon className="w-3.5 h-3.5" />
            </span>
            <div className="flex flex-wrap items-start gap-2">
              <span className="font-medium text-sm">{describe(e)}</span>
              <Badge variant="outline" className="text-xs">
                {formatDateTime(e.occurred_at)}
              </Badge>
            </div>
            {e.note ? <p className="text-sm text-muted-foreground mt-1">{e.note}</p> : null}
            {e.actor_display_name || e.actor_oidc_subject ? (
              <p className="text-xs text-muted-foreground mt-1">
                {e.actor_display_name ?? 'Unknown'}
                {e.actor_oidc_subject ? ` · ${e.actor_oidc_subject.slice(0, 8)}…` : ''}
              </p>
            ) : null}
          </li>
        );
      })}
    </ol>
  );
}
