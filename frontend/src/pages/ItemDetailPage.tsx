import { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate, useParams } from '@tanstack/react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Box,
  Copy,
  Hammer,
  Move,
  Pencil,
  Printer,
  RotateCcw,
  ShieldX,
  Sliders,
  PackageX,
  Plus,
  Search,
  Tag,
  Trash2,
} from 'lucide-react';
import { ApiError } from '@/api/client';
import { assignBarcode, createItem, getItem, getItemAvailability, listItems, moveItem, updateItem } from '@/api/items';
import { Badge } from '@/components/ui/Badge';
import { Barcode } from '@/components/Barcode';
import { Button } from '@/components/ui/Button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Input, Select } from '@/components/ui/Input';
import { ErrorRow, LoadingRow } from '@/components/ui/States';
import { Tabs } from '@/components/ui/Tabs';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { ItemHistoryTimeline } from '@/components/ItemHistoryTimeline';
import { ItemActionDialog } from '@/components/ItemActionDialogs';
import { ItemImagesPanel } from '@/components/ItemImagesPanel';
import { ItemAttachmentsPanel } from '@/components/ItemAttachmentsPanel';
import { ItemPurchasesPanel } from '@/components/ItemPurchasesPanel';
import { useCategories, useDepartments, useLocations } from '@/hooks/useReferenceData';
import { useFileBlobUrl } from '@/hooks/useFileBlobUrl';
import { buildLocationPathMap } from '@/lib/locationPath';
import { downloadCsvFile, rowsToCsv } from '@/lib/csvExport';
import type { Item } from '@/types';

type Action =
  | 'move'
  | 'adjust'
  | 'damage'
  | 'repair'
  | 'lost'
  | 'found'
  | 'retire'
  | 'restore';

export function ItemDetailPage() {
  const { id } = useParams({ strict: false }) as { id: string };
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [tab, setTab] = useState<'overview' | 'history'>('overview');
  const [dialog, setDialog] = useState<Action | null>(null);
  const [assignError, setAssignError] = useState<string | null>(null);

  const item = useQuery({ queryKey: ['item', id], queryFn: () => getItem(id) });
  const avail = useQuery({
    queryKey: ['item-availability', id],
    queryFn: () => getItemAvailability(id),
  });

  const assign = useMutation({
    mutationFn: () => assignBarcode(id),
    onSuccess: () => {
      setAssignError(null);
      qc.invalidateQueries({ queryKey: ['item', id] });
      qc.invalidateQueries({ queryKey: ['items'] });
    },
    onError: (err) => {
      setAssignError(err instanceof ApiError ? err.message : 'Failed to assign barcode.');
    },
  });

  const cats = useCategories();
  const depts = useDepartments();
  const locs = useLocations();
  const locPathById = useMemo(
    () => buildLocationPathMap(locs.data?.items ?? []),
    [locs.data?.items]
  );

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

  const it = item.data;
  const catName = cats.data?.items.find((c) => c.id === it.category_id)?.name;
  const deptName = depts.data?.items.find((d) => d.id === it.managed_by_department_id)?.name;
  const neededByName = depts.data?.items.find((d) => d.id === it.needed_by_department_id)?.name;
  const locLabel = it.location_id ? locPathById.get(it.location_id) ?? '—' : '—';

  const isRetired = it.status === 'retired';

  return (
    <PageContainer>
      <PageHeader
        title={it.name}
        description={it.description || undefined}
        actions={
          <>
            <Button variant="outline" onClick={() => navigate({ to: '/items/$id/edit', params: { id } })}>
              <Pencil className="w-4 h-4" />
              Edit
            </Button>
            <Button variant="outline" onClick={() => navigate({ to: '/items/new', search: { clone_from: id } })}>
              <Copy className="w-4 h-4" />
              Clone
            </Button>
            {it.asset_tag || it.qr_code || it.serial_number ? (
              <Button
                variant="outline"
                onClick={() => window.open(`/items/${id}/label`, '_blank', 'noopener')}
              >
                <Printer className="w-4 h-4" />
                Print label
              </Button>
            ) : null}
            {!isRetired && (
              <Button variant="ghost" onClick={() => setDialog('retire')}>
                <Box className="w-4 h-4" />
                Retire
              </Button>
            )}
            {isRetired && (
              <Button variant="ghost" onClick={() => setDialog('restore')}>
                <RotateCcw className="w-4 h-4" />
                Restore
              </Button>
            )}
          </>
        }
      />

      <Tabs
        className="mb-6"
        tabs={[
          { id: 'overview', label: 'Overview' },
          { id: 'history', label: 'History' },
        ]}
        value={tab}
        onChange={setTab}
      />

      {tab === 'overview' ? (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Card className="md:col-span-2">
            <CardHeader>
              <CardTitle>Details</CardTitle>
            </CardHeader>
            <CardContent>
              {it.primary_image_file_id ? (
                <PrimaryImage fileId={it.primary_image_file_id} alt={it.name} />
              ) : null}
              <dl className="grid grid-cols-2 gap-y-2 text-sm">
                <dt className="text-muted-foreground">Type</dt>
                <dd>
                  <Badge variant="secondary">{it.item_type}</Badge>
                </dd>
                <dt className="text-muted-foreground">Status</dt>
                <dd>
                  <Badge variant={isRetired ? 'outline' : 'success'}>{it.status}</Badge>
                </dd>
                <dt className="text-muted-foreground">Condition</dt>
                <dd>{it.condition}</dd>
                <dt className="text-muted-foreground">Category</dt>
                <dd>
                  {catName ? (
                    <Link to="/categories" className="hover:underline">
                      {catName}
                    </Link>
                  ) : (
                    '-'
                  )}
                </dd>
                <dt className="text-muted-foreground">Managed by</dt>
                <dd>{deptName ?? '-'}</dd>
                <dt className="text-muted-foreground">Needed by</dt>
                <dd>{neededByName ?? '-'}</dd>
                <dt className="text-muted-foreground">Location</dt>
                <dd>{locLabel}</dd>
                {it.parent_group_id ? (
                  <>
                    <dt className="text-muted-foreground">Part of group</dt>
                    <dd>
                      <Link
                        to="/items/$id"
                        params={{ id: it.parent_group_id }}
                        className="hover:underline"
                      >
                        View group
                      </Link>
                    </dd>
                  </>
                ) : null}
                {it.parent_container_id ? (
                  <>
                    <dt className="text-muted-foreground">Inside container</dt>
                    <dd>
                      <Link
                        to="/items/$id"
                        params={{ id: it.parent_container_id }}
                        className="hover:underline font-mono text-xs"
                      >
                        {it.parent_container_id.slice(0, 8)}…
                      </Link>
                    </dd>
                  </>
                ) : null}
                {it.size ? (
                  <>
                    <dt className="text-muted-foreground">Size</dt>
                    <dd>{it.size}</dd>
                  </>
                ) : null}
                {it.serial_number ? (
                  <>
                    <dt className="text-muted-foreground">Serial number</dt>
                    <dd>{it.serial_number}</dd>
                  </>
                ) : null}
                {it.asset_tag ? (
                  <>
                    <dt className="text-muted-foreground">Barcode / asset tag</dt>
                    <dd className="font-mono">{it.asset_tag}</dd>
                  </>
                ) : null}
                {it.purchase_price ? (
                  <>
                    <dt className="text-muted-foreground">Purchase price</dt>
                    <dd>
                      {it.purchase_price} {it.purchase_currency}
                    </dd>
                  </>
                ) : null}
                {it.purchase_date ? (
                  <>
                    <dt className="text-muted-foreground">Purchase date</dt>
                    <dd>{it.purchase_date}</dd>
                  </>
                ) : null}
                {it.expiry_date ? (
                  <>
                    <dt className="text-muted-foreground">Expiry date</dt>
                    <dd
                      className={
                        new Date(it.expiry_date + 'T00:00:00').getTime() < Date.now()
                          ? 'text-destructive font-medium'
                          : undefined
                      }
                    >
                      {it.expiry_date}
                    </dd>
                  </>
                ) : null}
                {it.supplier ? (
                  <>
                    <dt className="text-muted-foreground">Supplier</dt>
                    <dd>{it.supplier}</dd>
                  </>
                ) : null}
              </dl>
              {it.notes ? (
                <div className="mt-4 pt-4 border-t border-border">
                  <p className="text-xs uppercase text-muted-foreground tracking-wide mb-1">Notes</p>
                  <p className="text-sm whitespace-pre-line">{it.notes}</p>
                </div>
              ) : null}
            </CardContent>
          </Card>

          <div className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Barcode</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col items-center gap-2">
                {it.asset_tag || it.qr_code || it.serial_number ? (
                  <>
                    <Barcode value={(it.asset_tag || it.qr_code || it.serial_number) as string} />
                    <div className="flex flex-wrap gap-2 justify-center">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => window.open(`/items/${id}/label`, '_blank', 'noopener')}
                      >
                        <Printer className="w-4 h-4" />
                        Print label
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          if (
                            window.confirm(
                              `Replace ${it.asset_tag ?? 'current barcode'} with a newly generated code?`
                            )
                          ) {
                            assign.mutate();
                          }
                        }}
                        disabled={assign.isPending}
                      >
                        <Tag className="w-4 h-4" />
                        New code
                      </Button>
                    </div>
                  </>
                ) : (
                  <>
                    <p className="text-sm text-muted-foreground text-center">
                      No barcode yet. Generate a code, or scan a pre-printed sticker with the
                      Scan button in the sidebar.
                    </p>
                    <Button onClick={() => assign.mutate()} disabled={assign.isPending} size="sm">
                      <Tag className="w-4 h-4" />
                      {assign.isPending ? 'Generating…' : 'Generate code'}
                    </Button>
                  </>
                )}
                {assignError ? (
                  <p className="text-xs text-destructive text-center">{assignError}</p>
                ) : null}
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>Availability</CardTitle>
              </CardHeader>
              <CardContent>
                {avail.isLoading ? (
                  <LoadingRow />
                ) : avail.data ? (
                  <dl className="grid grid-cols-2 gap-y-1 text-sm">
                    <dt className="text-muted-foreground">Owned</dt>
                    <dd className="text-right tabular-nums font-medium">{avail.data.quantity_owned}</dd>
                    <dt className="text-muted-foreground">Damaged</dt>
                    <dd className="text-right tabular-nums">{avail.data.quantity_damaged}</dd>
                    <dt className="text-muted-foreground">Lost</dt>
                    <dd className="text-right tabular-nums">{avail.data.quantity_lost}</dd>
                    <dt className="text-muted-foreground">Signed out</dt>
                    <dd className="text-right tabular-nums">{avail.data.quantity_signed_out}</dd>
                    <dt className="text-muted-foreground border-t border-border pt-1">Available</dt>
                    <dd className="text-right tabular-nums font-semibold border-t border-border pt-1">
                      {avail.data.quantity_available}
                    </dd>
                  </dl>
                ) : null}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Actions</CardTitle>
              </CardHeader>
              <CardContent className="grid grid-cols-2 gap-2">
                <Button variant="outline" size="sm" onClick={() => setDialog('move')} disabled={isRetired}>
                  <Move className="w-4 h-4" />
                  Move
                </Button>
                <Button variant="outline" size="sm" onClick={() => setDialog('adjust')} disabled={isRetired}>
                  <Sliders className="w-4 h-4" />
                  Adjust qty
                </Button>
                <Button variant="outline" size="sm" onClick={() => setDialog('damage')} disabled={isRetired}>
                  <ShieldX className="w-4 h-4" />
                  Damage
                </Button>
                <Button variant="outline" size="sm" onClick={() => setDialog('repair')} disabled={isRetired}>
                  <Hammer className="w-4 h-4" />
                  Repair
                </Button>
                <Button variant="outline" size="sm" onClick={() => setDialog('lost')} disabled={isRetired}>
                  <PackageX className="w-4 h-4" />
                  Lost
                </Button>
                <Button variant="outline" size="sm" onClick={() => setDialog('found')} disabled={isRetired}>
                  <Search className="w-4 h-4" />
                  Found
                </Button>
              </CardContent>
            </Card>
          </div>

          {it.item_type === 'container' || it.item_type === 'grouped' ? (
            <ContainerContentsPanel
              containerId={id}
              containerName={it.name}
              mode={it.item_type === 'grouped' ? 'grouped' : 'container'}
            />
          ) : null}

          <Card className="md:col-span-3">
            <CardHeader>
              <CardTitle>Images</CardTitle>
            </CardHeader>
            <CardContent>
              <ItemImagesPanel itemId={id} primaryImageFileID={it.primary_image_file_id} />
            </CardContent>
          </Card>

          <Card className="md:col-span-3">
            <CardHeader>
              <CardTitle>Documents</CardTitle>
            </CardHeader>
            <CardContent>
              <ItemAttachmentsPanel itemId={id} />
            </CardContent>
          </Card>

          <Card className="md:col-span-3">
            <CardHeader>
              <CardTitle>Purchases</CardTitle>
            </CardHeader>
            <CardContent>
              <ItemPurchasesPanel itemId={id} />
            </CardContent>
          </Card>
        </div>
      ) : (
        <Card>
          <CardContent className="pt-6">
            <ItemHistoryTimeline itemId={id} />
          </CardContent>
        </Card>
      )}

      <ItemActionDialog
        open={dialog !== null}
        action={dialog}
        itemId={id}
        onClose={() => setDialog(null)}
      />
    </PageContainer>
  );
}

function ContainerContentsPanel({
  containerId,
  containerName,
  mode,
}: {
  containerId: string;
  containerName: string;
  mode: 'container' | 'grouped';
}) {
  const qc = useQueryClient();
  const [addOpen, setAddOpen] = useState(false);
  const [editRow, setEditRow] = useState<Item | null>(null);
  const [removePendingId, setRemovePendingId] = useState<string | null>(null);
  const isGrouped = mode === 'grouped';

  const contents = useQuery({
    queryKey: [isGrouped ? 'group-contents' : 'container-contents', containerId],
    queryFn: async () => {
      const out: Item[] = [];
      let offset = 0;
      const limit = 500;
      let total = Infinity;
      while (out.length < total && out.length < 10_000) {
        const page = await listItems(
          isGrouped
            ? { parent_group_id: containerId, limit, offset }
            : { parent_container_id: containerId, limit, offset }
        );
        out.push(...page.items);
        total = page.total;
        if (page.items.length < limit) break;
        offset += limit;
      }
      return out;
    },
  });

  const containers = useQuery({
    queryKey: ['containers-lookup'],
    queryFn: () => listItems({ item_type: 'container', limit: 500 }),
  });

  const containerNameById = useMemo(() => {
    const map = new Map<string, string>();
    for (const c of containers.data?.items ?? []) {
      map.set(c.id, c.name);
    }
    return map;
  }, [containers.data?.items]);

  const totalOwned = useMemo(
    () => (contents.data ?? []).reduce((sum, row) => sum + row.quantity_owned, 0),
    [contents.data]
  );

  const remove = useMutation({
    mutationFn: (itemId: string) =>
      isGrouped
        ? updateItem(itemId, { clear_group: true })
        : moveItem(itemId, { clear_container: true, note: `removed from ${containerName}` }),
    onMutate: (itemId: string) => {
      setRemovePendingId(itemId);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [isGrouped ? 'group-contents' : 'container-contents', containerId] });
      qc.invalidateQueries({ queryKey: ['items'] });
    },
    onSettled: () => {
      setRemovePendingId(null);
    },
  });

  const handleRemove = (row: Item) => {
    const target = isGrouped ? 'this group' : containerName;
    const detail = isGrouped
      ? 'The size variant will be removed from the group entirely.'
      : 'It will stay in inventory but no longer be stored in this container.';
    if (!window.confirm(`Remove "${row.name}" from ${target}? ${detail}`)) {
      return;
    }
    remove.mutate(row.id);
  };

  const exportCsv = () => {
    const rows = contents.data;
    if (!rows?.length) return;
    const head = [
      'id',
      'name',
      ...(isGrouped ? ['size', 'container_id'] : []),
      'item_type',
      'status',
      'quantity_owned',
      'quantity_damaged',
      'quantity_lost',
      'asset_tag',
      'serial_number',
    ];
    const body = rows.map((i) => [
      i.id,
      i.name,
      ...(isGrouped ? [i.size ?? '', i.parent_container_id ?? ''] : []),
      i.item_type,
      i.status,
      String(i.quantity_owned),
      String(i.quantity_damaged),
      String(i.quantity_lost),
      i.asset_tag ?? '',
      i.serial_number ?? '',
    ]);
    const safe = containerName.replace(/[^\w.-]+/g, '_').slice(0, 48) || 'container';
    downloadCsvFile(`${safe}-${isGrouped ? 'sizes' : 'contents'}.csv`, rowsToCsv([head, ...body]));
  };

  return (
    <Card className="md:col-span-3">
      <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
        <CardTitle>{isGrouped ? 'Sizes' : 'Contents'}</CardTitle>
        <div className="flex items-center gap-2">
          <Button type="button" size="sm" onClick={() => setAddOpen(true)}>
            <Plus className="w-4 h-4" />
            {isGrouped ? 'Add size' : 'Add items'}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={exportCsv}
            disabled={!contents.data?.length}
          >
            Export CSV
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {isGrouped ? (
          <p className="text-sm text-muted-foreground mb-3">
            Total across all sizes and containers:{' '}
            <span className="font-semibold tabular-nums text-foreground">{totalOwned}</span>
            <span className="block text-xs mt-1">
              Add the same size again with a different container to split stock (e.g. 5000 XL in bulk storage vs 50 at the venue).
            </span>
          </p>
        ) : null}
        {contents.isLoading ? (
          <LoadingRow />
        ) : contents.isError ? (
          <ErrorRow message={isGrouped ? 'Failed to load sizes.' : 'Failed to load contents.'} />
        ) : !contents.data?.length ? (
          <p className="text-sm text-muted-foreground">
            {isGrouped
              ? 'No size variants yet. Add sizes to track stock per size.'
              : 'No items are stored in this container yet.'}
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left">
                  {isGrouped ? <th className="py-2 pr-2 font-medium">Size</th> : null}
                  <th className="py-2 pr-2 font-medium">Name</th>
                  {isGrouped ? <th className="py-2 pr-2 font-medium">Container</th> : null}
                  {!isGrouped ? <th className="py-2 pr-2 font-medium">Type</th> : null}
                  <th className="py-2 pr-2 font-medium text-right">Owned</th>
                  <th className="py-2 pr-2 font-medium">Status</th>
                  <th className="py-2 pr-2 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {contents.data.map((row) => (
                  <tr key={row.id} className="border-b border-border/60">
                    {isGrouped ? (
                      <td className="py-2 pr-2 font-medium">{row.size || '—'}</td>
                    ) : null}
                    <td className="py-2 pr-2">
                      <Link to="/items/$id" params={{ id: row.id }} className="font-medium hover:underline">
                        {row.name}
                      </Link>
                    </td>
                    {isGrouped ? (
                      <td className="py-2 pr-2">
                        {row.parent_container_id ? (
                          <Link
                            to="/items/$id"
                            params={{ id: row.parent_container_id }}
                            className="hover:underline"
                          >
                            {containerNameById.get(row.parent_container_id) ?? 'Container'}
                          </Link>
                        ) : (
                          <span className="text-muted-foreground">Unassigned</span>
                        )}
                      </td>
                    ) : null}
                    {!isGrouped ? (
                      <td className="py-2 pr-2">
                        <Badge variant="secondary">{row.item_type}</Badge>
                      </td>
                    ) : null}
                    <td className="py-2 pr-2 text-right tabular-nums">{row.quantity_owned}</td>
                    <td className="py-2 pr-2">
                      <Badge variant={row.status === 'retired' ? 'outline' : 'success'}>{row.status}</Badge>
                    </td>
                    <td className="py-2 pr-2 text-right">
                      <div className="flex justify-end gap-1">
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          onClick={() => setEditRow(row)}
                          title={isGrouped ? 'Change container' : 'Move to another container'}
                        >
                          <Pencil className="w-4 h-4" />
                          Move
                        </Button>
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          disabled={remove.isPending && removePendingId === row.id}
                          onClick={() => handleRemove(row)}
                        >
                          <Trash2 className="w-4 h-4" />
                          {remove.isPending && removePendingId === row.id ? 'Removing…' : 'Remove'}
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <p className="text-xs text-muted-foreground mt-2">
              {contents.data.length} {isGrouped ? 'size(s)' : 'item(s)'}
              {isGrouped ? ` · ${totalOwned} total owned` : ''}
            </p>
          </div>
        )}
      </CardContent>
      {isGrouped ? (
        <AddSizeToGroupDialog
          open={addOpen}
          onClose={() => setAddOpen(false)}
          groupId={containerId}
          groupName={containerName}
        />
      ) : (
        <AddItemsToContainerDialog
          open={addOpen}
          onClose={() => setAddOpen(false)}
          containerId={containerId}
          containerName={containerName}
        />
      )}
      <EditContainerDialog
        open={editRow !== null}
        item={editRow}
        onClose={() => setEditRow(null)}
        onSuccess={() => {
          qc.invalidateQueries({ queryKey: [isGrouped ? 'group-contents' : 'container-contents', containerId] });
          qc.invalidateQueries({ queryKey: ['items'] });
        }}
      />
    </Card>
  );
}

function EditContainerDialog({
  open,
  item,
  onClose,
  onSuccess,
}: {
  open: boolean;
  item: Item | null;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [containerId, setContainerId] = useState('');
  const [error, setError] = useState<string | null>(null);

  const containers = useQuery({
    queryKey: ['containers-lookup'],
    queryFn: () => listItems({ item_type: 'container', limit: 500 }),
    enabled: open,
  });

  useEffect(() => {
    if (open && item) {
      setContainerId(item.parent_container_id ?? '');
      setError(null);
    }
  }, [open, item]);

  const save = useMutation({
    mutationFn: async () => {
      if (!item) return;
      const previous = item.parent_container_id ?? '';
      if (containerId === previous) return;
      if (!containerId) {
        return moveItem(item.id, { clear_container: true, note: 'cleared container assignment' });
      }
      const dest = containers.data?.items.find((c) => c.id === containerId);
      return moveItem(item.id, {
        to_container_id: containerId,
        note: dest ? `moved to ${dest.name}` : 'moved to container',
      });
    },
    onSuccess: () => {
      onSuccess();
      onClose();
    },
    onError: (e: unknown) => {
      setError(e instanceof ApiError ? e.message : 'Failed to update container.');
    },
  });

  if (!item) return null;

  const label = item.size ? `${item.size}${item.name ? ` · ${item.name}` : ''}` : item.name;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Move to container"
      description={`Choose where ${label} is stored.`}
    >
      <div className="space-y-3">
        <Field label="Container" htmlFor="edit-container">
          <Select
            id="edit-container"
            value={containerId}
            onChange={(e) => setContainerId(e.target.value)}
          >
            <option value="">No container (unassigned)</option>
            {(containers.data?.items ?? []).map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        </Field>
        {error ? <ErrorRow message={error} /> : null}
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button type="button" onClick={() => save.mutate()} disabled={save.isPending}>
            {save.isPending ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

function AddSizeToGroupDialog({
  open,
  onClose,
  groupId,
  groupName,
}: {
  open: boolean;
  onClose: () => void;
  groupId: string;
  groupName: string;
}) {
  const qc = useQueryClient();
  const [size, setSize] = useState('');
  const [quantity, setQuantity] = useState('0');
  const [containerId, setContainerId] = useState('');
  const [error, setError] = useState<string | null>(null);

  const containers = useQuery({
    queryKey: ['containers-lookup'],
    queryFn: () => listItems({ item_type: 'container', limit: 500 }),
    enabled: open,
  });

  const add = useMutation({
    mutationFn: async () => {
      const trimmedSize = size.trim();
      if (!trimmedSize) {
        throw new Error('Size is required.');
      }
      const qty = Number(quantity);
      if (!Number.isFinite(qty) || qty < 0) {
        throw new Error('Quantity must be zero or greater.');
      }
      const container = containers.data?.items.find((c) => c.id === containerId);
      const suffix = container ? ` (${container.name})` : '';
      return createItem({
        name: `${groupName} — ${trimmedSize}${suffix}`,
        item_type: 'consumable',
        size: trimmedSize,
        quantity_owned: qty,
        parent_group_id: groupId,
        parent_container_id: containerId || undefined,
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['group-contents', groupId] });
      qc.invalidateQueries({ queryKey: ['items'] });
      setSize('');
      setQuantity('0');
      setContainerId('');
      setError(null);
      onClose();
    },
    onError: (e: unknown) => {
      setError(e instanceof ApiError ? e.message : e instanceof Error ? e.message : 'Failed to add size.');
    },
  });

  const handleClose = () => {
    setSize('');
    setQuantity('0');
    setContainerId('');
    setError(null);
    onClose();
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Add size variant"
      description={`Add a size and quantity to ${groupName}. Pick a container to store this batch separately.`}
    >
      <div className="space-y-3">
        <Field label="Size" htmlFor="group-size">
          <Input
            id="group-size"
            autoFocus
            placeholder="S, M, L, XL…"
            value={size}
            onChange={(e) => setSize(e.target.value)}
          />
        </Field>
        <Field label="Quantity owned" htmlFor="group-qty">
          <Input
            id="group-qty"
            type="number"
            min={0}
            step={1}
            value={quantity}
            onChange={(e) => setQuantity(e.target.value)}
          />
        </Field>
        <Field label="Container" htmlFor="group-container" hint="Optional — use different containers for the same size">
          <Select
            id="group-container"
            value={containerId}
            onChange={(e) => setContainerId(e.target.value)}
          >
            <option value="">No container (unassigned)</option>
            {(containers.data?.items ?? []).map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        </Field>
        {error ? <ErrorRow message={error} /> : null}
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={handleClose}>
            Cancel
          </Button>
          <Button type="button" onClick={() => add.mutate()} disabled={add.isPending}>
            {add.isPending ? 'Adding…' : 'Add size'}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

function AddItemsToContainerDialog({
  open,
  onClose,
  containerId,
  containerName,
}: {
  open: boolean;
  onClose: () => void;
  containerId: string;
  containerName: string;
}) {
  const qc = useQueryClient();
  const [query, setQuery] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [pendingId, setPendingId] = useState<string | null>(null);

  const search = useQuery({
    queryKey: ['container-add-search', containerId, query],
    queryFn: () => listItems({ q: query.trim() || undefined, limit: 25 }),
    enabled: open,
  });

  const candidates = useMemo(
    () =>
      (search.data?.items ?? []).filter(
        (i) => i.id !== containerId && i.parent_container_id !== containerId
      ),
    [search.data?.items, containerId]
  );

  const add = useMutation({
    mutationFn: (itemId: string) =>
      moveItem(itemId, { to_container_id: containerId, note: `added to ${containerName}` }),
    onMutate: (itemId: string) => {
      setPendingId(itemId);
      setError(null);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['container-contents', containerId] });
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['container-add-search', containerId] });
    },
    onError: (e: unknown) => {
      setError(e instanceof ApiError ? e.message : 'Failed to add item to container.');
    },
    onSettled: () => {
      setPendingId(null);
    },
  });

  const handleClose = () => {
    setQuery('');
    setError(null);
    onClose();
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Add items to container"
      description={`Search for items to move into ${containerName}.`}
    >
      <div className="space-y-3">
        <Input
          autoFocus
          placeholder="Search by name, serial or barcode…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        {error ? <ErrorRow message={error} /> : null}
        <div className="max-h-72 overflow-auto rounded-md border border-border">
          {search.isLoading ? (
            <LoadingRow />
          ) : search.isError ? (
            <ErrorRow message="Search failed." />
          ) : candidates.length === 0 ? (
            <p className="p-3 text-sm text-muted-foreground">No matching items.</p>
          ) : (
            <ul className="divide-y divide-border/60">
              {candidates.map((row) => (
                <li key={row.id} className="flex items-center justify-between gap-2 p-2">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">{row.name}</p>
                    <p className="truncate text-xs text-muted-foreground">
                      <Badge variant="secondary">{row.item_type}</Badge>
                      {row.asset_tag ? <span className="ml-2 font-mono">{row.asset_tag}</span> : null}
                    </p>
                  </div>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={add.isPending && pendingId === row.id}
                    onClick={() => add.mutate(row.id)}
                  >
                    {add.isPending && pendingId === row.id ? 'Adding…' : 'Add'}
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
        <div className="flex justify-end">
          <Button type="button" variant="outline" onClick={handleClose}>
            Done
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

function PrimaryImage({ fileId, alt }: { fileId: string; alt: string }) {
  const preview = useFileBlobUrl(fileId);
  if (preview.isLoading) {
    return (
      <div className="mb-4 h-48 w-full rounded-md border border-border bg-muted/30 animate-pulse" />
    );
  }
  if (!preview.data) return null;
  return (
    <div className="mb-4 overflow-hidden rounded-md border border-border bg-muted/30">
      <img
        src={preview.data}
        alt={alt}
        className="block w-full max-h-72 object-contain mx-auto"
        loading="lazy"
      />
    </div>
  );
}
