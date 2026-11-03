import { forwardRef } from 'react';
import { cn } from '@/lib/utils';

const fieldClasses =
  'w-full rounded-md border border-input bg-background text-sm placeholder:text-muted-foreground transition-colors focus-visible:outline-none focus-visible:border-primary focus-visible:ring-[3px] focus-visible:ring-primary-soft disabled:cursor-not-allowed disabled:opacity-50';

export const Input = forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className, ...rest }, ref) {
    return <input ref={ref} className={cn('flex h-9 px-3 py-1', fieldClasses, className)} {...rest} />;
  }
);

export const Textarea = forwardRef<HTMLTextAreaElement, React.TextareaHTMLAttributes<HTMLTextAreaElement>>(
  function Textarea({ className, ...rest }, ref) {
    return <textarea ref={ref} className={cn('flex min-h-20 px-3 py-2', fieldClasses, className)} {...rest} />;
  }
);

export const Select = forwardRef<HTMLSelectElement, React.SelectHTMLAttributes<HTMLSelectElement>>(
  function Select({ className, children, ...rest }, ref) {
    return (
      <select ref={ref} className={cn('flex h-9 px-3 py-1', fieldClasses, className)} {...rest}>
        {children}
      </select>
    );
  }
);

export function Label({
  htmlFor,
  children,
  className,
}: {
  htmlFor?: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <label htmlFor={htmlFor} className={cn('text-sm font-semibold block mb-1', className)}>
      {children}
    </label>
  );
}

export function Field({
  label,
  htmlFor,
  hint,
  error,
  children,
}: {
  label?: string;
  htmlFor?: string;
  hint?: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      {label ? <Label htmlFor={htmlFor}>{label}</Label> : null}
      {children}
      {hint && !error ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  );
}
