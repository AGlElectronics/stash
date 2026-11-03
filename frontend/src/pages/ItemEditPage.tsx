import { useState } from 'react';
import { useNavigate, useParams } from '@tanstack/react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@/api/client';
import { adjustItem, getItem, updateItem } from '@/api/items';
import { ItemForm, type ItemFormValues } from '@/forms/ItemForm';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { LoadingRow, ErrorRow } from '@/components/ui/States';
import { itemFormRefFields } from '@/lib/resolveCreatableRef';

export function ItemEditPage() {
  const { id } = useParams({ strict: false }) as { id: string };
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [serverError, setServerError] = useState<string | null>(null);

  const item = useQuery({
    queryKey: ['item', id],
    queryFn: () => getItem(id),
  });

  const handleSubmit = async (values: ItemFormValues) => {
    const it = item.data;
    if (!it) return;
    setServerError(null);
    try {
      const refs = itemFormRefFields(values);

      await updateItem(id, {
        name: values.name,
        description: values.description,
        ...refs,
        clear_category: !refs.category_id && !refs.category_name && Boolean(it.category_id),
        item_type: values.item_type,
        size: values.size,
        unit: values.unit,
        condition: values.condition,
        clear_location: !refs.location_id && !refs.location_name && Boolean(it.location_id),
        parent_container_id: values.parent_container_id || undefined,
        clear_container:
          !values.parent_container_id && Boolean(it.parent_container_id),
        clear_managed_by:
          !refs.managed_by_department_id &&
          !refs.managed_by_department_name &&
          Boolean(it.managed_by_department_id),
        clear_needed_by:
          !refs.needed_by_department_id &&
          !refs.needed_by_department_name &&
          Boolean(it.needed_by_department_id),
        serial_number: values.serial_number,
        asset_tag: values.asset_tag,
        qr_code: values.qr_code,
        owner: values.owner,
        purchase_price: values.purchase_price,
        purchase_currency: values.purchase_currency,
        purchase_date: values.purchase_date,
        clear_purchase_date: !values.purchase_date && Boolean(it.purchase_date),
        expiry_date: values.expiry_date,
        clear_expiry_date: !values.expiry_date && Boolean(it.expiry_date),
        supplier: values.supplier,
        purchase_url: values.purchase_url,
        replacement_cost: values.replacement_cost,
        minimum_quantity: values.minimum_quantity,
        notes: values.notes,
      });

      const qtyDelta = values.quantity_owned - it.quantity_owned;
      if (qtyDelta !== 0) {
        await adjustItem(id, { delta: qtyDelta, note: 'quantity updated on item edit' });
      }

      qc.invalidateQueries({ queryKey: ['items'] });
      await qc.invalidateQueries({ queryKey: ['item', id] });
      await qc.invalidateQueries({ queryKey: ['item-images', id] });
      await qc.invalidateQueries({ queryKey: ['item-availability', id] });
      if (it.primary_image_file_id) {
        qc.removeQueries({ queryKey: ['file-blob', it.primary_image_file_id] });
      }
      qc.invalidateQueries({ queryKey: ['categories'] });
      qc.invalidateQueries({ queryKey: ['departments'] });
      qc.invalidateQueries({ queryKey: ['locations'] });
      navigate({ to: '/items/$id', params: { id } });
    } catch (err) {
      if (err instanceof ApiError) {
        setServerError(err.message);
      } else {
        setServerError('Failed to update item.');
      }
    }
  };

  if (item.isLoading)
    return (
      <PageContainer>
        <LoadingRow />
      </PageContainer>
    );
  if (item.isError || !item.data)
    return (
      <PageContainer>
        <ErrorRow message="Failed to load item." />
      </PageContainer>
    );

  return (
    <PageContainer>
      <PageHeader title={`Edit: ${item.data.name}`} />
      {serverError ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 text-destructive text-sm px-4 py-2 mb-4">
          {serverError}
        </div>
      ) : null}
      <ItemForm
        defaultValues={{
          name: item.data.name,
          description: item.data.description,
          category: item.data.category_id
            ? { id: item.data.category_id, name: '' }
            : { name: '' },
          item_type: item.data.item_type,
          size: item.data.size ?? '',
          quantity_owned: item.data.quantity_owned,
          unit: item.data.unit,
          condition: item.data.condition,
          location: item.data.location_id ? { id: item.data.location_id, name: '' } : { name: '' },
          parent_container_id: item.data.parent_container_id ?? '',
          managed_by_department: item.data.managed_by_department_id
            ? { id: item.data.managed_by_department_id, name: '' }
            : { name: '' },
          needed_by_department: item.data.needed_by_department_id
            ? { id: item.data.needed_by_department_id, name: '' }
            : { name: '' },
          serial_number: item.data.serial_number ?? '',
          asset_tag: item.data.asset_tag ?? '',
          qr_code: item.data.qr_code ?? '',
          owner: item.data.owner,
          purchase_price: item.data.purchase_price ?? '',
          purchase_currency: item.data.purchase_currency,
          purchase_date: item.data.purchase_date ?? '',
          expiry_date: item.data.expiry_date ?? '',
          supplier: item.data.supplier ?? '',
          purchase_url: item.data.purchase_url ?? '',
          replacement_cost: item.data.replacement_cost ?? '',
          minimum_quantity: item.data.minimum_quantity,
          notes: item.data.notes,
        }}
        submitLabel="Save changes"
        onSubmit={handleSubmit}
        onCancel={() => navigate({ to: '/items/$id', params: { id } })}
      />
    </PageContainer>
  );
}
