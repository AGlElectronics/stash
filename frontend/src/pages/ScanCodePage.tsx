import { useEffect, useState } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { ApiError } from '@/api/client';
import { lookupItemByBarcode } from '@/api/items';
import { lookupLostFoundByReportNumber } from '@/api/lostfound';
import { LinkButton } from '@/components/ui/Button';
import { Card, CardContent } from '@/components/ui/Card';
import { PageContainer } from '@/components/ui/PageHeader';
import { LoadingRow } from '@/components/ui/States';
import { isLostFoundReportNumber, normalizeLostFoundReportNumber } from '@/lib/scanCode';

/**
 * Target of QR labels (/scan?code=ST000042): opens the item or Lost & Found
 * report the code belongs to. A phone's camera app lands here directly.
 */
export function ScanCodePage() {
  const { code = '' } = useSearch({ strict: false }) as { code?: string };
  const navigate = useNavigate();
  const [problem, setProblem] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const value = code.trim();
    if (!value) {
      setProblem('This link has no code. Use Scan in the menu instead.');
      return;
    }
    (async () => {
      try {
        if (isLostFoundReportNumber(value)) {
          const report = await lookupLostFoundByReportNumber(normalizeLostFoundReportNumber(value));
          if (!cancelled) await navigate({ to: '/lost-and-found/$id', params: { id: report.id }, replace: true });
          return;
        }
        const item = await lookupItemByBarcode(value);
        if (!cancelled) await navigate({ to: '/items/$id', params: { id: item.id }, replace: true });
      } catch (err) {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 404) {
          setProblem(`Nothing in Stash has the code ${value}.`);
        } else if (err instanceof ApiError && err.status === 403) {
          setProblem(`You are not allowed to open ${value}.`);
        } else {
          setProblem('Lookup failed. Please try again.');
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [code, navigate]);

  return (
    <PageContainer className="max-w-lg">
      {problem ? (
        <Card>
          <CardContent className="py-8 text-center space-y-4">
            <p className="text-sm">{problem}</p>
            <LinkButton to="/dashboard" variant="secondary">
              Go to the dashboard
            </LinkButton>
          </CardContent>
        </Card>
      ) : (
        <LoadingRow />
      )}
    </PageContainer>
  );
}
