import { useState } from 'react';
import { useNavigate, useParams } from '@tanstack/react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Ban, CheckCircle, Lock, Undo2 } from 'lucide-react';
import { ApiError } from '@/api/client';
import { listItems } from '@/api/items';
import { cancelSignout, closeSignout, confirmSignout, getSignout, returnSignout, type ReturnLine } from '@/api/signouts';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { Field, Input } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { ErrorRow, LoadingRow } from '@/components/ui/States';
import { useDepartments } from '@/hooks/useReferenceData';
import { formatDateTime } from '@/lib/utils';
import type { SignoutItem, SignoutStatus } from '@/types';

const STATUS_VARIANT: Record<SignoutStatus, 'default' | 'success' | 'warning' | 'outline' | 'destructive' | 'secondary'> = {
  draft: 'outline',
  prepared: 'secondary',
  signed_out: 'success',
  partially_returned: 'warning',
  returned: 'secondary',
  closed: 'outline',
  cancelled: 'destructive',
};

type LineDraft = {
  returned: number;
  consumed: number;
  lost: number;
  damaged: number;
};

function remaining(it: SignoutItem) {
  return Math.max(0, it.quantity_signed_out - it.quantity_returned - it.quantity_consumed - it.quantity_lost - it.quantity_damaged);
}

export function SignoutDetailPage() {
  const { id } = useParams({ strict: false }) as { id: string };
  const navigate = useNavigate();
  const qc = useQueryClient();
  const depts = useDepartments();

  const so = useQuery({ queryKey: ['signout', id], queryFn: () => getSignout(id) });

  const itemIDs = so.data?.items.map((i) => i.item_id) ?? [];
  const itemsQ = useQuery({
    queryKey: ['items-by-id', itemIDs],
    enabled: itemIDs.length > 0,
    queryFn: async () => {
      // pull all items in one call (no /items?ids= endpoint, use search-less list and filter client-side
      // since the dataset stays small for MVP)
      const all = await listItems({ limit: 1000, include_retired: true });
      const byId = new Map(all.items.map((i) => [i.id, i]));
      return itemIDs.map((id) => byId.get(id)!).filter(Boolean);
    },
  });

  const [drafts, setDrafts] = useState<Record<string, LineDraft>>({});
  const [actionError, setActionError] = useState<string | null>(null);

  const setLine = (lineID: string, patch: Partial<LineDraft>) =>
    setDrafts((cur) => ({
      ...cur,
      [lineID]: {
        returned: cur[lineID]?.returned ?? 0,
        consumed: cur[lineID]?.consumed ?? 0,
        lost: cur[lineID]?.lost ?? 0,
        damaged: cur[lineID]?.damaged ?? 0,
        ...patch,
      },
    }));

  const confirmM = useMutation({
    mutationFn: () => confirmSignout(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['signout', id] });
      qc.invalidateQueries({ queryKey: ['signouts'] });
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['item-availability'] });
    },
    onError: (e) => setActionError(e instanceof ApiError ? e.message : 'Confirm failed'),
  });

  const cancelM = useMutation({
    mutationFn: () => cancelSignout(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['signout', id] });
      qc.invalidateQueries({ queryKey: ['signouts'] });
    },
    onError: (e) => setActionError(e instanceof ApiError ? e.message : 'Cancel failed'),
  });

  const closeM = useMutation({
    mutationFn: () => closeSignout(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['signout', id] });
      qc.invalidateQueries({ queryKey: ['signouts'] });
      // Closing archives the signout — there's nothing else actionable on
      // this page, so kick the user back to the overview.
      navigate({ to: '/signouts' });
    },
    onError: (e) => setActionError(e instanceof ApiError ? e.message : 'Close failed'),
  });

  const returnM = useMutation({
    mutationFn: (lines: ReturnLine[]) => returnSignout(id, lines),
    onSuccess: () => {
      setDrafts({});
      qc.invalidateQueries({ queryKey: ['signout', id] });
      qc.invalidateQueries({ queryKey: ['signouts'] });
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['item-availability'] });
    },
    onError: (e) => setActionError(e instanceof ApiError ? e.message : 'Return failed'),
  });

  if (so.isLoading)
    return (
      <PageContainer>
        <LoadingRow />
      </PageContainer>
    );
  if (so.isError || !so.data)
    return (
      <PageContainer>
        <ErrorRow message="Failed to load sign-out." />
      </PageContainer>
    );

  const s = so.data;
  const deptName = depts.data?.items.find((d) => d.id === s.to_department_id)?.name ?? '-';
  const itemsById = new Map((itemsQ.data ?? []).map((i) => [i.id, i]));

  const isOpen = s.status === 'signed_out' || s.status === 'partially_returned';
  const isDraftLike = s.status === 'draft' || s.status === 'prepared';

  const submitReturn = () => {
    setActionError(null);
    const lines: ReturnLine[] = [];
    for (const it of s.items) {
      const d = drafts[it.id];
      if (!d) continue;
      if ((d.returned | 0) === 0 && (d.consumed | 0) === 0 && (d.lost | 0) === 0 && (d.damaged | 0) === 0) continue;
      lines.push({
        signout_item_id: it.id,
        returned: d.returned | 0,
        consumed: d.consumed | 0,
        lost: d.lost | 0,
        damaged: d.damaged | 0,
      });
    }
    if (lines.length === 0) {
      setActionError('Enter quantities to return on at least one line.');
      return;
    }
    returnM.mutate(lines);
  };

  return (
    <PageContainer>
      <PageHeader
        title={s.signout_number}
        description={`To ${deptName} · received by ${s.received_by_name ?? '-'}`}
        actions={
          <>
            <Button variant="ghost" onClick={() => navigate({ to: '/signouts' })}>
              <ArrowLeft className="w-4 h-4" />
              Back
            </Button>
            {isDraftLike ? (
              <Button onClick={() => confirmM.mutate()} disabled={confirmM.isPending}>
                <CheckCircle className="w-4 h-4" />
                Confirm
              </Button>
            ) : null}
            {isDraftLike ? (
              <Button variant="outline" onClick={() => cancelM.mutate()} disabled={cancelM.isPending}>
                <Ban className="w-4 h-4" />
                Cancel
              </Button>
            ) : null}
            {s.status === 'returned' ? (
              <Button variant="outline" onClick={() => closeM.mutate()} disabled={closeM.isPending}>
                <Lock className="w-4 h-4" />
                Close
              </Button>
            ) : null}
          </>
        }
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
        <Card className="md:col-span-2">
          <CardHeader>
            <CardTitle>Details</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-2 gap-y-2 text-sm">
              <dt className="text-muted-foreground">Status</dt>
              <dd>
                <Badge variant={STATUS_VARIANT[s.status]}>{s.status}</Badge>
              </dd>
              <dt className="text-muted-foreground">To department</dt>
              <dd>{deptName}</dd>
              <dt className="text-muted-foreground">Received by</dt>
              <dd>{s.received_by_name ?? '-'}</dd>
              <dt className="text-muted-foreground">Purpose</dt>
              <dd>{s.purpose || '-'}</dd>
              <dt className="text-muted-foreground">Signed out at</dt>
              <dd>{s.signed_out_at ? formatDateTime(s.signed_out_at) : '-'}</dd>
              {s.expected_return_at ? (
                <>
                  <dt className="text-muted-foreground">Expected return</dt>
                  <dd>{formatDateTime(s.expected_return_at)}</dd>
                </>
              ) : null}
              {s.returned_at ? (
                <>
                  <dt className="text-muted-foreground">Returned at</dt>
                  <dd>{formatDateTime(s.returned_at)}</dd>
                </>
              ) : null}
              {s.closed_at ? (
                <>
                  <dt className="text-muted-foreground">Closed at</dt>
                  <dd>{formatDateTime(s.closed_at)}</dd>
                </>
              ) : null}
            </dl>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Items</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <THead>
              <TR>
                <TH>Item</TH>
                <TH className="text-right">Signed out</TH>
                <TH className="text-right">Returned</TH>
                <TH className="text-right">Consumed</TH>
                <TH className="text-right">Lost</TH>
                <TH className="text-right">Damaged</TH>
                <TH className="text-right">Outstanding</TH>
                {isOpen ? <TH colSpan={4}>Process return</TH> : null}
              </TR>
            </THead>
            <TBody>
              {s.items.map((line) => {
                const item = itemsById.get(line.item_id);
                const out = remaining(line);
                const draft = drafts[line.id] ?? { returned: 0, consumed: 0, lost: 0, damaged: 0 };
                return (
                  <TR key={line.id}>
                    <TD className="font-medium">{item?.name ?? line.item_id.slice(0, 8)}</TD>
                    <TD className="text-right tabular-nums">{line.quantity_signed_out}</TD>
                    <TD className="text-right tabular-nums">{line.quantity_returned}</TD>
                    <TD className="text-right tabular-nums">{line.quantity_consumed}</TD>
                    <TD className="text-right tabular-nums">{line.quantity_lost}</TD>
                    <TD className="text-right tabular-nums">{line.quantity_damaged}</TD>
                    <TD className="text-right tabular-nums font-semibold">{out}</TD>
                    {isOpen ? (
                      <>
                        <TD>
                          <Field label="Ret">
                            <Input
                              type="number"
                              min={0}
                              max={out}
                              value={draft.returned}
                              onChange={(e) => setLine(line.id, { returned: Number(e.target.value) })}
                            />
                          </Field>
                        </TD>
                        <TD>
                          <Field label="Cons">
                            <Input
                              type="number"
                              min={0}
                              max={out}
                              value={draft.consumed}
                              onChange={(e) => setLine(line.id, { consumed: Number(e.target.value) })}
                            />
                          </Field>
                        </TD>
                        <TD>
                          <Field label="Lost">
                            <Input
                              type="number"
                              min={0}
                              max={out}
                              value={draft.lost}
                              onChange={(e) => setLine(line.id, { lost: Number(e.target.value) })}
                            />
                          </Field>
                        </TD>
                        <TD>
                          <Field label="Dmg">
                            <Input
                              type="number"
                              min={0}
                              max={out}
                              value={draft.damaged}
                              onChange={(e) => setLine(line.id, { damaged: Number(e.target.value) })}
                            />
                          </Field>
                        </TD>
                      </>
                    ) : null}
                  </TR>
                );
              })}
            </TBody>
          </Table>

          {actionError ? <p className="text-sm text-destructive mt-3">{actionError}</p> : null}

          {isOpen ? (
            <div className="flex justify-end mt-4">
              <Button onClick={submitReturn} disabled={returnM.isPending}>
                <Undo2 className="w-4 h-4" />
                {returnM.isPending ? 'Saving...' : 'Submit return'}
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </PageContainer>
  );
}
