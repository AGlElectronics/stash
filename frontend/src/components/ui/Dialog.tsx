import { useEffect } from 'react';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';

type DialogProps = {
  open: boolean;
  onClose: () => void;
  title?: string;
  description?: string;
  className?: string;
  children: React.ReactNode;
};

export function Dialog({ open, onClose, title, description, className, children }: DialogProps) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-[2px]"
      onClick={onClose}
    >
      <div
        className={cn(
          'relative bg-card text-card-foreground rounded-card border border-border shadow-pop w-full max-w-lg max-h-[90vh] overflow-auto',
          className
        )}
        onClick={(e) => e.stopPropagation()}
      >
        <button
          type="button"
          className="absolute right-3 top-3 rounded-pill p-1.5 opacity-70 hover:opacity-100 hover:bg-accent transition-colors"
          onClick={onClose}
          aria-label="Close"
        >
          <X className="w-4 h-4" />
        </button>
        {(title || description) && (
          <div className="p-5 pb-4 border-b border-border">
            {title ? <h2 className="font-heading font-bold leading-tight pr-8">{title}</h2> : null}
            {description ? <p className="text-sm text-muted-foreground mt-1">{description}</p> : null}
          </div>
        )}
        <div className="p-5">{children}</div>
      </div>
    </div>
  );
}
