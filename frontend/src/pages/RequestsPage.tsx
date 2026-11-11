import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { ApiError } from '@/api/client';
import { createRequest, decideRequest, listRequests, type RequestInput, type RequestStatus } from '@/api/planning';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Input, Select, Textarea } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { EmptyRow, ErrorRow, LoadingRow } from '@/components/ui/States';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { PERMS, useMe } from '@/hooks/useMe';
import { useDepartments } from '@/hooks/useReferenceData';

const NEXT: Record<RequestStatus, RequestStatus[]> = {
  open: ['approved', 'rejected'],
  approved: ['ordered', 'fulfilled', 'rejected'],
  ordered: ['fulfilled'],
  fulfilled: [],
  rejected: [],
  cancelled: [],
};

/** People ask logistics for gear; logistics approves, orders and fulfils. */
export function RequestsPage() {
  const qc = useQueryClient();
  const { can, me } = useMe();
  const depts = useDepartments();
  const [status, setStatus] = useState('');
  const requests = useQuery({ queryKey: ['requests', status], queryFn: () => listRequests(status) });
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<RequestInput>({ title: '', quantity: 1 });

  const create = useMutation({
    mutationFn: () => createRequest(form),
    onSuccess: () => {
      setOpen(false);
      setForm({ title: '', quantity: 1 });
      qc.invalidateQueries({ queryKey: ['requests'] });
    },
  });
  const decide = useMutation({
    mutationFn: ({ id, to }: { id: string; to: RequestStatus }) => decideRequest(id, to),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['requests'] }),
  });
  const manager = can(PERMS.requestManage);

  return (
    <PageContainer>
      <PageHeader
        title="Requests"
        description="Ask logistics for gear you need, or for something to buy."
        actions={
          can(PERMS.requestCreate) ? (
            <Button onClick={() => setOpen(true)}>
              <Plus className="w-4 h-4" />
              New request
            </Button>
          ) : null
        }
      />
      <div className="mb-4 max-w-xs">
        <Select aria-label="Status" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">All requests</option>
          {(Object.keys(NEXT) as RequestStatus[]).map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </Select>
      </div>
      {decide.isError ? (
        <p role="alert" className="text-sm text-destructive mb-3">
          {decide.error instanceof ApiError ? decide.error.message : 'Update failed.'}
        </p>
      ) : null}
      <Table>
        <THead>
          <TR>
            <TH>What</TH>
            <TH>Qty</TH>
            <TH>For</TH>
            <TH>Needed by</TH>
            <TH>Status</TH>
            <TH />
          </TR>
        </THead>
        <TBody>
          {requests.isLoading ? (
            <TR>
              <TD colSpan={6}>
                <LoadingRow />
              </TD>
            </TR>
          ) : requests.isError ? (
            <TR>
              <TD colSpan={6}>
                <ErrorRow message="Failed to load requests." />
              </TD>
            </TR>
          ) : !requests.data?.items.length ? (
            <TR>
              <TD colSpan={6}>
                <EmptyRow message="No requests." />
              </TD>
            </TR>
          ) : (
            requests.data.items.map((r) => (
              <TR key={r.id}>
                <TD>
                  <span className="font-medium">{r.title}</span>
                  {r.reason ? <span className="block text-xs text-muted-foreground">{r.reason}</span> : null}
                  <span className="block text-xs text-muted-foreground">by {r.requested_by_name ?? '—'}</span>
                </TD>
                <TD className="tabular-nums">{r.quantity}</TD>
                <TD>{r.department_name ?? '—'}</TD>
                <TD className="whitespace-nowrap">{r.needed_by ?? '—'}</TD>
                <TD>
                  <Badge variant={r.status === 'rejected' || r.status === 'cancelled' ? 'outline' : r.status === 'fulfilled' ? 'success' : 'secondary'}>
                    {r.status}
                  </Badge>
                </TD>
                <TD className="text-right whitespace-nowrap space-x-1">
                  {manager
                    ? NEXT[r.status].map((to) => (
                        <Button key={to} size="sm" variant="ghost" onClick={() => decide.mutate({ id: r.id, to })}>
                          {to}
                        </Button>
                      ))
                    : null}
                  {r.status === 'open' && r.requested_by === me?.id ? (
                    <Button size="sm" variant="ghost" onClick={() => decide.mutate({ id: r.id, to: 'cancelled' })}>
                      cancel
                    </Button>
                  ) : null}
                </TD>
              </TR>
            ))
          )}
        </TBody>
      </Table>

      <Dialog open={open} onClose={() => setOpen(false)} title="New request">
        <form
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate();
          }}
        >
          <Field label="What do you need?" htmlFor="r-title">
            <Input id="r-title" value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} required />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="Quantity" htmlFor="r-qty">
              <Input id="r-qty" type="number" min={1} value={form.quantity} onChange={(e) => setForm({ ...form, quantity: Number(e.target.value) })} />
            </Field>
            <Field label="Needed by" htmlFor="r-date">
              <Input id="r-date" type="date" value={form.needed_by ?? ''} onChange={(e) => setForm({ ...form, needed_by: e.target.value || undefined })} />
            </Field>
          </div>
          <Field label="For department" htmlFor="r-dept">
            <Select id="r-dept" value={form.department_id ?? ''} onChange={(e) => setForm({ ...form, department_id: e.target.value || undefined })}>
              <option value="">—</option>
              {depts.data?.items.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Why?" htmlFor="r-reason">
            <Textarea id="r-reason" rows={3} value={form.reason ?? ''} onChange={(e) => setForm({ ...form, reason: e.target.value })} />
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
              Send request
            </Button>
          </div>
        </form>
      </Dialog>
    </PageContainer>
  );
}
