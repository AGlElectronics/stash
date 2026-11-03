import { useCallback, useSyncExternalStore } from 'react';
import { Moon, Sun } from 'lucide-react';
import { applyTheme, getAppliedTheme, type ThemePreference } from '@/lib/theme';
import { cn } from '@/lib/utils';

function subscribe(onStoreChange: () => void) {
  const el = document.documentElement;
  const obs = new MutationObserver(() => onStoreChange());
  obs.observe(el, { attributes: true, attributeFilter: ['class'] });
  return () => obs.disconnect();
}

function getServerSnapshot(): ThemePreference {
  return 'dark';
}

export function ThemeToggle({ className }: { className?: string }) {
  const mode = useSyncExternalStore(subscribe, getAppliedTheme, getServerSnapshot);

  const onClick = useCallback(() => {
    const next: ThemePreference = mode === 'dark' ? 'light' : 'dark';
    applyTheme(next);
  }, [mode]);

  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'w-full flex items-center gap-2 text-sm px-3 py-2 rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors',
        className
      )}
      aria-label={mode === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
    >
      {mode === 'dark' ? <Sun className="w-4 h-4 shrink-0" /> : <Moon className="w-4 h-4 shrink-0" />}
      <span>{mode === 'dark' ? 'Light mode' : 'Dark mode'}</span>
    </button>
  );
}
