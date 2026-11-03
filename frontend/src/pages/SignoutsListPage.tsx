import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, useNavigate } from '@tanstack/react-router';
import { Plus, Search } from 'lucide-react';
import { listSignouts } from '@/api/signouts';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Input, Select } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { EmptyRow, ErrorRow, LoadingRow } from '@/components/ui/States';
import { useDepartments } from '@/hooks/useReferenceData';
import { formatDateTime } from '@/lib/utils';
import type { SignoutStatus } from '@/types';

const STATUS_VARIANT: Record<SignoutStatus, 'default' | 'success' | 'warning' | 'outline' | 'destructive' | 'secondary'> = {
  draft: 'outline',
  prepared: 'secondary',
  signed_out: 'success',
  partially_returned: 'warning',
  returned: 'secondary',
  closed: 'outline',
  cancelled: 'destructive',
};

export function SignoutsListPage() {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('');
  const [deptId, setDeptId] = useState('');

  const depts = useDepartments();

  const params = useMemo(
    () => ({
      q: search || undefined,
      status: status || undefined,
      to_department_id: deptId || undefined,
      limit: 100,
    }),
    [search, status, deptId]
  );

  const signouts = useQuery({
    queryKey: ['signouts', params],
    queryFn: () => listSignouts(params),
  });

  const deptName = useMemo(() => {
    const m = new Map<string, string>();
    depts.data?.items.forEach((d) => m.set(d.id, d.name));
    return m;
  }, [depts.data]);

  return (
    <PageContainer>
      <PageHeader
        title="Sign-outs"
        description="Department handovers, active and historical."
        actions={
          <Button onClick={() => navigate({ to: '/signouts/new' })} type="button">
            <Plus className="w-4 h-4" />
            New sign-out
          </Button>
        }
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-4">
        <div className="relative">
          <Search className="w-4 h-4 absolute left-3 top-2.5 text-muted-foreground pointer-events-none" />
          <Input
            placeholder="Search by number, recipient, purpose..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <Select value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">All statuses</option>
          <option value="draft">Draft</option>
          <option value="signed_out">Signed out</option>
          <option value="partially_returned">Partially returned</option>
          <option value="returned">Returned</option>
          <option value="closed">Closed</option>
          <option value="cancelled">Cancelled</option>
        </Select>
        <Select value={deptId} onChange={(e) => setDeptId(e.target.value)}>
          <option value="">All departments</option>
          {depts.data?.items.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name}
            </option>
          ))}
        </Select>
      </div>

      <Table>
        <THead>
          <TR>
            <TH>Number</TH>
            <TH>Status</TH>
            <TH>To department</TH>
            <TH>Received by</TH>
            <TH>Purpose</TH>
            <TH>Signed out at</TH>
          </TR>
        </THead>
        <TBody>
          {signouts.isLoading ? (
            <TR>
              <TD colSpan={6}>
                <LoadingRow />
              </TD>
            </TR>
          ) : signouts.isError ? (
            <TR>
              <TD colSpan={6}>
                <ErrorRow message="Failed to load sign-outs." />
              </TD>
            </TR>
          ) : signouts.data?.items.length === 0 ? (
            <TR>
              <TD colSpan={6}>
                <EmptyRow message="No sign-outs yet." />
              </TD>
            </TR>
          ) : (
            signouts.data?.items.map((s) => (
              <TR key={s.id}>
                <TD>
                  <Link to="/signouts/$id" params={{ id: s.id }} className="font-medium hover:underline">
                    {s.signout_number}
                  </Link>
                </TD>
                <TD>
                  <span className="inline-flex items-center gap-1.5">
                    <Badge variant={STATUS_VARIANT[s.status]}>{s.status}</Badge>
                    {(s.status === 'signed_out' || s.status === 'partially_returned') &&
                    s.expected_return_at &&
                    new Date(s.expected_return_at).getTime() < Date.now() ? (
                      <Badge variant="destructive">overdue</Badge>
                    ) : null}
                  </span>
                </TD>
                <TD>{deptName.get(s.to_department_id) ?? '-'}</TD>
                <TD>{s.received_by_name ?? '-'}</TD>
                <TD className="max-w-xs truncate">{s.purpose || '-'}</TD>
                <TD>{s.signed_out_at ? formatDateTime(s.signed_out_at) : '-'}</TD>
              </TR>
            ))
          )}
        </TBody>
      </Table>
    </PageContainer>
  );
}
