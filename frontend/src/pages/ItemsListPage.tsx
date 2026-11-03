import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearch } from '@tanstack/react-router';
import { Plus, Search, Send, ChevronLeft, ChevronRight } from 'lucide-react';
import { listItems } from '@/api/items';
import { Button, LinkButton } from '@/components/ui/Button';
import { Link } from '@tanstack/react-router';
import { Input, Select } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { Badge } from '@/components/ui/Badge';
import { EmptyRow, ErrorRow, LoadingRow } from '@/components/ui/States';
import { FastSignoutDialog } from '@/components/FastSignoutDialog';
import { useCategories, useDepartments, useLocations } from '@/hooks/useReferenceData';
import { useItemsAvailability } from '@/hooks/useItemsAvailability';
import { itemAvailable } from '@/lib/itemAvailability';
import { buildLocationPathMap } from '@/lib/locationPath';
import type { Item } from '@/types';

export function ItemsListPage() {
  // Seeded by the dashboard quick-search deep link (/items?q=...).
  const { q: initialQ } = useSearch({ strict: false }) as { q?: string };
  const [search, setSearch] = useState(initialQ ?? '');
  const [categoryId, setCategoryId] = useState('');
  const [managedDeptId, setManagedDeptId] = useState('');
  const [neededDeptId, setNeededDeptId] = useState('');
  const [locationId, setLocationId] = useState('');
  const [includeSublocations, setIncludeSublocations] = useState(false);
  const [itemType, setItemType] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(100);

  const [signoutItem, setSignoutItem] = useState<Item | null>(null);

  const cats = useCategories();
  const depts = useDepartments();
  const locs = useLocations();

  const locPath = useMemo(
    () => buildLocationPathMap(locs.data?.items ?? []),
    [locs.data?.items]
  );

  const params = useMemo(
    () => ({
      q: search || undefined,
      category_id: categoryId || undefined,
      managed_dept_id: managedDeptId || undefined,
      needed_dept_id: neededDeptId || undefined,
      location_id: locationId || undefined,
      include_sublocations: locationId ? includeSublocations : undefined,
      item_type: itemType || undefined,
      limit: pageSize,
      offset: (page - 1) * pageSize,
    }),
    [search, categoryId, managedDeptId, neededDeptId, locationId, includeSublocations, itemType, page, pageSize]
  );

  useEffect(() => {
    setPage(1);
  }, [search, categoryId, managedDeptId, neededDeptId, locationId, includeSublocations, itemType, pageSize]);

  const items = useQuery({
    queryKey: ['items', params],
    queryFn: () => listItems(params),
  });

  const itemIds = useMemo(() => items.data?.items.map((it) => it.id) ?? [], [items.data?.items]);
  const liveAvailable = useItemsAvailability(itemIds);

  const catName = useMemo(() => {
    const map = new Map<string, string>();
    cats.data?.items.forEach((c) => map.set(c.id, c.name));
    return map;
  }, [cats.data]);

  const deptName = useMemo(() => {
    const map = new Map<string, string>();
    depts.data?.items.forEach((d) => map.set(d.id, d.name));
    return map;
  }, [depts.data]);

  return (
    <PageContainer>
      <PageHeader
        title="Items"
        description="Browse, search and sign out inventory."
        actions={
          <LinkButton to="/items/new">
            <Plus className="w-4 h-4" />
            New item
          </LinkButton>
        }
      />

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3 mb-4">
        <div className="relative lg:col-span-2">
          <Search className="w-4 h-4 absolute left-3 top-2.5 text-muted-foreground pointer-events-none" />
          <Input
            placeholder="Search by name, serial, barcode..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <Select value={categoryId} onChange={(e) => setCategoryId(e.target.value)}>
          <option value="">All categories</option>
          {cats.data?.items.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </Select>
        <Select value={managedDeptId} onChange={(e) => setManagedDeptId(e.target.value)}>
          <option value="">Managed by (any)</option>
          {depts.data?.items.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name}
            </option>
          ))}
        </Select>
        <Select value={neededDeptId} onChange={(e) => setNeededDeptId(e.target.value)}>
          <option value="">Needed by (any)</option>
          {depts.data?.items.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name}
            </option>
          ))}
        </Select>
        <Select value={locationId} onChange={(e) => setLocationId(e.target.value)}>
          <option value="">Location (any)</option>
          {(locs.data?.items ?? []).map((l) => (
            <option key={l.id} value={l.id}>
              {locPath.get(l.id) ?? l.name}
            </option>
          ))}
        </Select>
        <div className="flex items-center gap-2 lg:col-span-2">
          <Select value={itemType} onChange={(e) => setItemType(e.target.value)} className="flex-1">
            <option value="">All types</option>
            <option value="asset">Assets</option>
            <option value="consumable">Consumables</option>
            <option value="container">Containers</option>
            <option value="grouped">Grouped</option>
          </Select>
          <label className="flex items-center gap-2 text-sm text-muted-foreground whitespace-nowrap shrink-0">
            <input
              type="checkbox"
              checked={includeSublocations}
              disabled={!locationId}
              onChange={(e) => setIncludeSublocations(e.target.checked)}
            />
            Include sub-locations
          </label>
        </div>
      </div>

      <Table>
        <THead>
          <TR>
            <TH>Name</TH>
            <TH>Type</TH>
            <TH className="text-right">Available</TH>
            <TH className="text-right">Owned</TH>
            <TH className="text-right">Damaged</TH>
            <TH>Category</TH>
            <TH>Managed by</TH>
            <TH>Needed by</TH>
            <TH>Location</TH>
            <TH>Status</TH>
            <TH className="w-32" />
          </TR>
        </THead>
        <TBody>
          {items.isLoading ? (
            <TR>
              <TD colSpan={11}>
                <LoadingRow />
              </TD>
            </TR>
          ) : items.isError ? (
            <TR>
              <TD colSpan={11}>
                <ErrorRow message="Failed to load items." />
              </TD>
            </TR>
          ) : items.data?.items.length === 0 ? (
            <TR>
              <TD colSpan={11}>
                <EmptyRow message="No items match your filters." />
              </TD>
            </TR>
          ) : (
            items.data?.items.map((it) => {
              const free = itemAvailable(it, liveAvailable.get(it.id));
              const isRetired = it.status === 'retired';
              return (
                <TR key={it.id}>
                  <TD>
                    <Link to="/items/$id" params={{ id: it.id }} className="font-medium hover:underline">
                      {it.name}
                    </Link>
                    {it.serial_number ? (
                      <div className="text-xs text-muted-foreground">SN {it.serial_number}</div>
                    ) : null}
                  </TD>
                  <TD>
                    <Badge variant="secondary">{it.item_type}</Badge>
                  </TD>
                  <TD className="text-right tabular-nums font-semibold">{free}</TD>
                  <TD className="text-right tabular-nums text-muted-foreground">{it.quantity_owned}</TD>
                  <TD className="text-right tabular-nums">
                    {it.quantity_damaged > 0 ? (
                      <Badge variant="warning">{it.quantity_damaged}</Badge>
                    ) : (
                      <span className="text-muted-foreground">0</span>
                    )}
                  </TD>
                  <TD>{it.category_id ? catName.get(it.category_id) ?? '-' : '-'}</TD>
                  <TD>
                    {it.managed_by_department_id
                      ? deptName.get(it.managed_by_department_id) ?? '-'
                      : '-'}
                  </TD>
                  <TD>
                    {it.needed_by_department_id
                      ? deptName.get(it.needed_by_department_id) ?? '-'
                      : '-'}
                  </TD>
                  <TD className="max-w-[12rem] truncate text-sm text-muted-foreground">
                    {it.location_id ? locPath.get(it.location_id) ?? '-' : '-'}
                  </TD>
                  <TD>
                    <Badge variant={isRetired ? 'outline' : 'success'}>{it.status}</Badge>
                  </TD>
                  <TD className="text-right">
                    <Button
                      size="sm"
                      onClick={() => setSignoutItem(it)}
                      disabled={isRetired || free <= 0}
                      title={free <= 0 ? 'Nothing available to sign out' : 'Fast sign-out'}
                    >
                      <Send className="w-4 h-4" />
                      Sign out
                    </Button>
                  </TD>
                </TR>
              );
            })
          )}
        </TBody>
      </Table>

      {signoutItem ? (
        <FastSignoutDialog
          open
          onClose={() => setSignoutItem(null)}
          item={signoutItem}
        />
      ) : null}

      {items.data ? (
        <div className="flex flex-wrap items-center justify-between gap-3 mt-2">
          <p className="text-xs text-muted-foreground">
            {items.data.total === 0
              ? 'No items'
              : `Showing ${(page - 1) * pageSize + 1}–${Math.min(page * pageSize, items.data.total)} of ${items.data.total}`}
          </p>
          <div className="flex items-center gap-2">
            <Select
              value={String(pageSize)}
              onChange={(e) => setPageSize(Number(e.target.value))}
              className="w-auto text-xs"
              aria-label="Page size"
            >
              <option value="50">50 / page</option>
              <option value="100">100 / page</option>
              <option value="200">200 / page</option>
            </Select>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              <ChevronLeft className="w-4 h-4" />
              Prev
            </Button>
            <span className="text-xs text-muted-foreground tabular-nums">
              Page {page} of {Math.max(1, Math.ceil(items.data.total / pageSize))}
            </span>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={page * pageSize >= items.data.total}
              onClick={() => setPage((p) => p + 1)}
            >
              Next
              <ChevronRight className="w-4 h-4" />
            </Button>
          </div>
        </div>
      ) : null}
    </PageContainer>
  );
}
