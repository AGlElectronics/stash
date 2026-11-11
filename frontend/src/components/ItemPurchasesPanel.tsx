import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { ApiError } from '@/api/client';
import { listPurchases, recordPurchase, type PurchaseInput } from '@/api/planning';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Input } from '@/components/ui/Input';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { PERMS, useMe } from '@/hooks/useMe';

const emptyForm = (): PurchaseInput => ({
  purchased_on: new Date().toISOString().slice(0, 10),
  quantity: 1,
  unit_price: '',
  currency: 'EUR',
  supplier: '',
  reference: '',
  note: '',
  add_to_stock: true,
});

/** What was bought for an item, and recording a new purchase (item.adjust). */
export function ItemPurchasesPanel({ itemId }: { itemId: string }) {
  const qc = useQueryClient();
  const { can } = useMe();
  const purchases = useQuery({ queryKey: ['purchases', itemId], queryFn: () => listPurchases(itemId) });
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<PurchaseInput>(emptyForm);

  const save = useMutation({
    mutationFn: () => recordPurchase(itemId, form),
    onSuccess: () => {
      setOpen(false);
      setForm(emptyForm());
      qc.invalidateQueries({ queryKey: ['purchases', itemId] });
      qc.invalidateQueries({ queryKey: ['item', itemId] });
    },
  });
  const set = <K extends keyof PurchaseInput>(k: K, v: PurchaseInput[K]) => setForm({ ...form, [k]: v });

  return (
    <div className="space-y-3">
      {purchases.data?.items.length ? (
        <Table>
          <THead>
            <TR>
              <TH>Date</TH>
              <TH>Qty</TH>
              <TH>Unit price</TH>
              <TH>Supplier</TH>
              <TH>Reference</TH>
            </TR>
          </THead>
          <TBody>
            {purchases.data.items.map((p) => (
              <TR key={p.id}>
                <TD className="whitespace-nowrap">{p.purchased_on}</TD>
                <TD className="tabular-nums">
                  {p.quantity}
                  {p.added_to_stock ? <span className="text-xs text-muted-foreground"> (added to stock)</span> : null}
                </TD>
                <TD className="tabular-nums">{p.unit_price ? `${p.unit_price} ${p.currency}` : '—'}</TD>
                <TD>{p.supplier || '—'}</TD>
                <TD>{p.reference || '—'}</TD>
              </TR>
            ))}
          </TBody>
        </Table>
      ) : (
        <p className="text-sm text-muted-foreground">{purchases.isLoading ? 'Loading…' : 'No purchases recorded.'}</p>
      )}
      {can(PERMS.itemAdjust) ? (
        <Button variant="outline" onClick={() => setOpen(true)}>
          <Plus className="w-4 h-4" />
          Record purchase
        </Button>
      ) : null}

      <Dialog open={open} onClose={() => setOpen(false)} title="Record purchase">
        <form
          className="grid grid-cols-1 sm:grid-cols-2 gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            save.mutate();
          }}
        >
          <Field label="Date" htmlFor="p-date">
            <Input id="p-date" type="date" value={form.purchased_on} onChange={(e) => set('purchased_on', e.target.value)} />
          </Field>
          <Field label="Quantity" htmlFor="p-qty">
            <Input id="p-qty" type="number" min={1} value={form.quantity} onChange={(e) => set('quantity', Number(e.target.value))} />
          </Field>
          <Field label="Unit price" htmlFor="p-price">
            <Input id="p-price" inputMode="decimal" value={form.unit_price} onChange={(e) => set('unit_price', e.target.value)} />
          </Field>
          <Field label="Currency" htmlFor="p-cur">
            <Input id="p-cur" maxLength={3} value={form.currency} onChange={(e) => set('currency', e.target.value.toUpperCase())} />
          </Field>
          <Field label="Supplier" htmlFor="p-sup">
            <Input id="p-sup" value={form.supplier} onChange={(e) => set('supplier', e.target.value)} />
          </Field>
          <Field label="Invoice / order number" htmlFor="p-ref">
            <Input id="p-ref" value={form.reference} onChange={(e) => set('reference', e.target.value)} />
          </Field>
          <label className="sm:col-span-2 flex items-center gap-2 text-sm">
            <input type="checkbox" checked={form.add_to_stock} onChange={(e) => set('add_to_stock', e.target.checked)} />
            Add the quantity to stock
          </label>
          {save.isError ? (
            <p role="alert" className="sm:col-span-2 text-sm text-destructive">
              {save.error instanceof ApiError ? save.error.message : 'Saving failed.'}
            </p>
          ) : null}
          <div className="sm:col-span-2 flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={save.isPending}>
              Save
            </Button>
          </div>
        </form>
      </Dialog>
    </div>
  );
}
