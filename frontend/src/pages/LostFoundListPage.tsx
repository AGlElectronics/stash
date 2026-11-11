import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import {
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Download,
  MapPin,
  Plus,
  Search,
} from 'lucide-react';
import { listLostFound, type LostFoundReport, type LostFoundType } from '@/api/lostfound';
import { Badge } from '@/components/ui/Badge';
import { LinkButton } from '@/components/ui/Button';
import { Button } from '@/components/ui/Button';
import { Card, CardContent } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { EmptyState, ErrorRow, LoadingRow } from '@/components/ui/States';
import { Tabs } from '@/components/ui/Tabs';
import { useMe, PERMS } from '@/hooks/useMe';
import { fetchAuthed } from '@/api/client';
import { downloadBlob } from '@/lib/csvExport';
import { formatDateTime, cn } from '@/lib/utils';

type TypeTab = 'all' | LostFoundType;

export function LostFoundListPage() {
  const { can } = useMe();
  const [tab, setTab] = useState<TypeTab>('all');
  const [search, setSearch] = useState('');
  const [showResolved, setShowResolved] = useState(false);

  const reports = useQuery({
    queryKey: ['lostfound'],
    queryFn: () => listLostFound({ limit: 500 }),
    retry: false,
  });

  const filtered = useMemo(() => {
    let rows = reports.data?.items ?? [];
    if (tab !== 'all') rows = rows.filter((r) => r.type === tab);
    const q = search.trim().toLowerCase();
    if (q) {
      rows = rows.filter((r) =>
        [r.report_number, r.item_name, r.description, r.location, r.contact_name ?? '']
          .join(' ')
          .toLowerCase()
          .includes(q)
      );
    }
    return rows;
  }, [reports.data, tab, search]);

  const open = filtered.filter((r) => r.status === 'open');
  const resolved = filtered.filter((r) => r.status !== 'open');

  const all = reports.data?.items ?? [];
  const countFor = (t: TypeTab) =>
    (t === 'all' ? all : all.filter((r) => r.type === t)).filter((r) => r.status === 'open').length;

  const [exportError, setExportError] = useState('');

  async function exportCsv() {
    setExportError('');
    const res = await fetchAuthed('/api/export/lostfound.csv');
    if (!res.ok) {
      setExportError('Export failed. You need the export.create permission.');
      return;
    }
    const stamp = new Date().toISOString().slice(0, 10);
    downloadBlob(`lost-and-found-${stamp}.csv`, await res.blob());
  }

  return (
    <PageContainer className="max-w-page">
      <PageHeader
        title="Lost & Found"
        description="Report lost or found belongings and track them until they find their way home."
        actions={
          <>
            {can(PERMS.exportCreate) && all.length > 0 ? (
              <Button variant="secondary" onClick={exportCsv}>
                <Download className="w-4 h-4" />
                Export datasheet
              </Button>
            ) : null}
            {can(PERMS.lostfoundCreate) ? (
              <LinkButton to="/lost-and-found/new">
                <Plus className="w-4 h-4" />
                New report
              </LinkButton>
            ) : null}
          </>
        }
      />
      {exportError ? (
        <p role="alert" className="text-sm text-destructive mb-3">
          {exportError}
        </p>
      ) : null}

      <div className="flex flex-col sm:flex-row gap-3 mb-5">
        <Tabs<TypeTab>
          tabs={[
            { id: 'all', label: 'All', count: reports.data ? countFor('all') : undefined },
            { id: 'lost', label: 'Lost', icon: AlertTriangle, count: reports.data ? countFor('lost') : undefined },
            { id: 'found', label: 'Found', icon: CheckCircle2, count: reports.data ? countFor('found') : undefined },
          ]}
          value={tab}
          onChange={setTab}
        />
        <div className="relative flex-1 sm:max-w-xs sm:ml-auto">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search number, item, place…"
            className="pl-9"
          />
        </div>
      </div>

      {reports.isLoading ? (
        <LoadingRow />
      ) : reports.isError ? (
        <ErrorRow message="Lost & Found isn't available yet — the backend for it hasn't been deployed." />
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={Search}
          title="No reports here"
          description={
            search
              ? 'Nothing matches your search. Try fewer words or a different spelling.'
              : 'Nothing has been reported yet. When something goes missing or turns up, log it here.'
          }
          action={
            can(PERMS.lostfoundCreate) ? (
              <LinkButton to="/lost-and-found/new">
                <Plus className="w-4 h-4" />
                New report
              </LinkButton>
            ) : undefined
          }
        />
      ) : (
        <div className="space-y-6">
          <section>
            <h2 className="font-heading font-bold text-sm uppercase tracking-wide text-muted-foreground mb-3">
              Open ({open.length})
            </h2>
            {open.length === 0 ? (
              <p className="text-sm text-muted-foreground">No open reports in this view.</p>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {open.map((r) => (
                  <ReportCard key={r.id} report={r} />
                ))}
              </div>
            )}
          </section>

          {resolved.length > 0 ? (
            <section>
              <button
                type="button"
                onClick={() => setShowResolved((v) => !v)}
                className="flex items-center gap-1.5 font-heading font-bold text-sm uppercase tracking-wide text-muted-foreground mb-3 hover:text-foreground transition-colors"
              >
                {showResolved ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                Resolved ({resolved.length})
              </button>
              {showResolved ? (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  {resolved.map((r) => (
                    <ReportCard key={r.id} report={r} />
                  ))}
                </div>
              ) : null}
            </section>
          ) : null}
        </div>
      )}
    </PageContainer>
  );
}

function ReportCard({ report }: { report: LostFoundReport }) {
  const resolved = report.status !== 'open';
  return (
    <Link to="/lost-and-found/$id" params={{ id: report.id }} className="block focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-card">
      <Card clickable className={cn(resolved && 'opacity-60')}>
        <CardContent className="p-4">
          <div className="flex items-start gap-3">
            <span
              className={cn(
                'rounded-pill p-2 shrink-0 mt-0.5',
                report.type === 'lost' ? 'bg-destructive/15 text-destructive' : 'bg-success/15 text-success'
              )}
            >
              {report.type === 'lost' ? (
                <AlertTriangle className="w-4 h-4" />
              ) : (
                <CheckCircle2 className="w-4 h-4" />
              )}
            </span>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 flex-wrap">
                <p className={cn('font-semibold truncate', resolved && 'line-through')}>{report.item_name}</p>
                <Badge variant={report.type === 'lost' ? 'destructive' : 'success'}>{report.type}</Badge>
                {report.status === 'returned' ? <Badge variant="secondary">returned</Badge> : null}
                {report.status === 'closed' ? <Badge variant="outline">closed</Badge> : null}
              </div>
              {report.description ? (
                <p className="text-sm text-muted-foreground truncate mt-0.5">{report.description}</p>
              ) : null}
              <p className="text-xs text-muted-foreground mt-1.5 flex items-center gap-3 flex-wrap">
                <span className="font-mono">{report.report_number}</span>
                {report.location ? (
                  <span className="inline-flex items-center gap-1">
                    <MapPin className="w-3 h-3" />
                    {report.location}
                  </span>
                ) : null}
                <span>{formatDateTime(report.date_reported)}</span>
              </p>
            </div>
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
