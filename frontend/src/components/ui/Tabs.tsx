import { cn } from '@/lib/utils';

export type TabDef<T extends string> = {
  id: T;
  label: string;
  icon?: React.ComponentType<{ className?: string }>;
  /** Optional count badge shown after the label. */
  count?: number;
};

/** Pill-style segmented tab control. */
export function Tabs<T extends string>({
  tabs,
  value,
  onChange,
  className,
}: {
  tabs: TabDef<T>[];
  value: T;
  onChange: (id: T) => void;
  className?: string;
}) {
  return (
    <div
      role="tablist"
      className={cn(
        'inline-flex flex-wrap items-center gap-1 rounded-pill bg-muted p-1 max-w-full overflow-x-auto',
        className
      )}
    >
      {tabs.map((t) => {
        const Icon = t.icon;
        const active = t.id === value;
        return (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => onChange(t.id)}
            className={cn(
              'inline-flex items-center gap-1.5 rounded-pill px-3.5 py-1.5 text-sm font-semibold whitespace-nowrap transition-colors',
              active
                ? 'bg-card text-foreground shadow-card'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {Icon ? <Icon className="w-4 h-4" /> : null}
            {t.label}
            {typeof t.count === 'number' ? (
              <span
                className={cn(
                  'rounded-pill px-1.5 text-xs tabular-nums',
                  active ? 'bg-primary-soft text-primary' : 'bg-secondary'
                )}
              >
                {t.count}
              </span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}
