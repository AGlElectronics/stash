import { useMemo, useState } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@/api/client';
import { uploadFile } from '@/api/files';
import { createItem, getItem, type CreateItemBody } from '@/api/items';
import { PendingItemPhotos } from '@/components/PendingItemPhotos';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card';
import { ItemForm, type ItemFormValues } from '@/forms/ItemForm';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { itemFormRefFields } from '@/lib/resolveCreatableRef';

export function ItemNewPage() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const search = useSearch({ from: '/app/items/new' });
  const [serverError, setServerError] = useState<string | null>(null);
  const [pendingPhotos, setPendingPhotos] = useState<File[]>([]);

  const cloneSource = useQuery({
    queryKey: ['item', search.clone_from],
    queryFn: () => getItem(search.clone_from!),
    enabled: !!search.clone_from,
  });

  const cloneDefaults = useMemo<Partial<ItemFormValues> | undefined>(() => {
    const src = cloneSource.data;
    if (!src) return undefined;
    return {
      name: `${src.name} (copy)`,
      description: src.description,
      item_type: src.item_type as ItemFormValues['item_type'],
      quantity_owned: src.quantity_owned,
      unit: src.unit,
      condition: src.condition,
      owner: src.owner,
      purchase_currency: src.purchase_currency,
      minimum_quantity: src.minimum_quantity,
      notes: src.notes,
      category: src.category_id ? { id: src.category_id, name: '' } : { name: '' },
      managed_by_department: src.managed_by_department_id
        ? { id: src.managed_by_department_id, name: '' }
        : { name: '' },
      needed_by_department: src.needed_by_department_id
        ? { id: src.needed_by_department_id, name: '' }
        : { name: '' },
      location: src.location_id ? { id: src.location_id, name: '' } : { name: '' },
    };
  }, [cloneSource.data]);

  const handleSubmit = async (values: ItemFormValues) => {
    setServerError(null);
    try {
      const body: CreateItemBody = {
        name: values.name,
        description: values.description,
        item_type: values.item_type,
        size: values.size,
        quantity_owned: values.item_type === 'grouped' ? 0 : Number(values.quantity_owned) || 1,
        unit: values.unit,
        condition: values.condition,
        ...itemFormRefFields(values),
        parent_container_id: values.parent_container_id || undefined,
        serial_number: values.serial_number,
        asset_tag: values.asset_tag,
        qr_code: values.qr_code,
        owner: values.owner,
        purchase_price: values.purchase_price,
        purchase_currency: values.purchase_currency,
        purchase_date: values.purchase_date,
        expiry_date: values.expiry_date,
        supplier: values.supplier,
        purchase_url: values.purchase_url,
        replacement_cost: values.replacement_cost,
        minimum_quantity: Number(values.minimum_quantity) || 0,
        notes: values.notes,
      };

      const created = await createItem(body);

      for (let i = 0; i < pendingPhotos.length; i++) {
        try {
          await uploadFile(pendingPhotos[i], {
            entity_type: 'item',
            entity_id: created.id,
            file_type: 'image',
            set_as_primary: i === 0,
          });
        } catch {
          // Best-effort: remaining uploads still run; user can add missing files on the item page.
        }
      }

      qc.invalidateQueries({ queryKey: ['items'] });
      await qc.invalidateQueries({ queryKey: ['item', created.id] });
      await qc.invalidateQueries({ queryKey: ['item-images', created.id] });
      qc.invalidateQueries({ queryKey: ['categories'] });
      qc.invalidateQueries({ queryKey: ['departments'] });
      qc.invalidateQueries({ queryKey: ['locations'] });
      navigate({ to: '/items/$id', params: { id: created.id } });
    } catch (err) {
      if (err instanceof ApiError) {
        setServerError(err.message);
      } else {
        setServerError('Failed to create item.');
      }
    }
  };

  const defaults: Partial<ItemFormValues> | undefined =
    cloneDefaults ?? (search.asset_tag ? { asset_tag: search.asset_tag } : undefined);

  const isClone = !!search.clone_from;
  const title = isClone ? 'Clone item' : 'New item';
  const description = isClone
    ? `Creating a copy of "${cloneSource.data?.name ?? '…'}". Unique fields (barcode, serial) are cleared.`
    : 'Add a new asset, consumable, container or grouped item to inventory.';

  if (isClone && cloneSource.isLoading) {
    return (
      <PageContainer>
        <PageHeader title={title} description="Loading source item…" />
      </PageContainer>
    );
  }

  return (
    <PageContainer>
      <PageHeader title={title} description={description} />
      {serverError ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 text-destructive text-sm px-4 py-2 mb-4">
          {serverError}
        </div>
      ) : null}

      <Card className="mb-6">
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Photos</CardTitle>
          <CardDescription>
            Attach pictures before you save, or add them later on the item detail page.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <PendingItemPhotos files={pendingPhotos} onChange={setPendingPhotos} />
        </CardContent>
      </Card>

      <ItemForm
        onSubmit={handleSubmit}
        submitLabel={isClone ? 'Create clone' : 'Create item'}
        onCancel={() => navigate({ to: '/items' })}
        defaultValues={defaults}
      />
    </PageContainer>
  );
}
