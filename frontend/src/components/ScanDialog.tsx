import { useEffect, useRef, useState } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Boxes, Camera, CameraOff, HandHeart, Plus, Printer, Send, Tag } from 'lucide-react';
import { ApiError } from '@/api/client';
import {
  getItemAvailability,
  listItems,
  lookupItemByBarcode,
  setItemBarcode,
} from '@/api/items';
import { lookupLostFoundByReportNumber } from '@/api/lostfound';
import {
  extractScanCode,
  isLostFoundReportNumber,
  normalizeLostFoundReportNumber,
} from '@/lib/scanCode';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Input, Select } from '@/components/ui/Input';
import { CameraScanner } from '@/components/CameraScanner';
import { BarcodeHelpNote } from '@/components/BarcodeHelpNote';
import { FastSignoutDialog } from '@/components/FastSignoutDialog';
import { Badge } from '@/components/ui/Badge';
import { LoadingRow } from '@/components/ui/States';
import { useCameraStream } from '@/hooks/useCameraStream';
import type { Item } from '@/types';

type LookupState =
  | { kind: 'idle' }
  | { kind: 'searching'; code: string }
  | { kind: 'found'; code: string; item: Item }
  | { kind: 'not_found'; code: string }
  | { kind: 'lf_not_found'; code: string }
  | { kind: 'error'; message: string };

/**
 * Global barcode lookup, openable from anywhere via the sidebar scan button.
 * Covers what the old /scan page did: look up a code, act on the item
 * (sign out / open / print label), or bind an unknown code to an item.
 */
export function ScanDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const navigate = useNavigate();
  const inputRef = useRef<HTMLInputElement>(null);
  const [code, setCode] = useState('');
  const [state, setState] = useState<LookupState>({ kind: 'idle' });
  const { active: cameraActive, stream: cameraStream, error: cameraError, stop: stopCamera, toggle: toggleCamera, clearError: clearCameraError } =
    useCameraStream();
  const [signoutItem, setSignoutItem] = useState<Item | null>(null);

  // Reset on every open/close so a stale result or camera stream never lingers.
  useEffect(() => {
    if (open) {
      setCode('');
      setState({ kind: 'idle' });
      clearCameraError();
      setSignoutItem(null);
    } else {
      stopCamera();
      clearCameraError();
    }
  }, [open, stopCamera, clearCameraError]);

  useEffect(() => {
    if (open && !cameraActive) inputRef.current?.focus();
  }, [open, cameraActive]);

  async function lookup(raw: string) {
    const trimmed = extractScanCode(raw);
    if (!trimmed) return;
    setState({ kind: 'searching', code: trimmed });

    if (isLostFoundReportNumber(trimmed)) {
      const reportNumber = normalizeLostFoundReportNumber(trimmed);
      try {
        const report = await lookupLostFoundByReportNumber(reportNumber);
        onClose();
        await navigate({ to: '/lost-and-found/$id', params: { id: report.id } });
        return;
      } catch (err) {
        if (err instanceof ApiError && err.status === 404) {
          setState({ kind: 'lf_not_found', code: reportNumber });
          return;
        }
        const message = err instanceof Error ? err.message : 'Lookup failed';
        setState({ kind: 'error', message });
        return;
      }
    }

    try {
      const item = await lookupItemByBarcode(trimmed);
      setState({ kind: 'found', code: trimmed, item });
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setState({ kind: 'not_found', code: trimmed });
        return;
      }
      const message = err instanceof Error ? err.message : 'Lookup failed';
      setState({ kind: 'error', message });
    }
  }

  return (
    <>
      <Dialog
        open={open}
        onClose={onClose}
        title="Scan barcode or QR code"
        description="Inventory codes (asset tags), QR labels or Lost & Found report numbers (LF-0003). USB scanners type + Enter automatically; on a phone, use the camera."
        className="max-w-6xl w-[min(100vw-2rem,72rem)]"
      >
        <div className="space-y-4">
          <BarcodeHelpNote />
          {cameraActive && cameraStream ? (
            <div className="-mx-5 px-5">
              <CameraScanner
                wide
                stream={cameraStream}
                onResult={(text) => {
                  stopCamera();
                  setCode(extractScanCode(text));
                  void lookup(text);
                }}
              />
            </div>
          ) : null}

          {cameraError ? (
            <p className="text-sm text-destructive" role="alert">
              {cameraError}
            </p>
          ) : null}

          <form
            onSubmit={(e) => {
              e.preventDefault();
              void lookup(code);
            }}
            className="flex gap-2 items-end"
          >
            <div className="flex-1">
              <Field label="Barcode" htmlFor="scan-dialog-input">
                <Input
                  id="scan-dialog-input"
                  ref={inputRef}
                  value={code}
                  onChange={(e) => {
                    setCode(e.target.value);
                    if (state.kind !== 'idle' && state.kind !== 'searching') {
                      setState({ kind: 'idle' });
                    }
                  }}
                  placeholder="ST000042 or LF-0003"
                  autoComplete="off"
                  spellCheck={false}
                  className="font-mono text-base h-11"
                />
              </Field>
            </div>
            <Button type="submit" size="lg" disabled={state.kind === 'searching' || !code.trim()}>
              {state.kind === 'searching' ? 'Searching…' : 'Look up'}
            </Button>
            <Button
              type="button"
              size="lg"
              variant={cameraActive ? 'destructive' : 'outline'}
              onClick={() => void toggleCamera()}
              aria-label={cameraActive ? 'Stop camera' : 'Use camera'}
            >
              {cameraActive ? <CameraOff className="w-4 h-4" /> : <Camera className="w-4 h-4" />}
            </Button>
          </form>

          {state.kind === 'error' ? (
            <p className="text-sm text-destructive" role="alert">
              {state.message}
            </p>
          ) : null}

          {state.kind === 'found' ? (
            <FoundResult
              item={state.item}
              onSignout={() => setSignoutItem(state.item)}
              onClose={onClose}
            />
          ) : null}

          {state.kind === 'lf_not_found' ? (
            <div className="rounded-lg border border-border bg-secondary/50 p-4 space-y-2">
              <p className="font-heading font-bold flex items-center gap-2">
                <HandHeart className="w-4 h-4" />
                No report <span className="font-mono">{state.code}</span>
              </p>
              <p className="text-sm text-muted-foreground">
                That label isn&apos;t in Stash — it may have been returned, closed, or never logged.
              </p>
            </div>
          ) : null}

          {state.kind === 'not_found' ? (
            <NotFoundResult
              code={state.code}
              onAssigned={(item) => setState({ kind: 'found', code: state.code, item })}
              onClose={onClose}
            />
          ) : null}
        </div>
      </Dialog>

      {signoutItem ? (
        <FastSignoutDialog
          open
          onClose={() => {
            setSignoutItem(null);
            onClose();
          }}
          item={signoutItem}
        />
      ) : null}
    </>
  );
}

function FoundResult({
  item,
  onSignout,
  onClose,
}: {
  item: Item;
  onSignout: () => void;
  onClose: () => void;
}) {
  const navigate = useNavigate();
  const avail = useQuery({
    queryKey: ['item-availability', item.id],
    queryFn: () => getItemAvailability(item.id),
    staleTime: 0,
  });
  const onHand = Math.max(0, item.quantity_owned - item.quantity_damaged - item.quantity_lost);
  const free = avail.data?.quantity_available ?? onHand;
  const signedOut = avail.data?.quantity_signed_out ?? 0;

  return (
    <div className="rounded-lg border border-border bg-secondary/50 p-4 space-y-3">
      <div>
        <p className="font-heading font-bold">{item.name}</p>
        <p className="text-xs text-muted-foreground">
          {item.asset_tag ? <span className="font-mono">{item.asset_tag} · </span> : null}
          {item.quantity_owned} owned
          {signedOut > 0 ? ` · ${signedOut} signed out` : ''}
          {' · '}
          <span className="font-semibold text-foreground">
            {avail.isLoading ? 'checking…' : `${free} available`}
          </span>
        </p>
        {item.description ? (
          <p className="text-sm text-muted-foreground mt-1">{item.description}</p>
        ) : null}
      </div>
      {item.item_type === 'container' || item.item_type === 'grouped' ? (
        <ScanContentsPreview
          parentId={item.id}
          mode={item.item_type === 'grouped' ? 'grouped' : 'container'}
        />
      ) : null}
      <div className="flex flex-wrap gap-2">
        <Button onClick={onSignout} disabled={free <= 0} title={free <= 0 ? 'Nothing available to sign out' : undefined}>
          <Send className="w-4 h-4" />
          Sign out
        </Button>
        <Button
          variant="secondary"
          onClick={() => {
            onClose();
            navigate({ to: '/items/$id', params: { id: item.id } }).catch(() => undefined);
          }}
        >
          Open item
        </Button>
        <Button
          variant="secondary"
          onClick={() => window.open(`/items/${item.id}/label`, '_blank', 'noopener')}
        >
          <Printer className="w-4 h-4" />
          Print label
        </Button>
      </div>
    </div>
  );
}

function ScanContentsPreview({
  parentId,
  mode,
}: {
  parentId: string;
  mode: 'container' | 'grouped';
}) {
  const isGrouped = mode === 'grouped';
  const contents = useQuery({
    queryKey: ['scan-contents-preview', mode, parentId],
    queryFn: () =>
      listItems(
        isGrouped
          ? { parent_group_id: parentId, limit: 200 }
          : { parent_container_id: parentId, limit: 200 }
      ),
  });

  const containers = useQuery({
    queryKey: ['containers-lookup'],
    queryFn: () => listItems({ item_type: 'container', limit: 500 }),
    enabled: isGrouped,
  });

  const containerNameById = new Map(
    (containers.data?.items ?? []).map((c) => [c.id, c.name] as const)
  );

  const totalOwned = (contents.data?.items ?? []).reduce((sum, row) => sum + row.quantity_owned, 0);

  return (
    <div className="rounded-md border border-border bg-card/80 overflow-hidden">
      <div className="flex items-center justify-between gap-2 px-3 py-2 border-b border-border bg-muted/40">
        <p className="text-sm font-semibold flex items-center gap-2">
          <Boxes className="w-4 h-4 shrink-0" />
          {isGrouped ? 'Sizes in group' : 'Contents'}
        </p>
        {contents.data ? (
          <p className="text-xs text-muted-foreground tabular-nums">
            {contents.data.total} {contents.data.total === 1 ? 'row' : 'rows'}
            {totalOwned > 0 ? ` · ${totalOwned} total` : ''}
          </p>
        ) : null}
      </div>
      {contents.isLoading ? (
        <div className="p-3">
          <LoadingRow />
        </div>
      ) : contents.isError ? (
        <p className="p-3 text-sm text-destructive">Failed to load contents.</p>
      ) : !contents.data?.items.length ? (
        <p className="p-3 text-sm text-muted-foreground">
          {isGrouped ? 'No size variants in this group yet.' : 'This container is empty.'}
        </p>
      ) : (
        <div className="max-h-56 overflow-y-auto">
          <table className="w-full text-sm">
            <thead className="sticky top-0 bg-card z-[1]">
              <tr className="border-b border-border text-left text-xs text-muted-foreground">
                {isGrouped ? <th className="py-2 px-3 font-medium">Size</th> : null}
                <th className="py-2 px-3 font-medium">Name</th>
                {isGrouped ? <th className="py-2 px-3 font-medium">Container</th> : null}
                {!isGrouped ? <th className="py-2 px-3 font-medium">Type</th> : null}
                <th className="py-2 px-3 font-medium text-right">Qty</th>
                <th className="py-2 px-3 font-medium">Status</th>
              </tr>
            </thead>
            <tbody>
              {contents.data.items.map((row) => (
                <tr key={row.id} className="border-b border-border/50 last:border-0">
                  {isGrouped ? (
                    <td className="py-2 px-3 font-medium whitespace-nowrap">{row.size || '—'}</td>
                  ) : null}
                  <td className="py-2 px-3">
                    <Link to="/items/$id" params={{ id: row.id }} className="font-medium hover:underline">
                      {row.name}
                    </Link>
                  </td>
                  {isGrouped ? (
                    <td className="py-2 px-3 text-muted-foreground whitespace-nowrap">
                      {row.parent_container_id
                        ? containerNameById.get(row.parent_container_id) ?? 'Container'
                        : '—'}
                    </td>
                  ) : null}
                  {!isGrouped ? (
                    <td className="py-2 px-3">
                      <Badge variant="secondary" className="text-[10px]">
                        {row.item_type}
                      </Badge>
                    </td>
                  ) : null}
                  <td className="py-2 px-3 text-right tabular-nums font-medium">{row.quantity_owned}</td>
                  <td className="py-2 px-3">
                    <Badge variant={row.status === 'retired' ? 'outline' : 'success'} className="text-[10px]">
                      {row.status}
                    </Badge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function NotFoundResult({
  code,
  onAssigned,
  onClose,
}: {
  code: string;
  onAssigned: (item: Item) => void;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [pickedItemId, setPickedItemId] = useState('');
  const [search, setSearch] = useState('');

  const items = useQuery({
    queryKey: ['scan-pick', search],
    queryFn: () => listItems({ q: search || undefined, limit: 25 }),
  });

  const assign = useMutation({
    mutationFn: () => setItemBarcode(pickedItemId, code),
    onSuccess: (item) => {
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['item', item.id] });
      onAssigned(item);
    },
  });

  return (
    <div className="rounded-lg border border-border bg-secondary/50 p-4 space-y-4">
      <div>
        <p className="font-heading font-bold flex items-center gap-2">
          <Tag className="w-4 h-4" />
          No item with code <span className="font-mono">{code}</span>
        </p>
        <p className="text-sm text-muted-foreground mt-1">
          Pre-printed sticker? Assign it to an existing item — or create a new item with this code.
        </p>
      </div>
      <div className="space-y-2">
        <Field label="Search items">
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="name, serial, barcode…"
          />
        </Field>
        <Field label="Assign to">
          <Select value={pickedItemId} onChange={(e) => setPickedItemId(e.target.value)}>
            <option value="">-- pick an item --</option>
            {items.data?.items.map((it) => (
              <option key={it.id} value={it.id}>
                {it.name}
                {it.asset_tag ? ` (currently ${it.asset_tag})` : ''}
              </option>
            ))}
          </Select>
        </Field>
        <Button onClick={() => assign.mutate()} disabled={!pickedItemId || assign.isPending}>
          <Tag className="w-4 h-4" />
          {assign.isPending ? 'Saving…' : `Assign ${code}`}
        </Button>
        {assign.isError ? (
          <p className="text-sm text-destructive">
            {assign.error instanceof ApiError ? assign.error.message : 'Failed to assign barcode.'}
          </p>
        ) : null}
      </div>
      <div className="border-t border-border pt-3">
        <Button
          variant="secondary"
          onClick={() => {
            onClose();
            navigate({ to: '/items/new', search: { asset_tag: code } }).catch(() => undefined);
          }}
        >
          <Plus className="w-4 h-4" />
          Create new item with this code
        </Button>
      </div>
    </div>
  );
}
