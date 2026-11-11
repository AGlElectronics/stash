import { useEffect, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Activity, History, KeyRound, MapPin, Tags, UserX } from 'lucide-react';
import { ApiError, apiRequest } from '@/api/client';
import { getSettings, patchSettings } from '@/api/settings';
import { DenConnection } from '@/components/DenConnection';
import { PrivacyNotificationsCard } from '@/components/PrivacyNotificationsCard';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { useMe, PERMS } from '@/hooks/useMe';
import { formatUptime } from '@/lib/utils';

type HealthResponse = {
  status: string;
  database: string;
  version: string;
  uptime_sec: number;
};

export function SettingsPage() {
  const { can } = useMe();

  return (
    <PageContainer className="max-w-4xl">
      <PageHeader
        title="Settings"
        description="System toggles and administration. Anything here changes how everyone uses Stash."
      />

      <div className="space-y-5">
        {can(PERMS.accessManage) ? <DenConnection base="/api/den" /> : null}
        <SystemSettingsCard />
        {can(PERMS.accessManage) ? <PrivacyNotificationsCard /> : null}

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          {can(PERMS.locationManage) ? (
            <AdminLink
              to="/locations"
              icon={MapPin}
              title="Locations"
              description="Rooms, shelves and storage spots"
            />
          ) : null}
          {can(PERMS.categoryManage) ? (
            <AdminLink
              to="/categories"
              icon={Tags}
              title="Categories"
              description="How items are grouped"
            />
          ) : null}
          {can(PERMS.auditView) ? (
            <AdminLink
              to="/audit-log"
              icon={History}
              title="Audit log"
              description="Who changed what, when"
            />
          ) : null}
          {can(PERMS.userManage) ? (
            <AdminLink
              to="/users"
              icon={UserX}
              title="People"
              description="Anonymise people who have left"
            />
          ) : null}
        </div>

        <AccessInfoCard />
        <SystemHealthCard />
      </div>
    </PageContainer>
  );
}

function SystemSettingsCard() {
  const qc = useQueryClient();
  const settings = useQuery({ queryKey: ['system-settings'], queryFn: getSettings });
  const [forceScan, setForceScan] = useState(false);
  const [defaultOwner, setDefaultOwner] = useState('');
  const [defaultCurrency, setDefaultCurrency] = useState('EUR');
  const [barcodePrefix, setBarcodePrefix] = useState('ST');
  const [barcodeWidth, setBarcodeWidth] = useState(6);
  const [preserveSacred, setPreserveSacred] = useState(false);
  const [savedAt, setSavedAt] = useState<string | null>(null);

  useEffect(() => {
    if (!settings.data) return;
    setForceScan(settings.data.force_barcode_on_signout);
    setDefaultOwner(settings.data.default_item_owner ?? '');
    setDefaultCurrency(settings.data.default_currency || 'EUR');
    setBarcodePrefix(settings.data.barcode_prefix || 'ST');
    setBarcodeWidth(settings.data.barcode_width && settings.data.barcode_width > 0 ? settings.data.barcode_width : 6);
    setPreserveSacred(!!settings.data.preserve_sacred_lostfound);
  }, [settings.data]);

  const save = useMutation({
    mutationFn: () =>
      patchSettings({
        force_barcode_on_signout: forceScan,
        default_item_owner: defaultOwner,
        default_currency: defaultCurrency,
        barcode_prefix: barcodePrefix,
        barcode_width: barcodeWidth,
        preserve_sacred_lostfound: preserveSacred,
      }),
    onSuccess: (data) => {
      qc.setQueryData(['system-settings'], data);
      setSavedAt(new Date().toLocaleTimeString());
    },
  });

  const dirty =
    !!settings.data &&
    (settings.data.force_barcode_on_signout !== forceScan ||
      (settings.data.default_item_owner ?? '') !== defaultOwner ||
      (settings.data.default_currency || 'EUR') !== defaultCurrency ||
      (settings.data.barcode_prefix || 'ST') !== barcodePrefix ||
      (settings.data.barcode_width && settings.data.barcode_width > 0
        ? settings.data.barcode_width
        : 6) !== barcodeWidth ||
      !!settings.data.preserve_sacred_lostfound !== preserveSacred);

  return (
    <Card>
      <CardHeader>
        <CardTitle>System defaults</CardTitle>
        <CardDescription>
          Deployment-wide behaviour. The look comes from Den when connected, otherwise from{' '}
          <code className="rounded bg-muted px-1">/brand.json</code>.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        <label className="flex items-start gap-3 text-sm">
          <input
            type="checkbox"
            className="mt-1 accent-primary"
            checked={forceScan}
            onChange={(e) => setForceScan(e.target.checked)}
          />
          <span>
            <span className="font-semibold">Require barcode scan to sign out</span>
            <span className="block text-muted-foreground">
              Items without a matching scan cannot be signed out while this is on.
            </span>
          </span>
        </label>

        <label className="flex items-start gap-3 text-sm">
          <input
            type="checkbox"
            className="mt-1 accent-primary"
            checked={preserveSacred}
            onChange={(e) => setPreserveSacred(e.target.checked)}
          />
          <span>
            <span className="font-semibold">Protect Lost &amp; Found report LF-0001</span>
            <span className="block text-muted-foreground">
              An easter egg: the first report can no longer be edited or removed (retention skips
              it too). Off by default.
            </span>
          </span>
        </label>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
          <label className="space-y-1">
            <span className="font-medium">Default item owner</span>
            <input
              className="w-full rounded-md border border-border bg-background px-3 py-2"
              value={defaultOwner}
              onChange={(e) => setDefaultOwner(e.target.value)}
              placeholder="optional"
            />
          </label>
          <label className="space-y-1">
            <span className="font-medium">Default currency</span>
            <input
              className="w-full rounded-md border border-border bg-background px-3 py-2"
              value={defaultCurrency}
              onChange={(e) => setDefaultCurrency(e.target.value.toUpperCase())}
              placeholder="EUR"
            />
          </label>
          <label className="space-y-1">
            <span className="font-medium">Barcode prefix</span>
            <input
              className="w-full rounded-md border border-border bg-background px-3 py-2"
              value={barcodePrefix}
              onChange={(e) => setBarcodePrefix(e.target.value)}
              placeholder="ST"
            />
          </label>
          <label className="space-y-1">
            <span className="font-medium">Barcode width</span>
            <input
              type="number"
              min={3}
              max={12}
              className="w-full rounded-md border border-border bg-background px-3 py-2"
              value={barcodeWidth}
              onChange={(e) => setBarcodeWidth(Number(e.target.value) || 6)}
            />
          </label>
        </div>

        <div className="flex items-center gap-3">
          <Button onClick={() => save.mutate()} disabled={!dirty || save.isPending} type="button">
            {save.isPending ? 'Saving…' : 'Save'}
          </Button>
          {save.isError ? (
            <span className="text-sm text-destructive">
              {save.error instanceof ApiError ? save.error.message : 'Failed to save settings.'}
            </span>
          ) : null}
          {!save.isError && savedAt ? (
            <span className="text-sm text-muted-foreground">Saved at {savedAt}.</span>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}

function AdminLink({
  to,
  icon: Icon,
  title,
  description,
}: {
  to: string;
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  description: string;
}) {
  return (
    <Link
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      to={to as any}
      className="block focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-card"
    >
      <Card clickable className="h-full">
        <CardContent className="p-4 flex items-start gap-3">
          <span className="rounded-pill bg-primary-soft text-primary p-2 shrink-0">
            <Icon className="w-4 h-4" />
          </span>
          <span>
            <span className="block font-heading font-bold text-sm">{title}</span>
            <span className="block text-xs text-muted-foreground mt-0.5">{description}</span>
          </span>
        </CardContent>
      </Card>
    </Link>
  );
}

function AccessInfoCard() {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <KeyRound className="w-4 h-4" />
          Access control
        </CardTitle>
        <CardDescription>
          Roles are assigned in HR (or directly in Keycloak), not in this UI. Every role is global and
          appears in HR as <code>stash_&lt;role&gt;</code>, for example <code>stash_volunteer</code>.
          What each role may do is set in <code>access-config.toml</code> on the backend. Your own
          access updates the next time your sign-in is renewed.
        </CardDescription>
      </CardHeader>
    </Card>
  );
}

function SystemHealthCard() {
  const health = useQuery({
    queryKey: ['health'],
    queryFn: () => apiRequest<HealthResponse>('/api/health'),
    refetchInterval: 30_000,
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Activity className="w-4 h-4" />
          System health
        </CardTitle>
      </CardHeader>
      <CardContent>
        {health.isLoading ? (
          <p className="text-sm text-muted-foreground">Checking…</p>
        ) : health.isError ? (
          <p className="text-sm text-destructive">Unable to reach the API.</p>
        ) : health.data ? (
          <dl className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm">
            <div>
              <dt className="text-muted-foreground text-xs uppercase tracking-wide mb-1">Status</dt>
              <dd>
                <Badge variant={health.data.status === 'ok' ? 'success' : 'destructive'}>
                  {health.data.status}
                </Badge>
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground text-xs uppercase tracking-wide mb-1">Database</dt>
              <dd>
                <Badge variant={health.data.database === 'ok' ? 'success' : 'destructive'}>
                  {health.data.database}
                </Badge>
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground text-xs uppercase tracking-wide mb-1">Version</dt>
              <dd className="font-mono text-xs">{health.data.version}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground text-xs uppercase tracking-wide mb-1">Uptime</dt>
              <dd className="tabular-nums">{formatUptime(health.data.uptime_sec)}</dd>
            </div>
          </dl>
        ) : null}
      </CardContent>
    </Card>
  );
}
