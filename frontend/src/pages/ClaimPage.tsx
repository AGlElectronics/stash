import { useState } from 'react';
import { Link, useParams } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { HandHeart, MapPin, PackageOpen, Search } from 'lucide-react';
import {
  getPublicReport,
  searchPublicReports,
  type PublicLostFoundReport,
} from '@/api/publicLostfound';
import { Button, LinkButton } from '@/components/ui/Button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { isPublicClaimEnabled } from '@/lib/publicClaim';
import { useBrand } from '@/brand/BrandProvider';

/**
 * Public claim page — no auth required. Guests reach this by browsing /claim
 * when VITE_PUBLIC_CLAIM_ENABLED=true (disabled by default during the con).
 */
export function ClaimPage() {
  const { reportNumber } = useParams({ strict: false }) as { reportNumber?: string };

  if (!isPublicClaimEnabled()) {
    return <ClaimDisabled />;
  }

  if (reportNumber) {
    return <ClaimDetail reportNumber={reportNumber} />;
  }
  return <ClaimBrowse />;
}

function ClaimDisabled() {
  const { brand } = useBrand();
  return (
    <PublicShell>
      <Card className="max-w-lg mx-auto">
        <CardContent className="py-10 text-center space-y-3">
          <PackageOpen className="w-10 h-10 mx-auto text-muted-foreground" />
          <h2 className="font-heading text-lg font-bold">Claim lookup unavailable</h2>
          <p className="text-sm text-muted-foreground">
            Online claim lookup is not active during the event. Please visit the{' '}
            {brand.publicDeskName} with your item or a description of what you lost.
          </p>
        </CardContent>
      </Card>
    </PublicShell>
  );
}

function ClaimDetail({ reportNumber }: { reportNumber: string }) {
  const report = useQuery({
    queryKey: ['public-lostfound', reportNumber],
    queryFn: () => getPublicReport(reportNumber),
    retry: false,
  });

  if (report.isLoading) {
    return (
      <PublicShell>
        <div className="text-center text-sm text-muted-foreground py-12">Loading…</div>
      </PublicShell>
    );
  }

  if (report.isError || !report.data) {
    return (
      <PublicShell>
        <Card className="max-w-lg mx-auto">
          <CardContent className="py-10 text-center space-y-3">
            <PackageOpen className="w-10 h-10 mx-auto text-muted-foreground" />
            <p className="text-sm text-muted-foreground">
              {report.error instanceof Error
                ? report.error.message
                : 'This report was not found. It may have already been resolved.'}
            </p>
            <LinkButton to="/claim" variant="secondary">
              Browse all found items
            </LinkButton>
          </CardContent>
        </Card>
      </PublicShell>
    );
  }

  return (
    <PublicShell>
      <div className="max-w-lg mx-auto space-y-4">
        <LinkButton to="/claim" variant="ghost" size="sm">
          ← All found items
        </LinkButton>
        <ReportCard report={report.data} />
      </div>
    </PublicShell>
  );
}

function ClaimBrowse() {
  const [query, setQuery] = useState('');
  const [submitted, setSubmitted] = useState('');

  const browsing = submitted.length === 0;

  const browse = useQuery({
    queryKey: ['public-lostfound-browse'],
    queryFn: () => searchPublicReports({ limit: 50 }),
    enabled: browsing,
  });

  const search = useQuery({
    queryKey: ['public-lostfound-search', submitted],
    queryFn: () => searchPublicReports({ q: submitted, limit: 50 }),
    enabled: !browsing,
  });

  const active = browsing ? browse : search;
  const items = active.data?.items ?? [];

  return (
    <PublicShell wide>
      <div className="space-y-6">
        <div className="text-center space-y-2 max-w-2xl mx-auto">
          <h2 className="text-xl font-semibold">Find your item</h2>
          <p className="text-sm text-muted-foreground">
            Browse everything turned in below, or search by name or description.
          </p>
        </div>

        <form
          className="flex gap-2 max-w-2xl mx-auto"
          onSubmit={(e) => {
            e.preventDefault();
            setSubmitted(query.trim());
          }}
        >
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="e.g. blue backpack, phone, keys…"
            className="flex-1"
          />
          <Button type="submit" disabled={!query.trim()}>
            <Search className="w-4 h-4" />
            Search
          </Button>
        </form>

        {submitted ? (
          <div className="text-center">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                setQuery('');
                setSubmitted('');
              }}
            >
              Clear search — show all found items
            </Button>
          </div>
        ) : null}

        {active.isLoading ? (
          <p className="text-center text-sm text-muted-foreground py-6">
            {browsing ? 'Loading found items…' : 'Searching…'}
          </p>
        ) : null}

        {active.isError ? (
          <Card className="max-w-lg mx-auto">
            <CardContent className="py-10 text-center space-y-2">
              <p className="text-sm text-muted-foreground">
                {browsing
                  ? 'Could not load found items. Please try again or visit the Con Ops desk.'
                  : 'Search failed. Please try again.'}
              </p>
            </CardContent>
          </Card>
        ) : null}

        {active.data && items.length === 0 ? (
          <Card className="max-w-lg mx-auto">
            <CardContent className="py-10 text-center space-y-2">
              <PackageOpen className="w-8 h-8 mx-auto text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                {submitted
                  ? 'No matching found items. Try different keywords or check back later.'
                  : 'No open found items right now. Check back later or visit the Con Ops desk.'}
              </p>
            </CardContent>
          </Card>
        ) : null}

        {items.length > 0 ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((item) => (
              <Link
                key={item.report_number}
                to="/claim/$reportNumber"
                params={{ reportNumber: item.report_number }}
                className="block rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <ReportCard report={item} compact />
              </Link>
            ))}
          </div>
        ) : null}

        {active.data && active.data.total > items.length ? (
          <p className="text-center text-xs text-muted-foreground">
            Showing {items.length} of {active.data.total} open found items.
          </p>
        ) : null}

        <p className="text-center text-xs text-muted-foreground pt-2">
          Only open &quot;found&quot; items are listed. If yours isn&apos;t here, visit the Con Ops desk.
        </p>
      </div>
    </PublicShell>
  );
}

function ReportCard({ report, compact }: { report: PublicLostFoundReport; compact?: boolean }) {
  const [showInstructions, setShowInstructions] = useState(false);

  return (
    <Card className={compact ? 'h-full transition-colors hover:bg-muted/30' : ''}>
      <CardHeader>
        <div className="flex items-center gap-2">
          <span className="rounded-pill p-2 bg-success/15 text-success shrink-0">
            <PackageOpen className="w-5 h-5" />
          </span>
          <div className="min-w-0 flex-1">
            <CardTitle className={compact ? 'text-base' : 'text-xl'}>{report.item_name}</CardTitle>
            <p className="text-xs text-muted-foreground font-mono">{report.report_number}</p>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {report.photo_url ? (
          <img
            src={report.photo_url}
            alt={report.item_name}
            className="max-h-48 rounded-lg border border-border object-contain w-full bg-muted/20"
            loading="lazy"
            onError={(e) => {
              e.currentTarget.style.display = 'none';
            }}
          />
        ) : null}

        {report.description ? (
          <p className={compact ? 'text-xs line-clamp-3' : 'text-sm'}>{report.description}</p>
        ) : null}

        {report.location ? (
          <p className="text-xs text-muted-foreground inline-flex items-center gap-1">
            <MapPin className="w-3.5 h-3.5 shrink-0" />
            Found at: {report.location}
          </p>
        ) : null}

        {report.date_reported ? (
          <p className="text-xs text-muted-foreground">
            Reported: {new Date(report.date_reported).toLocaleDateString()}
          </p>
        ) : null}

        {compact ? null : showInstructions ? (
          <div className="rounded-lg bg-success/10 border border-success/20 p-4 space-y-2">
            <p className="text-sm font-medium flex items-center gap-2">
              <HandHeart className="w-4 h-4 text-success" />
              How to claim this item
            </p>
            <ClaimInstructions />
          </div>
        ) : (
          <Button className="w-full" onClick={() => setShowInstructions(true)}>
            <HandHeart className="w-4 h-4" />
            This is mine!
          </Button>
        )}
      </CardContent>
    </Card>
  );
}

function ClaimInstructions() {
  const { brand } = useBrand();
  return (
    <ol className="text-sm space-y-1 list-decimal list-inside text-muted-foreground">
      <li>
        Come to the <strong>{brand.publicDeskName}</strong>
      </li>
      <li>Show this page or the QR label to a staff member</li>
      <li>Describe the item to confirm it&apos;s yours</li>
      <li>The staff member will hand it over and mark it returned</li>
    </ol>
  );
}

function PublicShell({ children, wide }: { children: React.ReactNode; wide?: boolean }) {
  const { brand } = useBrand();
  const maxWidth = wide ? 'max-w-6xl' : 'max-w-3xl';
  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="border-b border-border bg-card/50 backdrop-blur-sm sticky top-0 z-10">
        <div className={`${maxWidth} mx-auto px-4 py-3 flex items-center gap-3`}>
          <PackageOpen className="w-6 h-6 text-primary" />
          <div>
            <h1 className="font-heading text-lg font-bold leading-tight">
              {brand.productName} — Lost &amp; Found
            </h1>
            <p className="text-xs text-muted-foreground">{brand.claimHeaderSubtitle}</p>
          </div>
        </div>
      </header>
      <main className={`${maxWidth} mx-auto px-4 py-8`}>{children}</main>
    </div>
  );
}
