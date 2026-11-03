import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, Navigate, useNavigate } from '@tanstack/react-router';
import {
  AlarmClock,
  Boxes,
  CalendarClock,
  PackageMinus,
  PackageX,
  Plus,
  Search,
  Send,
  Undo2,
  Wrench,
} from 'lucide-react';
import { listItems } from '@/api/items';
import { listSignouts } from '@/api/signouts';
import { listLostFound } from '@/api/lostfound';
import { Badge } from '@/components/ui/Badge';
import { LinkButton } from '@/components/ui/Button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { PageContainer } from '@/components/ui/PageHeader';
import { StatCard } from '@/components/ui/StatCard';
import { useDepartments } from '@/hooks/useReferenceData';
import { useMe, PERMS } from '@/hooks/useMe';
import { formatDateTime, cn } from '@/lib/utils';
import type { Signout } from '@/types';

function greeting(): string {
  const h = new Date().getHours();
  if (h < 5) return 'Up late';
  if (h < 12) return 'Good morning';
  if (h < 18) return 'Good afternoon';
  return 'Good evening';
}

function isOverdue(s: Signout): boolean {
  return !!s.expected_return_at && new Date(s.expected_return_at).getTime() < Date.now();
}

function isExpiringSoon(expiryDate: string | undefined): boolean {
  if (!expiryDate) return false;
  const exp = new Date(expiryDate + 'T00:00:00');
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const horizon = new Date(today);
  horizon.setDate(horizon.getDate() + 30);
  return exp <= horizon;
}

export function DashboardPage() {
  const { me, can, isSuccess } = useMe();

  const showSignouts = can(PERMS.signoutView);
  const showInventory = can(PERMS.itemView);
  const showLostFound = can(PERMS.lostfoundView);

  // Users whose role only grants Lost & Found with no other dashboard
  // sections (basic frontdesk volunteers) land straight in their section.
  if (isSuccess && !showSignouts && !showInventory && showLostFound) {
    return <Navigate to="/lost-and-found" />;
  }

  const firstName = me?.display_name?.split(' ')[0];

  return (
    <PageContainer>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between mb-6">
        <div>
          <h1 className="font-heading text-2xl font-extrabold leading-tight tracking-tight">
            {greeting()}
            {firstName ? `, ${firstName}` : ''}!
          </h1>
          <p className="text-sm text-muted-foreground mt-1">Here's what's happening in Stash.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {can(PERMS.itemCreate) ? (
            <LinkButton to="/items/new" variant="secondary">
              <Plus className="w-4 h-4" />
              New item
            </LinkButton>
          ) : null}
          {can(PERMS.lostfoundCreate) ? (
            <LinkButton
              to="/lost-and-found/new"
              variant={can(PERMS.signoutCreate) ? 'secondary' : 'primary'}
            >
              <Search className="w-4 h-4" />
              Report lost / found
            </LinkButton>
          ) : null}
          {can(PERMS.signoutCreate) ? (
            <LinkButton to="/signouts/new">
              <Send className="w-4 h-4" />
              New sign-out
            </LinkButton>
          ) : null}
        </div>
      </div>

      {showInventory ? <QuickSearch /> : null}

      <div className="space-y-6">
        {showSignouts ? <SignoutOpsSection /> : null}
        {showInventory ? <InventoryHealthSection /> : null}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {showLostFound ? <LostFoundBlock /> : null}
        </div>
      </div>
    </PageContainer>
  );
}

/** Big search input that jumps straight into the items list. */
function QuickSearch() {
  const navigate = useNavigate();
  const [q, setQ] = useState('');
  return (
    <form
      className="relative mb-6"
      onSubmit={(e) => {
        e.preventDefault();
        navigate({ to: '/items', search: q.trim() ? { q: q.trim() } : {} }).catch(() => undefined);
      }}
    >
      <Search className="w-4 h-4 absolute left-4 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
      <Input
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder="Quick search the inventory — name, serial, barcode… (Enter)"
        className="h-11 pl-10 rounded-pill text-base shadow-card"
      />
    </form>
  );
}

function SignoutOpsSection() {
  const active = useQuery({
    queryKey: ['dashboard-signouts-active'],
    queryFn: () => listSignouts({ status: 'signed_out', limit: 200 }),
  });
  const partial = useQuery({
    queryKey: ['dashboard-signouts-partial'],
    queryFn: () => listSignouts({ status: 'partially_returned', limit: 200 }),
  });

  const all = useMemo(() => {
    const rows = [...(active.data?.items ?? []), ...(partial.data?.items ?? [])];
    rows.sort((a, b) => (b.signed_out_at ?? b.created_at).localeCompare(a.signed_out_at ?? a.created_at));
    return rows;
  }, [active.data, partial.data]);

  const overdue = all.filter(isOverdue);

  return (
    <section>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4 mb-4">
        <StatCard
          label="Signed out"
          value={active.data ? active.data.total : '…'}
          icon={Send}
          to="/signouts"
        />
        <StatCard
          label="Overdue"
          value={active.data || partial.data ? overdue.length : '…'}
          icon={AlarmClock}
          tone={overdue.length > 0 ? 'bad' : 'good'}
          to="/signouts"
          hint={overdue.length > 0 ? 'past expected return' : 'nothing overdue'}
        />
        <StatCard
          label="Partial returns"
          value={partial.data ? partial.data.total : '…'}
          icon={Undo2}
          tone={(partial.data?.total ?? 0) > 0 ? 'warn' : 'default'}
          to="/signouts"
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Currently signed out</CardTitle>
        </CardHeader>
        <CardContent>
          {active.isLoading || partial.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading…</p>
          ) : all.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Nothing is checked out right now. Enjoy the calm!
            </p>
          ) : (
            <ul className="divide-y divide-border">
              {all.slice(0, 6).map((s) => {
                const late = isOverdue(s);
                return (
                  <li key={s.id} className="py-2.5 flex flex-wrap items-center gap-x-3 gap-y-1">
                    <Link
                      to="/signouts/$id"
                      params={{ id: s.id }}
                      className="font-semibold text-sm hover:underline"
                    >
                      {s.signout_number}
                    </Link>
                    {s.status === 'partially_returned' ? (
                      <Badge variant="warning">partial</Badge>
                    ) : null}
                    {late ? <Badge variant="destructive">overdue</Badge> : null}
                    <span className={cn('text-xs ml-auto', late ? 'text-destructive' : 'text-muted-foreground')}>
                      {s.received_by_name}
                      {s.signed_out_at ? ` · ${formatDateTime(s.signed_out_at)}` : ''}
                    </span>
                  </li>
                );
              })}
            </ul>
          )}
          {all.length > 6 ? (
            <Link to="/signouts" className="inline-block text-sm text-primary hover:underline mt-3">
              View all {all.length} active sign-outs →
            </Link>
          ) : null}
        </CardContent>
      </Card>
    </section>
  );
}

function InventoryHealthSection() {
  const { me } = useMe();
  const depts = useDepartments();
  const myDeptId = me?.primary_department_id || '';
  const [scope, setScope] = useState<'mine' | 'all'>(myDeptId ? 'mine' : 'all');

  const items = useQuery({
    queryKey: ['dashboard-items'],
    queryFn: () => listItems({ limit: 1000 }),
  });

  const scoped = useMemo(() => {
    const rows = items.data?.items ?? [];
    if (scope === 'mine' && myDeptId) {
      return rows.filter((i) => i.managed_by_department_id === myDeptId);
    }
    return rows;
  }, [items.data, scope, myDeptId]);

  const lowStock = scoped.filter(
    (i) =>
      i.status !== 'retired' &&
      i.minimum_quantity > 0 &&
      i.quantity_owned - i.quantity_damaged - i.quantity_lost < i.minimum_quantity
  );
  const damaged = scoped.filter((i) => i.quantity_damaged > 0);
  const lost = scoped.reduce((n, i) => n + i.quantity_lost, 0);
  const expiringSoon = scoped.filter(
    (i) => i.status !== 'retired' && isExpiringSoon(i.expiry_date)
  );

  const myDeptName = myDeptId
    ? depts.data?.items.find((d) => d.id === myDeptId)?.name ?? 'My department'
    : null;

  return (
    <section>
      <div className="flex items-center justify-between gap-2 mb-3">
        <h2 className="font-heading font-bold text-sm uppercase tracking-wide text-muted-foreground">
          Inventory health
        </h2>
        {myDeptId ? (
          <div className="inline-flex items-center gap-1 rounded-pill bg-muted p-1">
            {(['mine', 'all'] as const).map((v) => (
              <button
                key={v}
                type="button"
                onClick={() => setScope(v)}
                className={cn(
                  'rounded-pill px-3 py-1 text-xs font-semibold transition-colors',
                  scope === v ? 'bg-card shadow-card' : 'text-muted-foreground hover:text-foreground'
                )}
              >
                {v === 'mine' ? myDeptName : 'All departments'}
              </button>
            ))}
          </div>
        ) : null}
      </div>
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
        <StatCard
          label="Items"
          value={items.data ? scoped.length : '…'}
          icon={Boxes}
          to="/items"
        />
        <StatCard
          label="Low stock"
          value={items.data ? lowStock.length : '…'}
          icon={PackageMinus}
          tone={lowStock.length > 0 ? 'warn' : 'good'}
          to="/items"
          hint={lowStock.length > 0 ? 'below minimum quantity' : 'all stocked up'}
        />
        <StatCard
          label="Expiring soon"
          value={items.data ? expiringSoon.length : '…'}
          icon={CalendarClock}
          tone={expiringSoon.length > 0 ? 'warn' : 'default'}
          to="/items"
          hint={expiringSoon.length > 0 ? 'within 30 days (or past due)' : undefined}
        />
        <StatCard
          label="Damaged"
          value={items.data ? damaged.length : '…'}
          icon={Wrench}
          tone={damaged.length > 0 ? 'warn' : 'default'}
          to="/items"
          hint={damaged.length > 0 ? 'items in the repair queue' : undefined}
        />
        <StatCard
          label="Lost"
          value={items.data ? lost : '…'}
          icon={PackageX}
          tone={lost > 0 ? 'bad' : 'default'}
        />
      </div>
    </section>
  );
}

function LostFoundBlock() {
  const open = useQuery({
    queryKey: ['dashboard-lostfound'],
    queryFn: () => listLostFound({ status: 'open', limit: 5 }),
    retry: false,
  });

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between">
        <CardTitle>Lost &amp; Found</CardTitle>
        <Link to="/lost-and-found" className="text-sm text-primary hover:underline">
          Open section →
        </Link>
      </CardHeader>
      <CardContent>
        {open.isLoading ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : open.isError || !open.data ? (
          <p className="text-sm text-muted-foreground">Lost &amp; Found isn't available yet.</p>
        ) : open.data.items.length === 0 ? (
          <p className="text-sm text-muted-foreground">No open reports. Everything found its way home.</p>
        ) : (
          <>
            <p className="text-sm mb-2">
              <span className="font-heading font-extrabold text-2xl tabular-nums">{open.data.total}</span>{' '}
              <span className="text-muted-foreground">open report{open.data.total === 1 ? '' : 's'}</span>
            </p>
            <ul className="divide-y divide-border">
              {open.data.items.map((r) => (
                <li key={r.id} className="py-2 flex items-center gap-2">
                  <Badge variant={r.type === 'lost' ? 'destructive' : 'success'}>{r.type}</Badge>
                  <Link
                    to="/lost-and-found/$id"
                    params={{ id: r.id }}
                    className="text-sm font-medium hover:underline truncate"
                  >
                    {r.item_name}
                  </Link>
                  <span className="text-xs text-muted-foreground ml-auto shrink-0">{r.report_number}</span>
                </li>
              ))}
            </ul>
          </>
        )}
      </CardContent>
    </Card>
  );
}
