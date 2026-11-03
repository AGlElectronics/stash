import { useEffect, useState } from 'react';
import { useParams, useSearch, useNavigate } from '@tanstack/react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  ArrowLeft,
  CheckCircle2,
  HandHeart,
  MapPin,
  Pencil,
  Printer,
  RotateCcw,
  Trash2,
  XCircle,
} from 'lucide-react';
import { ApiError } from '@/api/client';
import {
  getLostFoundReport,
  updateLostFoundReport,
  deleteLostFoundReport,
  type LostFoundReport,
} from '@/api/lostfound';
import { fetchFileBlobURL, listFiles } from '@/api/files';
import { Badge } from '@/components/ui/Badge';
import { Button, LinkButton } from '@/components/ui/Button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Input, Textarea } from '@/components/ui/Input';
import { PageContainer } from '@/components/ui/PageHeader';
import { ErrorRow, LoadingRow } from '@/components/ui/States';
import { useMe, PERMS } from '@/hooks/useMe';
import { useSettings } from '@/hooks/useSettings';
import { isSacredLostFoundReport } from '@/lib/lostfoundSacred';
import { formatDateTime, cn } from '@/lib/utils';

export function LostFoundDetailPage() {
  const { id } = useParams({ strict: false }) as { id: string };
  const { fresh } = useSearch({ strict: false }) as { fresh?: boolean };
  const navigate = useNavigate();
  const qc = useQueryClient();
  const { can } = useMe();
  const settings = useSettings();

  const report = useQuery({
    queryKey: ['lostfound', id],
    queryFn: () => getLostFoundReport(id),
    retry: false,
  });

  const [returnOpen, setReturnOpen] = useState(false);
  const [closeOpen, setCloseOpen] = useState(false);
  const [returnConfirmOpen, setReturnConfirmOpen] = useState(false);
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const patch = useMutation({
    mutationFn: (body: Parameters<typeof updateLostFoundReport>[1]) =>
      updateLostFoundReport(id, body),
    onSuccess: (data) => {
      qc.setQueryData(['lostfound', id], data);
      qc.invalidateQueries({ queryKey: ['lostfound'] });
      setReturnOpen(false);
      setCloseOpen(false);
    },
  });

  const remove = useMutation({
    mutationFn: () => deleteLostFoundReport(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['lostfound'] });
      qc.removeQueries({ queryKey: ['lostfound', id] });
      navigate({ to: '/lost-and-found' }).catch(() => undefined);
    },
  });

  if (report.isLoading) {
    return (
      <PageContainer className="max-w-3xl">
        <LoadingRow />
      </PageContainer>
    );
  }
  if (report.isError || !report.data) {
    return (
      <PageContainer className="max-w-3xl">
        <ErrorRow message="Couldn't load this report." />
        <div className="text-center">
          <LinkButton to="/lost-and-found" variant="ghost">
            <ArrowLeft className="w-4 h-4" />
            Back to Lost &amp; Found
          </LinkButton>
        </div>
      </PageContainer>
    );
  }

  const r = report.data;
  const sacred = isSacredLostFoundReport(
    r.report_number,
    !!settings.data?.preserve_sacred_lostfound
  );
  const resolved = r.status !== 'open';
  const canResolve = can(PERMS.lostfoundResolve);
  const canReopen = can(PERMS.lostfoundReopen);
  const canEdit = can(PERMS.lostfoundEdit) && r.status === 'open' && !sacred;
  const canDelete = can(PERMS.lostfoundDelete) && !sacred;

  return (
    <PageContainer className="max-w-3xl">
      {fresh ? (
        <div className="mb-4 rounded-md border border-success/30 bg-success/10 px-4 py-3 text-sm">
          <p className="font-medium text-success">Report saved.</p>
          <p className="text-muted-foreground mt-0.5">
            Head back to Lost &amp; Found when you're done. Only mark returned or close when the item is
            actually resolved.
          </p>
        </div>
      ) : null}
      {sacred ? (
        <div className="mb-4 rounded-md border border-awo-amaranth/30 bg-awo-amaranth/10 px-4 py-3 text-sm">
          <p className="font-medium">Sacred test item</p>
          <p className="text-muted-foreground mt-0.5">
            {r.report_number} was the first report ever filed. It is immortal — editing and removing are not allowed.
          </p>
        </div>
      ) : null}
      <Card>
        <CardHeader>
          <div className="flex items-start gap-3">
            <span
              className={cn(
                'rounded-pill p-2.5 shrink-0',
                r.type === 'lost' ? 'bg-destructive/15 text-destructive' : 'bg-success/15 text-success'
              )}
            >
              {r.type === 'lost' ? <AlertTriangle className="w-5 h-5" /> : <CheckCircle2 className="w-5 h-5" />}
            </span>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 flex-wrap">
                <CardTitle className={cn('text-xl', resolved && 'line-through opacity-70')}>
                  {r.item_name}
                </CardTitle>
                <Badge variant={r.type === 'lost' ? 'destructive' : 'success'}>{r.type}</Badge>
                {r.status === 'open' ? <Badge>open</Badge> : null}
                {r.status === 'returned' ? <Badge variant="secondary">returned</Badge> : null}
                {r.status === 'closed' ? <Badge variant="outline">closed</Badge> : null}
              </div>
              <p className="text-sm text-muted-foreground mt-1 font-mono">{r.report_number}</p>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-5">
          <ReportPhoto reportId={r.id} />

          {r.description ? <p className="text-sm">{r.description}</p> : null}

          <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2 text-sm">
            {r.location ? (
              <Row label={r.type === 'found' ? 'Found at' : 'Last seen'}>
                <span className="inline-flex items-center gap-1">
                  <MapPin className="w-3.5 h-3.5" />
                  {r.location}
                </span>
              </Row>
            ) : null}
            <Row label="Reported">{formatDateTime(r.date_reported)}</Row>
            {r.reported_by_name ? <Row label="Reported by">{r.reported_by_name}</Row> : null}
            {r.contact_name ? <Row label="Guest">{r.contact_name}</Row> : null}
            {r.contact_info ? <Row label="Guest contact">{r.contact_info}</Row> : null}
            {r.returned_to ? <Row label="Returned to">{r.returned_to}</Row> : null}
            {r.resolved_at ? <Row label="Resolved">{formatDateTime(r.resolved_at)}</Row> : null}
            {r.resolved_by_name ? <Row label="Resolved by">{r.resolved_by_name}</Row> : null}
            {r.resolution_note ? <Row label="Note">{r.resolution_note}</Row> : null}
          </dl>

          {(canResolve || canReopen || canEdit || canDelete) ? (
            <div className="flex flex-wrap gap-2 pt-2 border-t border-border">
              {canEdit ? (
                <LinkButton to="/lost-and-found/$id/edit" params={{ id: r.id }} variant="outline">
                  <Pencil className="w-4 h-4" />
                  Edit
                </LinkButton>
              ) : null}
              {r.status === 'open' && canResolve ? (
                <>
                  <Button variant="outline" onClick={() => setReturnConfirmOpen(true)}>
                    <HandHeart className="w-4 h-4" />
                    Mark returned
                  </Button>
                  <Button variant="outline" onClick={() => setCloseConfirmOpen(true)}>
                    <XCircle className="w-4 h-4" />
                    Close report
                  </Button>
                </>
              ) : null}
              {resolved && canReopen ? (
                <Button
                  variant="secondary"
                  onClick={() => patch.mutate({ status: 'open' })}
                  disabled={patch.isPending}
                >
                  <RotateCcw className="w-4 h-4" />
                  Reopen
                </Button>
              ) : null}
              {canDelete ? (
                <Button variant="ghost" className="text-destructive hover:text-destructive" onClick={() => setDeleteOpen(true)}>
                  <Trash2 className="w-4 h-4" />
                  Remove
                </Button>
              ) : null}
            </div>
          ) : null}

          {patch.isError && !returnOpen && !closeOpen ? (
            <p className="text-sm text-destructive">
              {patch.error instanceof ApiError ? patch.error.message : 'Update failed.'}
            </p>
          ) : null}

          {remove.isError ? (
            <p className="text-sm text-destructive">
              {remove.error instanceof ApiError ? remove.error.message : 'Remove failed.'}
            </p>
          ) : null}

          <div className="flex items-center justify-between pt-4 border-t border-border">
            {r.type === 'found' ? (
              <Button
                variant="outline"
                onClick={() => window.open(`/lost-and-found/${r.id}/label`, '_blank')}
              >
                <Printer className="w-4 h-4" />
                Print label
              </Button>
            ) : <span />}
            <LinkButton to="/lost-and-found" variant="secondary" size="lg">
              <ArrowLeft className="w-4 h-4" />
              Back to Lost &amp; Found
            </LinkButton>
          </div>
        </CardContent>
      </Card>

      <ReturnDialog
        open={returnOpen}
        report={r}
        pending={patch.isPending}
        error={patch.isError ? patch.error : null}
        onClose={() => setReturnOpen(false)}
        onConfirm={(returnedTo, note) =>
          patch.mutate({ status: 'returned', returned_to: returnedTo, resolution_note: note || undefined })
        }
      />
      <CloseDialog
        open={closeOpen}
        pending={patch.isPending}
        error={patch.isError ? patch.error : null}
        onClose={() => setCloseOpen(false)}
        onConfirm={(note) => patch.mutate({ status: 'closed', resolution_note: note || undefined })}
      />

      <Dialog
        open={returnConfirmOpen}
        onClose={() => setReturnConfirmOpen(false)}
        title="Mark as returned?"
        description="Only continue if the owner has actually picked up the item. You'll be asked who received it next."
      >
        <div className="flex gap-2 justify-end">
          <Button type="button" variant="ghost" onClick={() => setReturnConfirmOpen(false)}>
            Cancel
          </Button>
          <Button
            type="button"
            onClick={() => {
              setReturnConfirmOpen(false);
              setReturnOpen(true);
            }}
          >
            Continue
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={closeConfirmOpen}
        onClose={() => setCloseConfirmOpen(false)}
        title="Close this report?"
        description="Only close if the item was never claimed — disposed, donated, or handed to venue lost property. This is an end-of-con outcome, not for items still being looked for."
      >
        <div className="flex gap-2 justify-end">
          <Button type="button" variant="ghost" onClick={() => setCloseConfirmOpen(false)}>
            Cancel
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={() => {
              setCloseConfirmOpen(false);
              setCloseOpen(true);
            }}
          >
            Continue
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        title="Remove this report?"
        description={`Hide ${r.report_number} (${r.item_name}) from the list? The record stays in the database — use for duplicates or reports entered by mistake.`}
      >
        {remove.isError ? (
          <p className="text-sm text-destructive mb-3">
            {remove.error instanceof ApiError ? remove.error.message : 'Remove failed.'}
          </p>
        ) : null}
        <div className="flex gap-2 justify-end">
          <Button type="button" variant="ghost" onClick={() => setDeleteOpen(false)} disabled={remove.isPending}>
            Cancel
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={remove.isPending}
            onClick={() => remove.mutate()}
          >
            {remove.isPending ? 'Removing…' : 'Remove from list'}
          </Button>
        </div>
      </Dialog>
    </PageContainer>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-2">
      <dt className="text-muted-foreground w-28 shrink-0">{label}</dt>
      <dd className="min-w-0">{children}</dd>
    </div>
  );
}

/** Loads the report's first image (if any) as an authed blob. */
function ReportPhoto({ reportId }: { reportId: string }) {
  const files = useQuery({
    queryKey: ['lostfound-files', reportId],
    queryFn: () => listFiles({ entity_type: 'lostfound', entity_id: reportId, file_type: 'image' }),
    retry: false,
  });
  const fileId = files.data?.items[0]?.id;
  const [url, setUrl] = useState<string | null>(null);

  useEffect(() => {
    let revoked: string | null = null;
    if (fileId) {
      fetchFileBlobURL(fileId)
        .then((u) => {
          revoked = u;
          setUrl(u);
        })
        .catch(() => undefined);
    }
    return () => {
      if (revoked) URL.revokeObjectURL(revoked);
    };
  }, [fileId]);

  if (!fileId || !url) return null;
  return <img src={url} alt="Report photo" className="max-h-72 rounded-lg border border-border object-contain" />;
}

function ReturnDialog({
  open,
  report,
  pending,
  error,
  onClose,
  onConfirm,
}: {
  open: boolean;
  report: LostFoundReport;
  pending: boolean;
  error: unknown;
  onClose: () => void;
  onConfirm: (returnedTo: string, note: string) => void;
}) {
  const [returnedTo, setReturnedTo] = useState('');
  const [note, setNote] = useState('');

  useEffect(() => {
    if (open) {
      setReturnedTo(report.contact_name ?? '');
      setNote('');
    }
  }, [open, report.contact_name]);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Mark as returned"
      description="Record who picked the item up so the datasheet stays accurate."
    >
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (returnedTo.trim()) onConfirm(returnedTo.trim(), note.trim());
        }}
        onKeyDown={(e) => {
          if (e.key !== 'Enter' || (!e.ctrlKey && !e.metaKey)) return;
          e.preventDefault();
          if (!pending && returnedTo.trim()) {
            e.currentTarget.requestSubmit();
          }
        }}
      >
        <Field label="Returned to" htmlFor="lf-returned-to">
          <Input
            id="lf-returned-to"
            value={returnedTo}
            onChange={(e) => setReturnedTo(e.target.value)}
            placeholder="Name of the person who claimed it"
            required
          />
        </Field>
        <Field label="Note (optional)" htmlFor="lf-return-note">
          <Textarea id="lf-return-note" rows={2} value={note} onChange={(e) => setNote(e.target.value)} />
        </Field>
        {error ? (
          <p className="text-sm text-destructive">
            {error instanceof ApiError ? error.message : 'Update failed.'}
          </p>
        ) : null}
        <div className="flex gap-2 justify-end">
          <Button type="button" variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={!returnedTo.trim() || pending}>
            {pending ? 'Saving…' : 'Mark returned'}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

function CloseDialog({
  open,
  pending,
  error,
  onClose,
  onConfirm,
}: {
  open: boolean;
  pending: boolean;
  error: unknown;
  onClose: () => void;
  onConfirm: (note: string) => void;
}) {
  const [note, setNote] = useState('');

  useEffect(() => {
    if (open) setNote('');
  }, [open]);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Close report"
      description="For items that were never claimed — disposed, donated, or handed to lost property after the con."
    >
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          onConfirm(note.trim());
        }}
      >
        <Field label="What happened to it? (optional)" htmlFor="lf-close-note">
          <Textarea
            id="lf-close-note"
            rows={2}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="e.g. donated to charity, handed to venue lost property…"
          />
        </Field>
        {error ? (
          <p className="text-sm text-destructive">
            {error instanceof ApiError ? error.message : 'Update failed.'}
          </p>
        ) : null}
        <div className="flex gap-2 justify-end">
          <Button type="button" variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="destructive" disabled={pending}>
            {pending ? 'Saving…' : 'Close report'}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
