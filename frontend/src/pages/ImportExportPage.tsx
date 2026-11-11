import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Download, FileUp, ShieldCheck } from 'lucide-react';
import { ApiError } from '@/api/client';
import { downloadExport, importItemsCsv, listPlannedReports, type ImportResult } from '@/api/importExport';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { PERMS, useMe } from '@/hooks/useMe';

const today = () => new Date().toISOString().slice(0, 10);

/** CSV import / bulk update of items, and ready-made exports. */
export function ImportExportPage() {
  const { can, canAny } = useMe();
  return (
    <PageContainer>
      <PageHeader
        title="Import & export"
        description="Add or change many items from a spreadsheet, and download ready-made lists."
      />
      <div className="space-y-5">
        {canAny(PERMS.itemCreate, PERMS.itemEdit) ? <ImportCard /> : null}
        {can(PERMS.exportCreate) ? <ExportsCard /> : null}
      </div>
    </PageContainer>
  );
}

function ImportCard() {
  const [file, setFile] = useState<File | null>(null);
  const [report, setReport] = useState<ImportResult | null>(null);
  const run = useMutation({
    mutationFn: ({ dryRun }: { dryRun: boolean }) => importItemsCsv(file as File, dryRun),
    onSuccess: setReport,
  });
  const checkedOK = report?.dry_run && report.failed === 0;

  return (
    <Card>
      <CardHeader>
        <CardTitle>Import items from CSV</CardTitle>
        <CardDescription>
          One row per item, with a header row. Rows without an <code>id</code> create new items; rows with
          the <code>id</code> of an existing item change it, and empty cells leave a field as it is. The
          items export (below) is a good starting point. Departments, categories and locations must exist
          unless you may manage them. Check the file first: nothing is saved unless every row is fine.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-center gap-2">
          <input
            type="file"
            accept=".csv,text/csv"
            aria-label="CSV file"
            onChange={(e) => {
              setFile(e.target.files?.[0] ?? null);
              setReport(null);
            }}
            className="text-sm"
          />
          <Button variant="outline" disabled={!file || run.isPending} onClick={() => run.mutate({ dryRun: true })}>
            <ShieldCheck className="w-4 h-4" />
            Check file
          </Button>
          <Button disabled={!file || !checkedOK || run.isPending} onClick={() => run.mutate({ dryRun: false })}>
            <FileUp className="w-4 h-4" />
            Import
          </Button>
        </div>
        {run.isError ? (
          <p role="alert" className="text-sm text-destructive">
            {run.error instanceof ApiError ? run.error.message : 'Import failed.'}
          </p>
        ) : null}
        {report ? <ImportReport report={report} /> : null}
      </CardContent>
    </Card>
  );
}

function ImportReport({ report }: { report: ImportResult }) {
  const problems = report.rows.filter((r) => r.action === 'error');
  return (
    <div className="space-y-3">
      <p className="text-sm" role="status">
        {report.applied
          ? `Saved: ${report.created} created, ${report.updated} changed.`
          : report.failed > 0
            ? `${report.failed} row(s) have problems; nothing was saved. Fix them and check again.`
            : `Ready: ${report.created} to create, ${report.updated} to change. Press Import to save.`}
      </p>
      {problems.length > 0 ? (
        <Table>
          <THead>
            <TR>
              <TH>Row</TH>
              <TH>Name</TH>
              <TH>Problem</TH>
            </TR>
          </THead>
          <TBody>
            {problems.map((r) => (
              <TR key={r.row}>
                <TD className="tabular-nums">{r.row}</TD>
                <TD>{r.name || '—'}</TD>
                <TD className="text-destructive">{r.errors?.join('; ')}</TD>
              </TR>
            ))}
          </TBody>
        </Table>
      ) : null}
      <div className="flex gap-2 flex-wrap text-xs">
        <Badge variant="success">{report.created} create</Badge>
        <Badge variant="secondary">{report.updated} change</Badge>
        {report.failed ? <Badge variant="destructive">{report.failed} problems</Badge> : null}
      </div>
    </div>
  );
}

function ExportsCard() {
  const reports = useQuery({ queryKey: ['planned-reports'], queryFn: listPlannedReports });
  const [failed, setFailed] = useState('');
  const fixed = [
    { key: 'items', title: 'All items', description: 'Every item that is not retired, for editing and re-importing.', path: '/api/export/items.csv' },
    { key: 'signouts', title: 'All sign-outs', description: 'Every sign-out with its status and dates.', path: '/api/export/signouts.csv' },
  ];
  const all = [
    ...fixed,
    ...(reports.data?.items ?? []).map((r) => ({ ...r, path: `/api/export/reports/${r.key}.csv` })),
  ];
  return (
    <Card>
      <CardHeader>
        <CardTitle>Exports</CardTitle>
        <CardDescription>CSV files for spreadsheets. Every download is recorded in the audit log.</CardDescription>
      </CardHeader>
      <CardContent className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {all.map((r) => (
          <button
            key={r.key}
            type="button"
            className="text-left rounded-md border border-border p-3 hover:bg-muted/40 transition-colors"
            onClick={async () => {
              setFailed('');
              const ok = await downloadExport(r.path, `${r.key}-${today()}.csv`);
              if (!ok) setFailed(r.title);
            }}
          >
            <span className="flex items-center gap-2 font-semibold text-sm">
              <Download className="w-4 h-4" />
              {r.title}
            </span>
            <span className="block text-xs text-muted-foreground mt-1">{r.description}</span>
          </button>
        ))}
        {failed ? (
          <p role="alert" className="text-sm text-destructive sm:col-span-2">
            Downloading “{failed}” failed.
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}
