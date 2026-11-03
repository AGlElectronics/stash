import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { Download, Search } from 'lucide-react';
import { listItems } from '@/api/items';
import { getMe } from '@/api/me';
import { fetchAuthed } from '@/api/client';
import { Button } from '@/components/ui/Button';
import { Input, Select } from '@/components/ui/Input';
import { Tabs } from '@/components/ui/Tabs';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { Badge } from '@/components/ui/Badge';
import { EmptyRow, ErrorRow, LoadingRow } from '@/components/ui/States';
import { useCategories, useDepartments, useLocations } from '@/hooks/useReferenceData';
import { useItemsAvailability } from '@/hooks/useItemsAvailability';
import { itemAvailable } from '@/lib/itemAvailability';
import { buildLocationPathMap } from '@/lib/locationPath';
import { downloadCsvFile, rowsToCsv } from '@/lib/csvExport';
import type { Item } from '@/types';

const MAX_ROWS = 10_000;
const PAGE_SIZE = 500;

async function fetchItemsAllMatching(params: {
  q?: string;
  category_id?: string;
  item_type?: string;
  managed_dept_id?: string;
  needed_dept_id?: string;
}): Promise<{ items: Item[]; truncated: boolean; total: number }> {
  const base = { ...params, limit: PAGE_SIZE };
  const first = await listItems({ ...base, offset: 0 });
  const out: Item[] = [...first.items];
  let offset = first.items.length;
  const total = first.total;
  while (out.length < Math.min(total, MAX_ROWS) && offset < total) {
    const page = await listItems({ ...base, offset });
    if (page.items.length === 0) break;
    out.push(...page.items);
    offset += page.items.length;
  }
  const truncated = out.length < total;
  return { items: out, truncated, total };
}

type GroupMode = 'managed' | 'needed';

export function ItemsByDepartmentPage() {
  const [mode, setMode] = useState<GroupMode>('managed');
  const [search, setSearch] = useState('');
  const [categoryId, setCategoryId] = useState('');
  const [itemType, setItemType] = useState('');
  const [managedDeptFilter, setManagedDeptFilter] = useState('');
  const [neededDeptFilter, setNeededDeptFilter] = useState('');

  const cats = useCategories();
  const depts = useDepartments();
  const locs = useLocations();
  const me = useQuery({ queryKey: ['me'], queryFn: getMe });

  const listParams = useMemo(
    () => ({
      q: search || undefined,
      category_id: categoryId || undefined,
      item_type: itemType || undefined,
      managed_dept_id: managedDeptFilter || undefined,
      needed_dept_id: neededDeptFilter || undefined,
    }),
    [search, categoryId, itemType, managedDeptFilter, neededDeptFilter]
  );

  const itemsQ = useQuery({
    queryKey: ['items-by-dept', listParams],
    queryFn: () => fetchItemsAllMatching(listParams),
  });

  const itemIds = useMemo(() => itemsQ.data?.items.map((it) => it.id) ?? [], [itemsQ.data?.items]);
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

  const locPath = useMemo(() => buildLocationPathMap(locs.data?.items ?? []), [locs.data?.items]);

  const groups = useMemo(() => {
    const items = itemsQ.data?.items ?? [];
    const keyFn = (it: Item) =>
      mode === 'managed' ? it.managed_by_department_id ?? '' : it.needed_by_department_id ?? '';
    const map = new Map<string, Item[]>();
    for (const it of items) {
      const k = keyFn(it);
      if (!map.has(k)) map.set(k, []);
      map.get(k)!.push(it);
    }
    const keys = [...map.keys()].sort((a, b) => {
      const na = a ? deptName.get(a) ?? a : '';
      const nb = b ? deptName.get(b) ?? b : '';
      if (!a && b) return -1;
      if (a && !b) return 1;
      return na.localeCompare(nb, undefined, { sensitivity: 'base' });
    });
    return keys.map((id) => ({
      deptId: id,
      title: id ? deptName.get(id) ?? id : 'Unassigned',
      items: map.get(id) ?? [],
    }));
  }, [itemsQ.data?.items, mode, deptName]);

  const exportClientCsv = () => {
    const rows = itemsQ.data?.items;
    if (!rows?.length) return;
    const head = [
      'name',
      'item_type',
      'status',
      'quantity_owned',
      'available_approx',
      'category',
      'managed_by',
      'needed_by',
      'location_path',
      'asset_tag',
      'id',
    ];
    const body = rows.map((it) => {
      const free = itemAvailable(it, liveAvailable.get(it.id));
      return [
        it.name,
        it.item_type,
        it.status,
        String(it.quantity_owned),
        String(free),
        it.category_id ? catName.get(it.category_id) ?? '' : '',
        it.managed_by_department_id ? deptName.get(it.managed_by_department_id) ?? '' : '',
        it.needed_by_department_id ? deptName.get(it.needed_by_department_id) ?? '' : '',
        it.location_id ? locPath.get(it.location_id) ?? '' : '',
        it.asset_tag ?? '',
        it.id,
      ];
    });
    downloadCsvFile(`items-by-department-${mode}.csv`, rowsToCsv([head, ...body]));
  };

  const canServerExport = me.data?.permissions?.includes('export.create');

  const exportServerCsv = async () => {
    const res = await fetchAuthed('/api/export/items.csv');
    if (!res.ok) {
      window.alert('Export failed. You may need export permission.');
      return;
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `items-export-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <PageContainer>
      <PageHeader
        title="Items by department"
        description="Overview of inventory grouped by responsible department. Export for Excel or audit."
        actions={
          <div className="flex flex-wrap gap-2 justify-end">
            {canServerExport ? (
              <Button type="button" variant="outline" onClick={() => exportServerCsv().catch(() => undefined)}>
                <Download className="w-4 h-4" />
                Server CSV
              </Button>
            ) : null}
            <Button
              type="button"
              variant="outline"
              onClick={exportClientCsv}
              disabled={!itemsQ.data?.items.length}
            >
              <Download className="w-4 h-4" />
              Export view (CSV)
            </Button>
          </div>
        }
      />

      <Tabs
        className="mb-4"
        tabs={[
          { id: 'managed', label: 'Managed by department' },
          { id: 'needed', label: 'Needed by department' },
        ]}
        value={mode}
        onChange={setMode}
      />

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3 mb-6">
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
        <Select value={itemType} onChange={(e) => setItemType(e.target.value)}>
          <option value="">All types</option>
          <option value="asset">Assets</option>
          <option value="consumable">Consumables</option>
          <option value="container">Containers</option>
          <option value="grouped">Grouped</option>
        </Select>
        <Select value={managedDeptFilter} onChange={(e) => setManagedDeptFilter(e.target.value)}>
          <option value="">Any managed-by dept</option>
          {depts.data?.items.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name}
            </option>
          ))}
        </Select>
        <Select value={neededDeptFilter} onChange={(e) => setNeededDeptFilter(e.target.value)}>
          <option value="">Any needed-by dept</option>
          {depts.data?.items.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name}
            </option>
          ))}
        </Select>
      </div>

      {itemsQ.isLoading ? (
        <LoadingRow />
      ) : itemsQ.isError ? (
        <ErrorRow message="Failed to load items." />
      ) : (
        <>
          {itemsQ.data?.truncated ? (
            <p className="text-sm text-warning mb-4">
              Loaded first {MAX_ROWS} rows only. Narrow filters or use server export if you have access.
            </p>
          ) : null}
          {!groups.length || groups.every((g) => g.items.length === 0) ? (
            <EmptyRow message="No items match your filters." />
          ) : (
            <div className="space-y-10">
              {groups.map((g) =>
                g.items.length === 0 ? null : (
                  <section key={g.deptId || '__none__'}>
                    <h2 className="text-lg font-semibold mb-2">
                      {g.title}{' '}
                      <span className="text-muted-foreground font-normal text-sm">({g.items.length})</span>
                    </h2>
                    <Table>
                      <THead>
                        <TR>
                          <TH>Name</TH>
                          <TH>Type</TH>
                          <TH className="text-right">Available</TH>
                          <TH className="text-right">Owned</TH>
                          <TH>Category</TH>
                          {mode === 'managed' ? <TH>Needed by</TH> : <TH>Managed by</TH>}
                          <TH>Location</TH>
                          <TH>Status</TH>
                        </TR>
                      </THead>
                      <TBody>
                        {g.items.map((it) => {
                          const free = itemAvailable(it, liveAvailable.get(it.id));
                          const isRetired = it.status === 'retired';
                          const crossDept =
                            mode === 'managed'
                              ? it.needed_by_department_id
                                ? deptName.get(it.needed_by_department_id) ?? '—'
                                : '—'
                              : it.managed_by_department_id
                                ? deptName.get(it.managed_by_department_id) ?? '—'
                                : '—';
                          return (
                            <TR key={it.id}>
                              <TD>
                                <Link
                                  to="/items/$id"
                                  params={{ id: it.id }}
                                  className="font-medium hover:underline"
                                >
                                  {it.name}
                                </Link>
                              </TD>
                              <TD>
                                <Badge variant="secondary">{it.item_type}</Badge>
                              </TD>
                              <TD className="text-right tabular-nums font-semibold">{free}</TD>
                              <TD className="text-right tabular-nums text-muted-foreground">
                                {it.quantity_owned}
                              </TD>
                              <TD>{it.category_id ? catName.get(it.category_id) ?? '—' : '—'}</TD>
                              <TD>{crossDept}</TD>
                              <TD className="max-w-[14rem] truncate text-sm text-muted-foreground">
                                {it.location_id ? locPath.get(it.location_id) ?? '—' : '—'}
                              </TD>
                              <TD>
                                <Badge variant={isRetired ? 'outline' : 'success'}>{it.status}</Badge>
                              </TD>
                            </TR>
                          );
                        })}
                      </TBody>
                    </Table>
                  </section>
                )
              )}
            </div>
          )}
          {itemsQ.data ? (
            <p className="text-xs text-muted-foreground mt-6">
              Loaded {itemsQ.data.items.length} of {itemsQ.data.total} matching items
            </p>
          ) : null}
        </>
      )}
    </PageContainer>
  );
}
