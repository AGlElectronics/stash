import { useEffect, useMemo } from 'react';
import { Controller, useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { listItems } from '@/api/items';
import { Field, Input, Select, Textarea } from '@/components/ui/Input';
import { Button } from '@/components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card';
import { CreatableSelect, type CreatablePick } from '@/components/ui/CreatableSelect';
import { useCategories, useDepartments, useLocations } from '@/hooks/useReferenceData';
import { buildLocationPathMap } from '@/lib/locationPath';
import { creatablePickFromDom } from '@/lib/resolveCreatableRef';

const refPickSchema: z.ZodType<CreatablePick, CreatablePick> = z.object({
  id: z.uuid().optional(),
  name: z.string(),
});

const itemSchema = z.object({
  name: z.string().min(1, { error: 'Name is required' }),
  description: z.string().optional(),
  category: refPickSchema,
  item_type: z.enum(['asset', 'consumable', 'container', 'grouped']),
  size: z.string().optional(),
  quantity_owned: z.coerce.number().int().min(0),
  unit: z.string().optional(),
  condition: z.string().optional(),
  location: refPickSchema,
  parent_container_id: z.string().optional(),
  managed_by_department: refPickSchema,
  needed_by_department: refPickSchema,
  serial_number: z.string().optional(),
  asset_tag: z.string().optional(),
  qr_code: z.string().optional(),
  owner: z.string().optional(),
  purchase_price: z.string().optional(),
  purchase_currency: z.string().optional(),
  purchase_date: z.string().optional(),
  expiry_date: z.string().optional(),
  supplier: z.string().optional(),
  purchase_url: z.string().optional(),
  replacement_cost: z.string().optional(),
  minimum_quantity: z.coerce.number().int().min(0).optional(),
  notes: z.string().optional(),
});

export type ItemFormValues = z.output<typeof itemSchema>;
/** What the inputs hold before validation; the coerced numbers can still be strings. */
type ItemFormInput = z.input<typeof itemSchema>;

type Props = {
  defaultValues?: Partial<ItemFormValues>;
  onSubmit: (values: ItemFormValues) => Promise<void> | void;
  submitLabel?: string;
  busy?: boolean;
  onCancel?: () => void;
};

export function ItemForm({ defaultValues, onSubmit, submitLabel = 'Save', busy, onCancel }: Props) {
  const cats = useCategories();
  const depts = useDepartments();
  const locs = useLocations();

  const containers = useQuery({
    queryKey: ['containers-lookup'],
    queryFn: () => listItems({ item_type: 'container', limit: 500 }),
  });

  const locationOptions = useMemo(() => {
    const items = locs.data?.items ?? [];
    const paths = buildLocationPathMap(items);
    return items.map((l) => ({ id: l.id, name: paths.get(l.id) ?? l.name }));
  }, [locs.data?.items]);

  const form = useForm<ItemFormInput, unknown, ItemFormValues>({
    resolver: zodResolver(itemSchema),
    defaultValues: {
      item_type: 'asset',
      quantity_owned: 1,
      unit: 'pcs',
      condition: 'good',
      purchase_currency: 'EUR',
      minimum_quantity: 0,
      category: { name: '' },
      managed_by_department: { name: '' },
      needed_by_department: { name: '' },
      location: { name: '' },
      ...defaultValues,
    } as ItemFormInput,
  });

  const itemType = useWatch({ control: form.control, name: 'item_type' });
  const isGrouped = itemType === 'grouped';
  const showContainer = itemType === 'asset' || itemType === 'consumable';

  useEffect(() => {
    if (isGrouped) {
      form.setValue('quantity_owned', 0, { shouldDirty: true });
    }
  }, [isGrouped, form]);

  useEffect(() => {
    const dirty = form.formState.dirtyFields;
    const fill = (
      key: 'category' | 'managed_by_department' | 'needed_by_department' | 'location',
      list: { id: string; name: string }[] | undefined,
      idHint: string | undefined
    ) => {
      if (dirty[key]) return;
      if (!list?.length || !idHint) return;
      const cur = form.getValues(key);
      if (cur.name.trim()) return;
      const row = list.find((x) => x.id === idHint);
      if (row) form.setValue(key, { id: row.id, name: row.name }, { shouldDirty: false });
    };
    fill('category', cats.data?.items, defaultValues?.category?.id);
    fill('managed_by_department', depts.data?.items, defaultValues?.managed_by_department?.id);
    fill('needed_by_department', depts.data?.items, defaultValues?.needed_by_department?.id);
    fill('location', locationOptions, defaultValues?.location?.id);
    // form is stable from useForm; we only refill when reference data / default ids change.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentional deps above
  }, [
    cats.data,
    depts.data,
    locationOptions,
    defaultValues?.category?.id,
    defaultValues?.managed_by_department?.id,
    defaultValues?.needed_by_department?.id,
    defaultValues?.location?.id,
    form.formState.dirtyFields,
  ]);

  const syncCreatablesFromDom = () => {
    const deptOpts = depts.data?.items ?? [];
    const catOpts = cats.data?.items ?? [];
    const keys = [
      ['category', 'category', catOpts],
      ['managed_by_department', 'managed_by_department', deptOpts],
      ['needed_by_department', 'needed_by_department', deptOpts],
      ['location', 'location', locationOptions],
    ] as const;
    for (const [key, inputId, opts] of keys) {
      const pick = creatablePickFromDom(inputId, form.getValues(key), opts);
      form.setValue(key, pick, { shouldDirty: true, shouldValidate: true });
    }
  };

  const submit = form.handleSubmit(async (values) => {
    syncCreatablesFromDom();
    const committed = form.getValues();
    const deptOpts = depts.data?.items ?? [];
    const catOpts = cats.data?.items ?? [];

    const sanitized: ItemFormValues = {
      ...values,
      category: creatablePickFromDom('category', committed.category, catOpts),
      managed_by_department: creatablePickFromDom(
        'managed_by_department',
        committed.managed_by_department,
        deptOpts
      ),
      needed_by_department: creatablePickFromDom(
        'needed_by_department',
        committed.needed_by_department,
        deptOpts
      ),
      location: creatablePickFromDom('location', committed.location, locationOptions),
    };
    (Object.keys(sanitized) as Array<keyof ItemFormValues>).forEach((k) => {
      if (typeof sanitized[k] === 'string' && sanitized[k] === '') {
        delete (sanitized as Record<string, unknown>)[k];
      }
    });
    await onSubmit(sanitized);
  });

  const fieldError = (k: keyof ItemFormValues) => {
    const e = form.formState.errors[k];
    if (!e) return undefined;
    if ('message' in e && typeof e.message === 'string') return e.message;
    const record = e as Record<string, { message?: string }>;
    for (const sub of Object.values(record)) {
      if (sub && typeof sub.message === 'string') return sub.message;
    }
    return undefined;
  };

  return (
    <form onSubmitCapture={syncCreatablesFromDom} onSubmit={submit} className="space-y-5 max-w-3xl">
      <Card>
        <CardHeader>
          <CardTitle>Basics</CardTitle>
          <CardDescription>What is it, and what kind of thing is it?</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="md:col-span-2">
            <Field label="Name" htmlFor="name" error={fieldError('name')}>
              <Input id="name" {...form.register('name')} placeholder="Power strip 6-way" />
            </Field>
          </div>
          <Field label="Type" htmlFor="item_type" error={fieldError('item_type')}>
            <Select id="item_type" {...form.register('item_type')}>
              <option value="asset">Asset (tracked individually)</option>
              <option value="consumable">Consumable (used up)</option>
              <option value="container">Container (holds other items)</option>
              <option value="grouped">Grouped (holds sizes)</option>
            </Select>
          </Field>
          <Field label="Category" htmlFor="category" error={fieldError('category')}>
            <Controller
              control={form.control}
              name="category"
              render={({ field }) => (
                <CreatableSelect
                  id="category"
                  options={cats.data?.items ?? []}
                  value={field.value}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  entityLabel="category"
                />
              )}
            />
          </Field>
          <div className="md:col-span-2">
            <Field label="Description" htmlFor="description">
              <Textarea id="description" rows={2} {...form.register('description')} />
            </Field>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Departments &amp; storage</CardTitle>
          <CardDescription>Who looks after it, who needs it, and where it lives.</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Field label="Managed by department" htmlFor="managed_by_department">
            <Controller
              control={form.control}
              name="managed_by_department"
              render={({ field }) => (
                <CreatableSelect
                  id="managed_by_department"
                  options={depts.data?.items ?? []}
                  value={field.value}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  entityLabel="department"
                />
              )}
            />
          </Field>
          <Field label="Needed by department" htmlFor="needed_by_department">
            <Controller
              control={form.control}
              name="needed_by_department"
              render={({ field }) => (
                <CreatableSelect
                  id="needed_by_department"
                  options={depts.data?.items ?? []}
                  value={field.value}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  entityLabel="department"
                />
              )}
            />
          </Field>
          <div className="md:col-span-2">
            <Field label="Location" htmlFor="location">
              <Controller
                control={form.control}
                name="location"
                render={({ field }) => (
                  <CreatableSelect
                    id="location"
                    options={locationOptions}
                    value={field.value}
                    onChange={field.onChange}
                    onBlur={field.onBlur}
                    entityLabel="location"
                  />
                )}
              />
            </Field>
          </div>
          {showContainer ? (
            <div className="md:col-span-2">
              <Field
                label="Container"
                htmlFor="parent_container_id"
                hint="Optional — store this item inside a container"
              >
                <Select id="parent_container_id" {...form.register('parent_container_id')}>
                  <option value="">None</option>
                  {(containers.data?.items ?? []).map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </Select>
              </Field>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Stock</CardTitle>
          <CardDescription>
            {isGrouped
              ? 'Grouped items track stock per size variant — add sizes on the detail page after saving.'
              : 'How many we own and when to flag low stock on the dashboard.'}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {!isGrouped ? (
            <Field label="Quantity owned" htmlFor="quantity_owned" error={fieldError('quantity_owned')}>
              <Input id="quantity_owned" type="number" min={0} step={1} {...form.register('quantity_owned', { valueAsNumber: true })} />
            </Field>
          ) : null}
          {!isGrouped && (itemType === 'asset' || itemType === 'consumable') ? (
            <Field label="Size" htmlFor="size" hint="For size variants inside a grouped item">
              <Input id="size" {...form.register('size')} placeholder="S, M, L…" />
            </Field>
          ) : null}
          <Field label="Unit" htmlFor="unit" error={fieldError('unit')}>
            <Input id="unit" {...form.register('unit')} placeholder="pcs" />
          </Field>
          <Field label="Condition" htmlFor="condition" error={fieldError('condition')}>
            <Select id="condition" {...form.register('condition')}>
              <option value="good">Good</option>
              <option value="fair">Fair</option>
              <option value="damaged">Damaged</option>
              <option value="new">New</option>
            </Select>
          </Field>
          <Field label="Minimum quantity" htmlFor="minimum_quantity">
            <Input id="minimum_quantity" type="number" min={0} step={1} {...form.register('minimum_quantity', { valueAsNumber: true })} />
          </Field>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Identification</CardTitle>
          <CardDescription>Codes that let the scanner find this exact item.</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Field label="Serial number" htmlFor="serial_number">
            <Input id="serial_number" {...form.register('serial_number')} />
          </Field>
          <Field
            label="Barcode / asset tag"
            htmlFor="asset_tag"
            hint="Leave blank to auto-assign the next AW code on save. Used for barcode lookup."
          >
            <Input id="asset_tag" className="font-mono" {...form.register('asset_tag')} />
          </Field>
          <Field label="QR code" htmlFor="qr_code">
            <Input id="qr_code" {...form.register('qr_code')} />
          </Field>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Purchase &amp; ownership</CardTitle>
          <CardDescription>Optional bookkeeping — handy for insurance and replacements.</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Field label="Owner" htmlFor="owner">
            <Input id="owner" {...form.register('owner')} placeholder="organisation / owner" />
          </Field>
          <Field label="Purchase price" htmlFor="purchase_price">
            <Input id="purchase_price" {...form.register('purchase_price')} placeholder="14.95" />
          </Field>
          <Field label="Currency" htmlFor="purchase_currency">
            <Input id="purchase_currency" {...form.register('purchase_currency')} placeholder="EUR" />
          </Field>
          <Field label="Purchase date" htmlFor="purchase_date">
            <Input id="purchase_date" type="date" {...form.register('purchase_date')} />
          </Field>
          <Field label="Expiry date" htmlFor="expiry_date" hint="For consumables with a use-by date">
            <Input id="expiry_date" type="date" {...form.register('expiry_date')} />
          </Field>
          <Field label="Supplier" htmlFor="supplier">
            <Input id="supplier" {...form.register('supplier')} />
          </Field>
          <Field label="Purchase URL" htmlFor="purchase_url">
            <Input id="purchase_url" type="url" {...form.register('purchase_url')} />
          </Field>
          <Field label="Replacement cost" htmlFor="replacement_cost">
            <Input id="replacement_cost" {...form.register('replacement_cost')} />
          </Field>
          <div className="md:col-span-3">
            <Field label="Notes" htmlFor="notes">
              <Textarea id="notes" rows={3} {...form.register('notes')} />
            </Field>
          </div>
        </CardContent>
      </Card>

      <div className="flex gap-2 justify-end">
        {onCancel ? (
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
        ) : null}
        <Button type="submit" size="lg" disabled={busy || form.formState.isSubmitting}>
          {busy || form.formState.isSubmitting ? 'Saving…' : submitLabel}
        </Button>
      </div>
    </form>
  );
}
