import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Search } from 'lucide-react';
import { listAuditLog } from '@/api/audit';
import { Input, Select } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { Badge } from '@/components/ui/Badge';
import { ErrorRow, EmptyRow, LoadingRow } from '@/components/ui/States';
import { formatDateTime } from '@/lib/utils';

const ENTITY_TYPES = [
  '',
  'item',
  'signout',
  'lostfound',
  'file',
  'department',
  'category',
  'location',
  'settings',
  'den',
  'export',
  'user',
];

function formatActor(entry: { actor_display_name?: string; actor_oidc_subject?: string }): string {
  const name = entry.actor_display_name?.trim();
  const sub = entry.actor_oidc_subject?.trim();
  if (name && sub) return `${name} · ${sub.slice(0, 8)}…`;
  if (name) return name;
  if (sub) return sub;
  return '—';
}

export function AuditLogPage() {
  const [action, setAction] = useState('');
  const [entityType, setEntityType] = useState('');

  const params = useMemo(
    () => ({ action: action || undefined, entity_type: entityType || undefined, limit: 200 }),
    [action, entityType]
  );
  const q = useQuery({ queryKey: ['audit', params], queryFn: () => listAuditLog(params) });

  return (
    <PageContainer>
      <PageHeader
        title="Audit log"
        description="Every state-changing action recorded with before/after snapshots."
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-4">
        <div className="relative">
          <Search className="w-4 h-4 absolute left-3 top-2.5 text-muted-foreground pointer-events-none" />
          <Input
            placeholder="Filter by exact action (e.g. item.create)"
            value={action}
            onChange={(e) => setAction(e.target.value)}
            className="pl-9"
          />
        </div>
        <Select value={entityType} onChange={(e) => setEntityType(e.target.value)}>
          {ENTITY_TYPES.map((t) => (
            <option key={t} value={t}>
              {t === '' ? 'All entity types' : t}
            </option>
          ))}
        </Select>
      </div>

      <Table>
        <THead>
          <TR>
            <TH>When</TH>
            <TH>Actor</TH>
            <TH>Action</TH>
            <TH>Entity</TH>
            <TH>Entity ID</TH>
            <TH>Note</TH>
          </TR>
        </THead>
        <TBody>
          {q.isLoading ? (
            <TR>
              <TD colSpan={6}>
                <LoadingRow />
              </TD>
            </TR>
          ) : q.isError ? (
            <TR>
              <TD colSpan={6}>
                <ErrorRow message="Failed to load audit log." />
              </TD>
            </TR>
          ) : !q.data?.items.length ? (
            <TR>
              <TD colSpan={6}>
                <EmptyRow message="No entries." />
              </TD>
            </TR>
          ) : (
            q.data.items.map((a) => (
              <TR key={a.id}>
                <TD className="whitespace-nowrap text-xs text-muted-foreground">{formatDateTime(a.created_at)}</TD>
                <TD className="text-sm whitespace-nowrap" title={a.actor_oidc_subject}>
                  {formatActor(a)}
                </TD>
                <TD>
                  <Badge variant="secondary">{a.action}</Badge>
                </TD>
                <TD>{a.entity_type}</TD>
                <TD className="font-mono text-xs">{a.entity_id.slice(0, 8)}</TD>
                <TD className="max-w-md truncate text-muted-foreground">{a.note || '-'}</TD>
              </TR>
            ))
          )}
        </TBody>
      </Table>
    </PageContainer>
  );
}
