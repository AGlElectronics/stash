import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Input } from '@/components/ui/Input';

export type CreatableOption = { id: string; name: string };

export type CreatablePick = { id?: string; name: string };

type Props = {
  id?: string;
  options: CreatableOption[];
  value: CreatablePick;
  onChange: (v: CreatablePick) => void;
  onBlur?: () => void;
  disabled?: boolean;
  placeholder?: string;
  entityLabel: string;
};

export function CreatableSelect({
  id: propId,
  options,
  value,
  onChange,
  onBlur,
  disabled,
  placeholder = 'Search or type a new name…',
  entityLabel,
}: Props) {
  const autoId = useId();
  const listboxId = `${autoId}-listbox`;
  const rootRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [activeIdx, setActiveIdx] = useState(0);

  const q = value.name.trim().toLowerCase();
  const filtered = useMemo(() => {
    if (!q) return options;
    return options.filter((o) => o.name.toLowerCase().includes(q));
  }, [options, q]);

  const exact = useMemo(
    () => options.find((o) => o.name.toLowerCase() === value.name.trim().toLowerCase()),
    [options, value.name]
  );

  const showCreate =
    value.name.trim().length > 0 && !exact && !filtered.some((o) => o.name.toLowerCase() === q);

  const rows = useMemo(() => {
    const out: Array<{ kind: 'option'; opt: CreatableOption } | { kind: 'create' }> = [];
    for (const opt of filtered) out.push({ kind: 'option', opt });
    if (showCreate) out.push({ kind: 'create' });
    return out;
  }, [filtered, showCreate]);

  useEffect(() => {
    setActiveIdx(0);
  }, [open, q, rows.length]);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', onDoc);
    return () => document.removeEventListener('mousedown', onDoc);
  }, [open]);

  const inputRef = useRef<HTMLInputElement>(null);

  const commitPick = (text: string) => {
    const trimmed = text.trim();
    if (!trimmed) {
      onChange({ name: '' });
      return;
    }
    let nextId = value.id;
    if (value.id) {
      const sel = options.find((o) => o.id === value.id);
      if (!sel || sel.name !== trimmed) nextId = undefined;
    }
    const exactOpt = options.find((o) => o.name.toLowerCase() === trimmed.toLowerCase());
    if (exactOpt) {
      onChange({ id: exactOpt.id, name: exactOpt.name });
      return;
    }
    onChange({ id: nextId, name: trimmed });
  };

  const commitText = (text: string) => {
    commitPick(text);
  };

  const pickOption = (opt: CreatableOption) => {
    onChange({ id: opt.id, name: opt.name });
    setOpen(false);
  };

  const syncBlur = () => {
    const raw = inputRef.current?.value ?? value.name;
    commitPick(raw);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setOpen(false);
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (!open) setOpen(true);
      else setActiveIdx((i) => Math.min(i + 1, Math.max(rows.length - 1, 0)));
      return;
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (!open) setOpen(true);
      else setActiveIdx((i) => Math.max(i - 1, 0));
      return;
    }
    if (e.key === 'Enter' && open && rows.length > 0) {
      e.preventDefault();
      const row = rows[activeIdx];
      if (row?.kind === 'option') pickOption(row.opt);
      else if (row?.kind === 'create') {
        onChange({ name: value.name.trim() });
        setOpen(false);
      }
    }
  };

  return (
    <div ref={rootRef} className="relative">
      <div className="flex rounded-md border border-input bg-background shadow-sm focus-within:ring-2 focus-within:ring-ring">
        <Input
          ref={inputRef}
          id={propId}
          type="text"
          role="combobox"
          aria-expanded={open}
          aria-controls={listboxId}
          aria-autocomplete="list"
          disabled={disabled}
          placeholder={placeholder}
          className="border-0 shadow-none focus-visible:ring-0 rounded-r-none"
          value={value.name}
          onChange={(e) => commitText(e.target.value)}
          onFocus={() => setOpen(true)}
          onBlur={() => {
            syncBlur();
            onBlur?.();
            setTimeout(() => setOpen(false), 0);
          }}
          onKeyDown={onKeyDown}
        />
        <button
          type="button"
          tabIndex={-1}
          disabled={disabled}
          className="px-2 rounded-r-md border-l border-input text-muted-foreground hover:bg-muted/40 disabled:opacity-50"
          aria-label="Show suggestions"
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => setOpen((o) => !o)}
        >
          <ChevronDown className="h-4 w-4" />
        </button>
      </div>
      {open && rows.length > 0 ? (
        <ul
          id={listboxId}
          role="listbox"
          className="absolute z-50 mt-1 max-h-48 w-full overflow-auto rounded-md border border-input bg-card text-card-foreground text-sm shadow-md"
        >
          {rows.map((row, idx) => {
            if (row.kind === 'option') {
              const opt = row.opt;
              return (
                <li key={opt.id} role="presentation">
                  <button
                    type="button"
                    role="option"
                    aria-selected={value.id === opt.id}
                    className={cn(
                      'w-full text-left px-3 py-2 hover:bg-accent hover:text-accent-foreground',
                      idx === activeIdx && 'bg-accent text-accent-foreground'
                    )}
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => pickOption(opt)}
                  >
                    {opt.name}
                  </button>
                </li>
              );
            }
            return (
              <li key="__create__" role="presentation">
                <button
                  type="button"
                  role="option"
                  className={cn(
                    'w-full text-left px-3 py-2 text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                    idx === activeIdx && 'bg-accent text-accent-foreground'
                  )}
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => {
                    onChange({ name: value.name.trim() });
                    setOpen(false);
                  }}
                >
                  Create new {entityLabel} "{value.name.trim()}"
                </button>
              </li>
            );
          })}
        </ul>
      ) : null}
    </div>
  );
}
