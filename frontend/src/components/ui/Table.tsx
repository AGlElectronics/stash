import { cn } from '@/lib/utils';

export function Table({ className, ...rest }: React.HTMLAttributes<HTMLTableElement>) {
  return (
    <div className="relative w-full overflow-auto rounded-lg border border-border bg-card shadow-card">
      <table className={cn('w-full caption-bottom text-sm', className)} {...rest} />
    </div>
  );
}

export function THead({ className, ...rest }: React.HTMLAttributes<HTMLTableSectionElement>) {
  return <thead className={cn('bg-muted/60 border-b border-border', className)} {...rest} />;
}

export function TBody({ className, ...rest }: React.HTMLAttributes<HTMLTableSectionElement>) {
  return <tbody className={cn('[&_tr:last-child]:border-0', className)} {...rest} />;
}

export function TR({ className, ...rest }: React.HTMLAttributes<HTMLTableRowElement>) {
  return <tr className={cn('border-b border-border transition-colors hover:bg-muted/50', className)} {...rest} />;
}

export function TH({ className, ...rest }: React.ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th
      className={cn(
        'h-10 px-3 text-left align-middle text-xs font-semibold text-muted-foreground uppercase tracking-wide',
        className
      )}
      {...rest}
    />
  );
}

export function TD({ className, ...rest }: React.TdHTMLAttributes<HTMLTableCellElement>) {
  return <td className={cn('px-3 py-2 align-middle', className)} {...rest} />;
}
