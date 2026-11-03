import { cn } from '@/lib/utils';

type Variant = 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning';

const VARIANTS: Record<Variant, string> = {
  default: 'bg-primary text-primary-foreground',
  secondary: 'bg-secondary text-secondary-foreground',
  destructive: 'bg-destructive/15 text-destructive',
  outline: 'border border-border text-foreground',
  success: 'bg-success/15 text-success',
  warning: 'bg-warning/15 text-warning',
};

export function Badge({
  className,
  variant = 'default',
  children,
}: {
  className?: string;
  variant?: Variant;
  children: React.ReactNode;
}) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-pill px-2.5 py-0.5 text-xs font-semibold',
        VARIANTS[variant],
        className
      )}
    >
      {children}
    </span>
  );
}
