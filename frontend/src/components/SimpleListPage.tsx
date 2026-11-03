import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Trash2 } from 'lucide-react';
import { ApiError } from '@/api/client';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Input, Textarea } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { EmptyRow, ErrorRow, LoadingRow } from '@/components/ui/States';

type Row = { id: string; name: string; description: string };

type Props = {
  title: string;
  description: string;
  resourceLabel: string;
  queryKey: string[];
  query: () => unknown;
  rows: Row[] | undefined;
  isLoading: boolean;
  isError: boolean;
  onCreate: (body: { name: string; description: string }) => Promise<unknown>;
  onDelete: (id: string) => Promise<unknown>;
  extraColumns?: Array<{ header: string; render: (row: Row) => React.ReactNode }>;
};

export function SimpleResourcePage(props: Props) {
  const qc = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [name, setName] = useState('');
  const [desc, setDesc] = useState('');
  const [error, setError] = useState<string | null>(null);

  const createMutation = useMutation({
    mutationFn: () => props.onCreate({ name, description: desc }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: props.queryKey });
      setDialogOpen(false);
      setName('');
      setDesc('');
      setError(null);
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : `Failed to create ${props.resourceLabel}`);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => props.onDelete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: props.queryKey });
    },
  });

  const colspan = 3 + (props.extraColumns?.length ?? 0);

  return (
    <PageContainer>
      <PageHeader
        title={props.title}
        description={props.description}
        actions={
          <Button onClick={() => setDialogOpen(true)}>
            <Plus className="w-4 h-4" />
            New {props.resourceLabel}
          </Button>
        }
      />

      <Table>
        <THead>
          <TR>
            <TH>Name</TH>
            <TH>Description</TH>
            {props.extraColumns?.map((c) => (
              <TH key={c.header}>{c.header}</TH>
            ))}
            <TH className="w-16 text-right">Actions</TH>
          </TR>
        </THead>
        <TBody>
          {props.isLoading ? (
            <TR>
              <TD colSpan={colspan}>
                <LoadingRow />
              </TD>
            </TR>
          ) : props.isError ? (
            <TR>
              <TD colSpan={colspan}>
                <ErrorRow message={`Failed to load ${props.resourceLabel}s.`} />
              </TD>
            </TR>
          ) : !props.rows || props.rows.length === 0 ? (
            <TR>
              <TD colSpan={colspan}>
                <EmptyRow message={`No ${props.resourceLabel}s yet.`} />
              </TD>
            </TR>
          ) : (
            props.rows.map((row) => (
              <TR key={row.id}>
                <TD className="font-medium">{row.name}</TD>
                <TD className="text-muted-foreground">{row.description || '-'}</TD>
                {props.extraColumns?.map((c) => (
                  <TD key={c.header}>{c.render(row)}</TD>
                ))}
                <TD className="text-right">
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => {
                      if (window.confirm(`Delete ${row.name}?`)) {
                        deleteMutation.mutate(row.id);
                      }
                    }}
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </TD>
              </TR>
            ))
          )}
        </TBody>
      </Table>

      <Dialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        title={`New ${props.resourceLabel}`}
      >
        <div className="space-y-3">
          <Field label="Name">
            <Input value={name} onChange={(e) => setName(e.target.value)} autoFocus />
          </Field>
          <Field label="Description">
            <Textarea rows={3} value={desc} onChange={(e) => setDesc(e.target.value)} />
          </Field>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
          <div className="flex justify-end gap-2 mt-2">
            <Button variant="outline" onClick={() => setDialogOpen(false)} type="button">
              Cancel
            </Button>
            <Button
              onClick={() => createMutation.mutate()}
              disabled={!name.trim() || createMutation.isPending}
              type="button"
            >
              {createMutation.isPending ? 'Saving...' : 'Create'}
            </Button>
          </div>
        </div>
      </Dialog>
    </PageContainer>
  );
}
