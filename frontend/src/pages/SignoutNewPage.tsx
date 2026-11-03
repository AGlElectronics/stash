import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  Camera,
  CameraOff,
  Minus,
  Plus,
  ScanLine,
  Send,
  Trash2,
} from 'lucide-react';
import { ApiError } from '@/api/client';
import { getItemAvailability, listItems, lookupItemByBarcode } from '@/api/items';
import { createSignout } from '@/api/signouts';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card';
import { Field, Input, Select, Textarea } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { CameraScanner } from '@/components/CameraScanner';
import { useCameraStream } from '@/hooks/useCameraStream';
import { useDepartments } from '@/hooks/useReferenceData';
import { useSettings } from '@/hooks/useSettings';
import { cn } from '@/lib/utils';
import type { Item } from '@/types';

type CartLine = {
  /** Stable id for the row, used as React key (not the item id, because we
   *  also support manual lines added without scanning). */
  rowId: string;
  item: Item;
  quantity: number;
  /** Codes the operator scanned for this line, in order. */
  scannedCodes: string[];
  /** True when this row was added by scanning (not by search). */
  fromScan: boolean;
  /** Optimistic availability while the live `/availability` query is in
   *  flight — owned minus damaged/lost. The hook below replaces this with
   *  the server-authoritative value (owned − signed-out − damaged − lost)
   *  as soon as it arrives. */
  freeSnapshot: number;
};

const newRowId = () =>
  (typeof crypto !== 'undefined' && 'randomUUID' in crypto)
    ? crypto.randomUUID()
    : `row-${Date.now()}-${Math.random().toString(36).slice(2)}`;

export function SignoutNewPage() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const settings = useSettings();
  const depts = useDepartments();

  const forced = !!settings.data?.force_barcode_on_signout;

  // Header form state.
  const [toDept, setToDept] = useState('');
  const [receivedBy, setReceivedBy] = useState('');
  const [purpose, setPurpose] = useState('');
  const [notes, setNotes] = useState('');

  // Cart state — single query field: barcode scan (try first) or name search.
  const [lines, setLines] = useState<CartLine[]>([]);
  const [addQuery, setAddQuery] = useState('');
  const [addError, setAddError] = useState<string | null>(null);
  const { active: cameraActive, stream: cameraStream, error: cameraError, stop: stopCamera, toggle: toggleCamera } =
    useCameraStream();
  const addInputRef = useRef<HTMLInputElement>(null);

  const qTrim = addQuery.trim();
  const items = useQuery({
    queryKey: ['signout-cart-search', qTrim],
    queryFn: () => listItems({ q: qTrim, limit: 25 }),
    enabled: !forced && qTrim.length >= 2,
  });

  // Live per-item availability (subtracts active sign-outs), one query per
  // line. We re-fetch on every cart change so adding the same item twice
  // doesn't keep showing yesterday's number, and so the page reflects what
  // happens if a parallel sign-out lands while we shop.
  const availabilityQueries = useQueries({
    queries: lines.map((line) => ({
      queryKey: ['item-availability', line.item.id],
      queryFn: () => getItemAvailability(line.item.id),
      // Re-pull each time the cart re-renders (cheap, single-row sum).
      staleTime: 0,
    })),
  });
  /** Map item.id -> live available count (or `undefined` while loading). */
  const liveAvailable = useMemo(() => {
    const m = new Map<string, number>();
    lines.forEach((line, idx) => {
      const q = availabilityQueries[idx];
      if (q?.data) m.set(line.item.id, q.data.quantity_available);
    });
    return m;
  }, [lines, availabilityQueries]);

  useEffect(() => {
    if (!cameraActive) addInputRef.current?.focus();
  }, [cameraActive]);

  useEffect(() => {
    // Populate To Department with first available value if empty.
    if (!toDept && depts.data?.items.length) {
      setToDept(depts.data.items[0].id);
    }
  }, [depts.data, toDept]);

  function addOrIncrement(item: Item, scannedCode?: string) {
    setAddError(null);
    setLines((prev) => {
      const existing = prev.find((l) => l.item.id === item.id);
      if (existing) {
        return prev.map((l) =>
          l === existing
            ? {
                ...l,
                quantity: l.quantity + 1,
                scannedCodes: scannedCode ? [...l.scannedCodes, scannedCode] : l.scannedCodes,
                fromScan: l.fromScan || !!scannedCode,
              }
            : l,
        );
      }
      const free = Math.max(
        0,
        item.quantity_owned - item.quantity_damaged - item.quantity_lost,
      );
      return [
        ...prev,
        {
          rowId: newRowId(),
          item,
          quantity: 1,
          scannedCodes: scannedCode ? [scannedCode] : [],
          fromScan: !!scannedCode,
          freeSnapshot: free,
        },
      ];
    });
  }

  /** Barcode-only path (USB scanner + camera). */
  async function handleScan(raw: string) {
    const trimmed = raw.trim();
    if (!trimmed) return;
    setAddError(null);
    try {
      const item = await lookupItemByBarcode(trimmed);
      addOrIncrement(item, trimmed);
      setAddQuery('');
      addInputRef.current?.focus();
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setAddError(
          `No item is bound to barcode "${trimmed}". Assign it first via the Scan button in the sidebar.`,
        );
      } else if (err instanceof Error) {
        setAddError(err.message);
      } else {
        setAddError('Lookup failed.');
      }
    }
  }

  /**
   * Unified field: try barcode first, then (when not forced) resolve by name.
   */
  async function handleAddOrSearch() {
    const trimmed = addQuery.trim();
    if (!trimmed) return;
    setAddError(null);

    try {
      const item = await lookupItemByBarcode(trimmed);
      addOrIncrement(item, trimmed);
      setAddQuery('');
      addInputRef.current?.focus();
      return;
    } catch (err) {
      if (!(err instanceof ApiError && err.status === 404)) {
        setAddError(err instanceof Error ? err.message : 'Lookup failed.');
        return;
      }
    }

    if (forced) {
      setAddError(
        `No item is bound to barcode "${trimmed}". Assign it first via the Scan button in the sidebar.`,
      );
      return;
    }

    try {
      const res = await listItems({ q: trimmed, limit: 25 });
      if (res.items.length === 1) {
        addOrIncrement(res.items[0]);
        setAddQuery('');
        addInputRef.current?.focus();
        return;
      }
      if (res.items.length === 0) {
        setAddError('No item matches that barcode or name.');
        return;
      }
      setAddError('Several items match — pick one in the list below.');
    } catch (e) {
      setAddError(e instanceof Error ? e.message : 'Search failed.');
    }
  }

  function setQuantity(rowId: string, qty: number) {
    setLines((prev) =>
      prev
        .map((l) => (l.rowId === rowId ? { ...l, quantity: Math.max(0, Math.floor(qty)) } : l))
        .filter((l) => l.quantity > 0),
    );
  }
  function increment(rowId: string, delta: number) {
    setLines((prev) =>
      prev
        .map((l) => (l.rowId === rowId ? { ...l, quantity: l.quantity + delta } : l))
        .filter((l) => l.quantity > 0),
    );
  }
  function remove(rowId: string) {
    setLines((prev) => prev.filter((l) => l.rowId !== rowId));
  }

  const submit = useMutation({
    mutationFn: () =>
      createSignout({
        to_department_id: toDept,
        received_by_name: receivedBy.trim(),
        purpose: purpose.trim(),
        notes: notes.trim() || undefined,
        confirm: true,
        items: lines.map((l) => ({
          item_id: l.item.id,
          quantity: l.quantity,
          scanned_codes: l.scannedCodes.length ? l.scannedCodes : undefined,
        })),
      }),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['signouts'] });
      // The items the cart just signed out have a fresh availability —
      // bust their per-item caches so any other open page (and the cart
      // itself if the user comes back) gets the post-submit numbers.
      qc.invalidateQueries({ queryKey: ['item-availability'] });
      navigate({ to: '/signouts/$id', params: { id: data.id } });
    },
  });

  const errors = useMemo(() => {
    const list: string[] = [];
    if (!toDept) list.push('Pick a destination department.');
    if (!receivedBy.trim()) list.push('Type the recipient name.');
    if (lines.length === 0) list.push('Add at least one item.');
    if (forced) {
      const missing = lines.filter((l) => l.scannedCodes.length === 0);
      if (missing.length) {
        list.push(
          `Scan a barcode for: ${missing.map((m) => m.item.name).join(', ')}.`,
        );
      }
    }
    for (const l of lines) {
      // Prefer live availability (subtracts active sign-outs) over the
      // initial snapshot. While the live value is still loading, fall back
      // to the snapshot so the user can keep working — the server will
      // still reject any over-allocation on submit.
      const cap = liveAvailable.get(l.item.id) ?? l.freeSnapshot;
      if (l.quantity > cap) {
        list.push(
          `${l.item.name}: only ${cap} available (you asked for ${l.quantity}).`,
        );
      }
    }
    return list;
  }, [toDept, receivedBy, lines, forced, liveAvailable]);
  const valid = errors.length === 0;

  return (
    <PageContainer>
      <PageHeader
        title="New sign-out"
        description={
          forced
            ? 'Barcode scan required for every line. Items must already have a tag assigned.'
            : 'Use one field to scan or search. Enter tries the barcode first, then adds the item if there is exactly one name match. Submit confirms the sign-out instantly.'
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <ScanLine className="w-5 h-5" />
                Add to cart
              </CardTitle>
              <CardDescription>
                USB scanners and the camera both work. Scanning the same item twice increments quantity.
                {!forced ? ' Type at least two characters to see name matches, or press Enter to try barcode then name.' : null}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {cameraActive && cameraStream ? (
                <CameraScanner
                  wide
                  stream={cameraStream}
                  onResult={(text) => {
                    stopCamera();
                    void handleScan(text);
                  }}
                />
              ) : null}
              {cameraError ? (
                <p className="text-sm text-destructive" role="alert">
                  {cameraError}
                </p>
              ) : null}
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  void handleAddOrSearch();
                }}
                className="flex gap-2 items-end"
              >
                <div className="flex-1">
                  <Field
                    label={forced ? 'Barcode' : 'Barcode or name'}
                    htmlFor="cart-add"
                  >
                    <Input
                      id="cart-add"
                      ref={addInputRef}
                      value={addQuery}
                      onChange={(e) => {
                        setAddQuery(e.target.value);
                        if (addError) setAddError(null);
                      }}
                      placeholder={
                        forced ? 'ST000042' : 'Scan, type a code, or search by name…'
                      }
                      autoComplete="off"
                      spellCheck={false}
                      className="text-base h-11"
                    />
                  </Field>
                </div>
                <Button type="submit" disabled={!addQuery.trim()}>
                  Add
                </Button>
                <Button
                  type="button"
                  variant={cameraActive ? 'destructive' : 'outline'}
                  onClick={() => void toggleCamera()}
                >
                  {cameraActive ? <CameraOff className="w-4 h-4" /> : <Camera className="w-4 h-4" />}
                  {cameraActive ? 'Stop camera' : 'Camera'}
                </Button>
              </form>
              {addError ? (
                <p className="text-sm text-destructive" role="alert">
                  {addError}
                </p>
              ) : null}

              {!forced && qTrim.length >= 2 && items.data?.items.length ? (
                <div className="space-y-1 max-h-60 overflow-y-auto rounded-md border border-border p-1">
                  <p className="text-[11px] text-muted-foreground px-2 pt-1 pb-0.5">
                    Matches (tap to add — no scan recorded)
                  </p>
                  {items.data.items.map((it) => (
                    <button
                      key={it.id}
                      type="button"
                      className="w-full text-left text-sm px-2 py-1.5 rounded hover:bg-accent"
                      onClick={() => {
                        addOrIncrement(it);
                        setAddQuery('');
                        setAddError(null);
                      }}
                    >
                      <span className="font-medium">{it.name}</span>
                      {it.asset_tag ? (
                        <span className="ml-2 font-mono text-xs text-muted-foreground">
                          {it.asset_tag}
                        </span>
                      ) : null}
                    </button>
                  ))}
                </div>
              ) : null}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Cart ({lines.reduce((n, l) => n + l.quantity, 0)} units across {lines.length} lines)</CardTitle>
            </CardHeader>
            <CardContent>
              {lines.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  Empty. {forced ? 'Scan a barcode to start.' : 'Scan a code or search by name above.'}
                </p>
              ) : (
                <ul className="divide-y divide-border">
                  {lines.map((line) => {
                    const live = liveAvailable.get(line.item.id);
                    const cap = live ?? line.freeSnapshot;
                    return (
                    <li key={line.rowId} className="py-3 flex flex-wrap items-start gap-3">
                      <div className="flex-1 min-w-[12rem]">
                        <p className="font-medium">{line.item.name}</p>
                        <p className="text-xs text-muted-foreground">
                          {line.item.asset_tag ? (
                            <span className="font-mono">{line.item.asset_tag} · </span>
                          ) : null}
                          {live === undefined ? (
                            <span>{line.freeSnapshot} on hand · checking…</span>
                          ) : (
                            <span>
                              <span className={cn(line.quantity > cap && 'text-destructive font-medium')}>
                                {cap} available
                              </span>
                              {line.item.quantity_owned !== cap ? (
                                <span> of {line.item.quantity_owned} owned</span>
                              ) : null}
                            </span>
                          )}
                        </p>
                        <div className="mt-1 flex flex-wrap gap-1">
                          {line.scannedCodes.length ? (
                            line.scannedCodes.map((c, idx) => (
                              <Badge key={`${c}-${idx}`} variant="secondary" className="font-mono text-[10px]">
                                {c}
                              </Badge>
                            ))
                          ) : forced ? (
                            <Badge variant="destructive" className="text-[10px]">
                              <AlertTriangle className="w-3 h-3" />
                              Scan required
                            </Badge>
                          ) : (
                            <Badge variant="outline" className="text-[10px]">no scan</Badge>
                          )}
                        </div>
                      </div>
                      <div className="flex items-center gap-1">
                        <Button
                          variant="outline"
                          size="icon"
                          type="button"
                          onClick={() => increment(line.rowId, -1)}
                          aria-label="Decrease quantity"
                        >
                          <Minus className="w-4 h-4" />
                        </Button>
                        <Input
                          className="w-16 text-center"
                          type="number"
                          min={1}
                          max={cap}
                          value={line.quantity}
                          onChange={(e) => setQuantity(line.rowId, Number(e.target.value) || 0)}
                        />
                        <Button
                          variant="outline"
                          size="icon"
                          type="button"
                          onClick={() => increment(line.rowId, +1)}
                          aria-label="Increase quantity"
                        >
                          <Plus className="w-4 h-4" />
                        </Button>
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        type="button"
                        onClick={() => remove(line.rowId)}
                        aria-label="Remove line"
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </li>
                    );
                  })}
                </ul>
              )}
            </CardContent>
          </Card>
        </div>

        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Recipient</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <Field label="To department" htmlFor="cart-dept">
                <Select id="cart-dept" value={toDept} onChange={(e) => setToDept(e.target.value)}>
                  <option value="">-- pick department --</option>
                  {depts.data?.items.map((d) => (
                    <option key={d.id} value={d.id}>{d.name}</option>
                  ))}
                </Select>
              </Field>
              <Field label="Received by (person's name)" htmlFor="cart-recv">
                <Input
                  id="cart-recv"
                  value={receivedBy}
                  onChange={(e) => setReceivedBy(e.target.value)}
                  placeholder="e.g. Alice"
                />
              </Field>
              <Field label="Purpose (optional)" htmlFor="cart-purpose">
                <Input
                  id="cart-purpose"
                  value={purpose}
                  onChange={(e) => setPurpose(e.target.value)}
                  placeholder="e.g. urgent power for camera A"
                />
              </Field>
              <Field label="Notes (optional)" htmlFor="cart-notes">
                <Textarea
                  id="cart-notes"
                  rows={2}
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                />
              </Field>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Submit</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {errors.length ? (
                <ul className="text-sm text-warning list-disc list-inside">
                  {errors.map((e, idx) => (
                    <li key={idx}>{e}</li>
                  ))}
                </ul>
              ) : null}
              {submit.isError ? (
                <p className="text-sm text-destructive">
                  {submit.error instanceof ApiError ? submit.error.message : 'Failed to sign out.'}
                </p>
              ) : null}
              <Button
                onClick={() => submit.mutate()}
                disabled={!valid || submit.isPending}
                type="button"
                className="w-full"
              >
                <Send className="w-4 h-4" />
                {submit.isPending
                  ? 'Saving…'
                  : `Sign out ${lines.reduce((n, l) => n + l.quantity, 0)} units`}
              </Button>
            </CardContent>
          </Card>
        </div>
      </div>
    </PageContainer>
  );
}
