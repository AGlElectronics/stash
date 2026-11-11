import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from '@tanstack/react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { PackageCheck, Plus, Send, Trash2 } from 'lucide-react';
import { ApiError } from '@/api/client';
import { listItems } from '@/api/items';
import {
  createPackingList,
  deletePackingList,
  getPackingList,
  listPackingLists,
  packingListToSignout,
  setPackingItems,
  type PackingLineInput,
} from '@/api/planning';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card, CardContent } from '@/components/ui/Card';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Input, Select, Textarea } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { EmptyRow, ErrorRow, LoadingRow } from '@/components/ui/States';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { PERMS, useMe } from '@/hooks/useMe';
import { useDepartments } from '@/hooks/useReferenceData';

/** Packing lists: what goes to an event or department. */
export function PackingListsPage() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const { can } = useMe();
  const depts = useDepartments();
  const lists = useQuery({ queryKey: ['packing-lists'], queryFn: listPackingLists });
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [deptId, setDeptId] = useState('');
  const [date, setDate] = useState('');
  const [notes, setNotes] = useState('');

  const create = useMutation({
    mutationFn: () =>
      createPackingList({ name, department_id: deptId || undefined, event_date: date || undefined, notes }),
    onSuccess: (pl) => {
      qc.invalidateQueries({ queryKey: ['packing-lists'] });
      void navigate({ to: '/packing-lists/$id', params: { id: pl.id } });
    },
  });

  return (
    <PageContainer>
      <PageHeader
        title="Packing lists"
        description="Plan what goes to an event or department, tick it off while packing, then sign it out in one go."
        actions={
          can(PERMS.signoutCreate) ? (
            <Button onClick={() => setOpen(true)}>
              <Plus className="w-4 h-4" />
              New list
            </Button>
          ) : null
        }
      />
      <Table>
        <THead>
          <TR>
            <TH>Name</TH>
            <TH>Department</TH>
            <TH>Date</TH>
            <TH>Packed</TH>
          </TR>
        </THead>
        <TBody>
          {lists.isLoading ? (
            <TR>
              <TD colSpan={4}>
                <LoadingRow />
              </TD>
            </TR>
          ) : lists.isError ? (
            <TR>
              <TD colSpan={4}>
                <ErrorRow message="Failed to load packing lists." />
              </TD>
            </TR>
          ) : !lists.data?.items.length ? (
            <TR>
              <TD colSpan={4}>
                <EmptyRow message="No packing lists yet." />
              </TD>
            </TR>
          ) : (
            lists.data.items.map((pl) => (
              <TR key={pl.id}>
                <TD>
                  <Link to="/packing-lists/$id" params={{ id: pl.id }} className="font-medium text-primary hover:underline">
                    {pl.name}
                  </Link>
                </TD>
                <TD>{pl.department_name ?? '—'}</TD>
                <TD className="whitespace-nowrap">{pl.event_date ?? '—'}</TD>
                <TD className="tabular-nums">
                  {pl.packed_count}/{pl.item_count}
                  {pl.signout_id ? <Badge variant="success" className="ml-2">signed out</Badge> : null}
                </TD>
              </TR>
            ))
          )}
        </TBody>
      </Table>

      <Dialog open={open} onClose={() => setOpen(false)} title="New packing list">
        <form
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate();
          }}
        >
          <Field label="Name" htmlFor="pl-name">
            <Input id="pl-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Main stage Friday" required />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="Department" htmlFor="pl-dept">
              <Select id="pl-dept" value={deptId} onChange={(e) => setDeptId(e.target.value)}>
                <option value="">—</option>
                {depts.data?.items.map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Event date" htmlFor="pl-date">
              <Input id="pl-date" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
            </Field>
          </div>
          <Field label="Notes" htmlFor="pl-notes">
            <Textarea id="pl-notes" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
          </Field>
          {create.isError ? (
            <p role="alert" className="text-sm text-destructive">
              {create.error instanceof ApiError ? create.error.message : 'Saving failed.'}
            </p>
          ) : null}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={create.isPending}>
              Create
            </Button>
          </div>
        </form>
      </Dialog>
    </PageContainer>
  );
}

/** One packing list: lines with packed counts, add items, create the sign-out. */
export function PackingListDetailPage() {
  const { id } = useParams({ strict: false }) as { id: string };
  const qc = useQueryClient();
  const navigate = useNavigate();
  const { can } = useMe();
  const editable = can(PERMS.signoutCreate);
  const pl = useQuery({ queryKey: ['packing-list', id], queryFn: () => getPackingList(id) });
  const [lines, setLines] = useState<PackingLineInput[]>([]);
  const [search, setSearch] = useState('');
  const found = useQuery({
    queryKey: ['packing-search', search],
    queryFn: () => listItems({ q: search, limit: 8 }),
    enabled: search.trim().length > 1,
  });

  useEffect(() => {
    if (pl.data) {
      setLines((pl.data.items ?? []).map((l) => ({ item_id: l.item_id, quantity: l.quantity, packed: l.packed, note: l.note })));
    }
  }, [pl.data]);

  const save = useMutation({
    mutationFn: (next: PackingLineInput[]) => setPackingItems(id, next),
    onSuccess: (data) => qc.setQueryData(['packing-list', id], data),
  });
  const toSignout = useMutation({
    mutationFn: () => packingListToSignout(id),
    onSuccess: (so) => void navigate({ to: '/signouts/$id', params: { id: so.id } }),
  });
  const remove = useMutation({
    mutationFn: () => deletePackingList(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['packing-lists'] });
      void navigate({ to: '/packing-lists' });
    },
  });

  if (pl.isLoading) return <PageContainer><LoadingRow /></PageContainer>;
  if (pl.isError || !pl.data) return <PageContainer><ErrorRow message="Couldn't load this packing list." /></PageContainer>;

  const names = new Map((pl.data.items ?? []).map((l) => [l.item_id, l]));
  const update = (next: PackingLineInput[]) => {
    setLines(next);
    save.mutate(next);
  };
  const error = save.error ?? toSignout.error ?? remove.error;

  return (
    <PageContainer className="max-w-4xl">
      <PageHeader
        title={pl.data.name}
        description={[pl.data.department_name, pl.data.event_date, pl.data.notes].filter(Boolean).join(' · ')}
        actions={
          editable ? (
            <>
              <Button disabled={!lines.length || toSignout.isPending} onClick={() => toSignout.mutate()}>
                <Send className="w-4 h-4" />
                Create sign-out
              </Button>
              <Button variant="ghost" className="text-destructive" onClick={() => window.confirm('Delete this packing list?') && remove.mutate()}>
                <Trash2 className="w-4 h-4" />
              </Button>
            </>
          ) : null
        }
      />
      {pl.data.signout_id ? (
        <p className="text-sm mb-3">
          Signed out as{' '}
          <Link to="/signouts/$id" params={{ id: pl.data.signout_id }} className="text-primary underline">
            this sign-out
          </Link>
          .
        </p>
      ) : null}
      {error ? (
        <p role="alert" className="text-sm text-destructive mb-3">
          {error instanceof ApiError ? error.message : 'Something went wrong.'}
        </p>
      ) : null}
      <Card>
        <CardContent className="pt-6 space-y-4">
          <Table>
            <THead>
              <TR>
                <TH>Item</TH>
                <TH>Needed</TH>
                <TH>Packed</TH>
                <TH>Available</TH>
                <TH />
              </TR>
            </THead>
            <TBody>
              {lines.length === 0 ? (
                <TR>
                  <TD colSpan={5}>
                    <EmptyRow message="Nothing on this list yet." />
                  </TD>
                </TR>
              ) : (
                lines.map((l, i) => {
                  const info = names.get(l.item_id);
                  const done = l.packed >= l.quantity;
                  return (
                    <TR key={l.item_id}>
                      <TD>
                        <Link to="/items/$id" params={{ id: l.item_id }} className="hover:underline">
                          {info?.item_name ?? 'Item'}
                        </Link>
                        {info?.asset_tag ? <span className="block text-xs font-mono text-muted-foreground">{info.asset_tag}</span> : null}
                      </TD>
                      <TD>
                        <Input
                          type="number"
                          min={1}
                          className="w-20"
                          disabled={!editable}
                          value={l.quantity}
                          onChange={(e) => setLines(lines.map((x, j) => (j === i ? { ...x, quantity: Number(e.target.value) } : x)))}
                          onBlur={() => update(lines)}
                          aria-label="Needed"
                        />
                      </TD>
                      <TD>
                        <div className="flex items-center gap-2">
                          <Input
                            type="number"
                            min={0}
                            max={l.quantity}
                            className="w-20"
                            disabled={!editable}
                            value={l.packed}
                            onChange={(e) => setLines(lines.map((x, j) => (j === i ? { ...x, packed: Number(e.target.value) } : x)))}
                            onBlur={() => update(lines)}
                            aria-label="Packed"
                          />
                          {editable && !done ? (
                            <Button size="sm" variant="ghost" onClick={() => update(lines.map((x, j) => (j === i ? { ...x, packed: x.quantity } : x)))}>
                              <PackageCheck className="w-4 h-4" />
                            </Button>
                          ) : null}
                          {done ? <Badge variant="success">packed</Badge> : null}
                        </div>
                      </TD>
                      <TD className={info && info.available < l.quantity ? 'text-destructive tabular-nums' : 'tabular-nums'}>
                        {info?.available ?? '—'}
                      </TD>
                      <TD className="text-right">
                        {editable ? (
                          <Button size="sm" variant="ghost" aria-label="Remove line" onClick={() => update(lines.filter((_, j) => j !== i))}>
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        ) : null}
                      </TD>
                    </TR>
                  );
                })
              )}
            </TBody>
          </Table>
          {editable ? (
            <div className="space-y-2">
              <Input placeholder="Add an item: type its name…" value={search} onChange={(e) => setSearch(e.target.value)} />
              {found.data?.items.length ? (
                <ul className="rounded-md border border-border divide-y divide-border text-sm">
                  {found.data.items
                    .filter((it) => !lines.some((l) => l.item_id === it.id))
                    .map((it) => (
                      <li key={it.id}>
                        <button
                          type="button"
                          className="w-full text-left px-3 py-2 hover:bg-muted/40"
                          onClick={() => {
                            setSearch('');
                            update([...lines, { item_id: it.id, quantity: 1, packed: 0, note: '' }]);
                          }}
                        >
                          {it.name} {it.asset_tag ? <span className="font-mono text-xs text-muted-foreground">{it.asset_tag}</span> : null}
                        </button>
                      </li>
                    ))}
                </ul>
              ) : null}
            </div>
          ) : null}
        </CardContent>
      </Card>
    </PageContainer>
  );
}
