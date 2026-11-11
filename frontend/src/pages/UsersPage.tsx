import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { UserX } from 'lucide-react';
import { ApiError } from '@/api/client';
import { anonymiseUser, listUsers, type StashUser } from '@/api/users';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { EmptyRow, ErrorRow, LoadingRow } from '@/components/ui/States';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { useMe } from '@/hooks/useMe';
import { formatDateTime } from '@/lib/utils';

/**
 * People who have signed in to Stash. Someone who left can be anonymised:
 * their name and email disappear from Stash and its history, their actions
 * stay as "Former user".
 */
export function UsersPage() {
  const qc = useQueryClient();
  const { me } = useMe();
  const users = useQuery({ queryKey: ['users'], queryFn: () => listUsers({ limit: 500 }) });
  const [target, setTarget] = useState<StashUser | null>(null);

  const anonymise = useMutation({
    mutationFn: (id: string) => anonymiseUser(id),
    onSuccess: () => {
      setTarget(null);
      qc.invalidateQueries({ queryKey: ['users'] });
    },
  });

  return (
    <PageContainer>
      <PageHeader
        title="People"
        description="Everyone who has signed in to Stash. Anonymise people who have left; Stash can also do this automatically (Settings → Privacy)."
      />
      <Table>
        <THead>
          <TR>
            <TH>Name</TH>
            <TH>Email</TH>
            <TH>Last sign-in</TH>
            <TH />
          </TR>
        </THead>
        <TBody>
          {users.isLoading ? (
            <TR>
              <TD colSpan={4}>
                <LoadingRow />
              </TD>
            </TR>
          ) : users.isError ? (
            <TR>
              <TD colSpan={4}>
                <ErrorRow message="Failed to load people." />
              </TD>
            </TR>
          ) : !users.data?.items.length ? (
            <TR>
              <TD colSpan={4}>
                <EmptyRow message="Nobody has signed in yet." />
              </TD>
            </TR>
          ) : (
            users.data.items.map((u) => (
              <TR key={u.id}>
                <TD>{u.display_name}</TD>
                <TD className="text-muted-foreground">{u.email || '—'}</TD>
                <TD className="text-xs text-muted-foreground whitespace-nowrap">
                  {u.last_login_at ? formatDateTime(u.last_login_at) : '—'}
                </TD>
                <TD className="text-right">
                  {u.anonymised_at ? (
                    <Badge variant="outline">Anonymised</Badge>
                  ) : u.id !== me?.id ? (
                    <Button variant="ghost" size="sm" onClick={() => setTarget(u)}>
                      <UserX className="w-4 h-4" />
                      Anonymise
                    </Button>
                  ) : null}
                </TD>
              </TR>
            ))
          )}
        </TBody>
      </Table>

      <Dialog open={target !== null} onClose={() => setTarget(null)} title={`Anonymise ${target?.display_name ?? ''}?`}>
        <p className="text-sm text-muted-foreground mb-4">
          Their name and email are removed from Stash, including the audit log and item history, which
          will show “Former user”. This cannot be undone. If they sign in again they start as a new person.
        </p>
        {anonymise.isError ? (
          <p role="alert" className="text-sm text-destructive mb-3">
            {anonymise.error instanceof ApiError ? anonymise.error.message : 'Anonymising failed.'}
          </p>
        ) : null}
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setTarget(null)}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={anonymise.isPending}
            onClick={() => target && anonymise.mutate(target.id)}
          >
            Anonymise
          </Button>
        </div>
      </Dialog>
    </PageContainer>
  );
}
