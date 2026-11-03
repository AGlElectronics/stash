import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  adjustItem,
  damageItem,
  foundItem,
  lostItem,
  moveItem,
  repairItem,
  restoreItem,
  retireItem,
} from '@/api/items';
import { ApiError } from '@/api/client';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Input, Select, Textarea } from '@/components/ui/Input';
import { useLocations } from '@/hooks/useReferenceData';

type Action =
  | 'move'
  | 'adjust'
  | 'damage'
  | 'repair'
  | 'lost'
  | 'found'
  | 'retire'
  | 'restore';

const TITLES: Record<Action, string> = {
  move: 'Move item',
  adjust: 'Adjust quantity',
  damage: 'Mark damaged',
  repair: 'Mark repaired',
  lost: 'Mark lost',
  found: 'Mark found',
  retire: 'Retire item',
  restore: 'Restore item',
};

type Props = {
  open: boolean;
  action: Action | null;
  itemId: string;
  onClose: () => void;
};

export function ItemActionDialog({ open, action, itemId, onClose }: Props) {
  const qc = useQueryClient();
  const locs = useLocations();

  const [quantity, setQuantity] = useState(1);
  const [delta, setDelta] = useState(1);
  const [locationId, setLocationId] = useState('');
  const [clearLocation, setClearLocation] = useState(false);
  const [note, setNote] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const reset = () => {
    setQuantity(1);
    setDelta(1);
    setLocationId('');
    setClearLocation(false);
    setNote('');
    setBusy(false);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const submit = async () => {
    if (!action) return;
    setBusy(true);
    setError(null);
    try {
      switch (action) {
        case 'move':
          await moveItem(itemId, {
            to_location_id: locationId || undefined,
            clear_location: clearLocation,
            note,
          });
          break;
        case 'adjust':
          await adjustItem(itemId, { delta, note });
          break;
        case 'damage':
          await damageItem(itemId, { quantity, note });
          break;
        case 'repair':
          await repairItem(itemId, { quantity, note });
          break;
        case 'lost':
          await lostItem(itemId, { quantity, note });
          break;
        case 'found':
          await foundItem(itemId, { quantity, note });
          break;
        case 'retire':
          await retireItem(itemId, { note });
          break;
        case 'restore':
          await restoreItem(itemId, { note });
          break;
      }
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['item', itemId] });
      qc.invalidateQueries({ queryKey: ['item-history', itemId] });
      qc.invalidateQueries({ queryKey: ['item-availability', itemId] });
      handleClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Action failed');
      setBusy(false);
    }
  };

  return (
    <Dialog open={open && !!action} onClose={handleClose} title={action ? TITLES[action] : ''}>
      {action === 'move' ? (
        <div className="space-y-3">
          <Field label="New location">
            <Select
              value={locationId}
              onChange={(e) => {
                setLocationId(e.target.value);
                setClearLocation(false);
              }}
            >
              <option value="">-- choose --</option>
              {locs.data?.items.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.name}
                </option>
              ))}
            </Select>
          </Field>
          <label className="flex items-center gap-2 text-sm text-muted-foreground">
            <input
              type="checkbox"
              checked={clearLocation}
              onChange={(e) => {
                setClearLocation(e.target.checked);
                if (e.target.checked) setLocationId('');
              }}
            />
            Clear location (no specific spot)
          </label>
        </div>
      ) : null}

      {action === 'adjust' ? (
        <Field label="Delta (use negative to remove)">
          <Input type="number" value={delta} onChange={(e) => setDelta(Number(e.target.value))} />
        </Field>
      ) : null}

      {(action === 'damage' || action === 'repair' || action === 'lost' || action === 'found') ? (
        <Field label="Quantity">
          <Input
            type="number"
            min={1}
            value={quantity}
            onChange={(e) => setQuantity(Number(e.target.value))}
          />
        </Field>
      ) : null}

      <Field label="Note">
        <Textarea rows={3} value={note} onChange={(e) => setNote(e.target.value)} />
      </Field>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <div className="flex justify-end gap-2 mt-4">
        <Button variant="outline" onClick={handleClose} type="button">
          Cancel
        </Button>
        <Button onClick={submit} disabled={busy} type="button">
          {busy ? 'Saving...' : 'Confirm'}
        </Button>
      </div>
    </Dialog>
  );
}
