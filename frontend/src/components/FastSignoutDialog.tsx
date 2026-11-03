import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { ScanLine, Send } from 'lucide-react';
import { ApiError } from '@/api/client';
import { getItemAvailability } from '@/api/items';
import { createSignout } from '@/api/signouts';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Input, Select, Textarea } from '@/components/ui/Input';
import { useDepartments } from '@/hooks/useReferenceData';
import { useSettings } from '@/hooks/useSettings';
import { itemAvailable } from '@/lib/itemAvailability';
import type { Item, ItemAvailability } from '@/types';

type Props = {
  open: boolean;
  onClose: () => void;
  item: Item;
  availability?: ItemAvailability;
};

export function FastSignoutDialog({ open, onClose, item, availability }: Props) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const depts = useDepartments();
  const settings = useSettings();
  const forced = !!settings.data?.force_barcode_on_signout;
  const itemBarcodes = [item.asset_tag, item.qr_code, item.serial_number].filter(
    (v): v is string => !!v && v.length > 0,
  );

  const liveAvail = useQuery({
    queryKey: ['item-availability', item.id],
    queryFn: () => getItemAvailability(item.id),
    enabled: open && !availability,
    staleTime: 0,
  });

  const max = itemAvailable(
    item,
    availability?.quantity_available ?? liveAvail.data?.quantity_available,
  );
  const [toDept, setToDept] = useState('');
  const [receivedBy, setReceivedBy] = useState('');
  const [purpose, setPurpose] = useState('');
  const [quantity, setQuantity] = useState(1);
  const [scannedCode, setScannedCode] = useState('');
  const [scanError, setScanError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setToDept(depts.data?.items[0]?.id ?? '');
      setReceivedBy('');
      setPurpose('');
      setQuantity(Math.min(1, max) || 1);
      setScannedCode('');
      setScanError(null);
    }
  }, [open, depts.data, max]);

  const scannedCodeMatches =
    !!scannedCode && itemBarcodes.includes(scannedCode.trim());

  const create = useMutation({
    mutationFn: () =>
      createSignout({
        to_department_id: toDept,
        received_by_name: receivedBy,
        purpose,
        confirm: true,
        items: [
          {
            item_id: item.id,
            quantity,
            scanned_codes: scannedCodeMatches ? [scannedCode.trim()] : undefined,
          },
        ],
      }),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['item', item.id] });
      qc.invalidateQueries({ queryKey: ['item-availability'] });
      qc.invalidateQueries({ queryKey: ['signouts'] });
      onClose();
      navigate({ to: '/signouts/$id', params: { id: data.id } });
    },
  });

  const valid =
    !!toDept &&
    receivedBy.trim().length > 0 &&
    quantity > 0 &&
    quantity <= max &&
    (!forced || scannedCodeMatches);

  return (
    <Dialog open={open} onClose={onClose} title={`Sign out: ${item.name}`}>
      <div className="space-y-3">
        <p className="text-sm text-muted-foreground">
          {max > 0 ? `${max} available right now.` : 'Nothing available to sign out.'}
        </p>
        <Field label="To department">
          <Select value={toDept} onChange={(e) => setToDept(e.target.value)}>
            <option value="">-- pick department --</option>
            {depts.data?.items.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Received by (person's name)">
          <Input
            value={receivedBy}
            onChange={(e) => setReceivedBy(e.target.value)}
            placeholder="e.g. Alice"
            autoFocus
          />
        </Field>
        <Field label="Quantity">
          <Input
            type="number"
            min={1}
            max={max}
            value={quantity}
            onChange={(e) => setQuantity(Number(e.target.value))}
          />
        </Field>
        {forced || itemBarcodes.length > 0 ? (
          <Field
            label={forced ? 'Scan barcode (required)' : 'Scan barcode (optional, recorded for audit)'}
            hint={
              itemBarcodes.length === 0
                ? 'This item has no barcode yet — assign one before forcing scans.'
                : `Expected: ${itemBarcodes.join(' / ')}`
            }
          >
            <Input
              value={scannedCode}
              onChange={(e) => {
                setScannedCode(e.target.value);
                setScanError(null);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  if (!scannedCodeMatches && scannedCode.trim()) {
                    setScanError("Code doesn't match this item.");
                  }
                }
              }}
              placeholder="ST000042"
              className="font-mono"
            />
          </Field>
        ) : null}
        {scanError ? <p className="text-xs text-destructive">{scanError}</p> : null}
        <Field label="Purpose (optional)">
          <Textarea
            rows={2}
            value={purpose}
            onChange={(e) => setPurpose(e.target.value)}
            placeholder="e.g. urgent power for camera A"
          />
        </Field>
        {forced ? (
          <p className="text-xs text-muted-foreground flex items-start gap-1">
            <ScanLine className="w-3.5 h-3.5 mt-0.5" />
            Need multiple items? Use the
            <button
              type="button"
              className="underline mx-1"
              onClick={() => {
                onClose();
                navigate({ to: '/signouts/new' });
              }}
            >
              multi-item cart
            </button>
            instead.
          </p>
        ) : null}
        {create.isError ? (
          <p className="text-sm text-destructive">
            {create.error instanceof ApiError ? create.error.message : 'Failed to sign out.'}
          </p>
        ) : null}
        <div className="flex justify-end gap-2 mt-2">
          <Button variant="outline" onClick={onClose} type="button">
            Cancel
          </Button>
          <Button onClick={() => create.mutate()} disabled={!valid || create.isPending} type="button">
            <Send className="w-4 h-4" />
            {create.isPending ? 'Saving...' : 'Sign out'}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
