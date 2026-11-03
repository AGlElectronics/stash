import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Pencil, Plus, Trash2 } from 'lucide-react';
import { ApiError } from '@/api/client';
import { createLocation, deleteLocation, listLocations, updateLocation } from '@/api/locations';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Input, Select, Textarea } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { EmptyRow, ErrorRow, LoadingRow } from '@/components/ui/States';
import { buildLocationPathMap } from '@/lib/locationPath';
import type { Location } from '@/types';

export function LocationsPage() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ['locations'], queryFn: listLocations });

  const pathById = useMemo(
    () => buildLocationPathMap(q.data?.items ?? []),
    [q.data?.items]
  );

  const [createOpen, setCreateOpen] = useState(false);
  const [editRow, setEditRow] = useState<Location | null>(null);

  const [name, setName] = useState('');
  const [desc, setDesc] = useState('');
  const [parentId, setParentId] = useState('');
  const [formError, setFormError] = useState<string | null>(null);

  const openCreate = () => {
    setName('');
    setDesc('');
    setParentId('');
    setFormError(null);
    setCreateOpen(true);
  };

  const openEdit = (row: Location) => {
    setEditRow(row);
    setName(row.name);
    setDesc(row.description);
    setParentId(row.parent_id ?? '');
    setFormError(null);
  };

  const closeDialogs = () => {
    setCreateOpen(false);
    setEditRow(null);
    setFormError(null);
  };

  const createMut = useMutation({
    mutationFn: () =>
      createLocation({
        name: name.trim(),
        description: desc,
        parent_id: parentId || undefined,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['locations'] });
      closeDialogs();
    },
    onError: (err) => {
      setFormError(err instanceof ApiError ? err.message : 'Failed to create location.');
    },
  });

  const updateMut = useMutation({
    mutationFn: () => {
      if (!editRow) throw new Error('no row');
      return updateLocation(editRow.id, {
        name: name.trim(),
        description: desc,
        ...(parentId ? { parent_id: parentId } : { clear_parent: true }),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['locations'] });
      closeDialogs();
    },
    onError: (err) => {
      setFormError(err instanceof ApiError ? err.message : 'Failed to update location.');
    },
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteLocation(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['locations'] }),
  });

  const parentOptions = useMemo(() => {
    const items = q.data?.items ?? [];
    const editingId = editRow?.id;
    return items.filter((l) => l.id !== editingId);
  }, [q.data?.items, editRow?.id]);

  return (
    <PageContainer>
      <PageHeader
        title="Locations"
        description="Hierarchical places — rooms, shelves, bins. Pick a parent to nest (e.g. shelf under a room)."
        actions={
          <Button onClick={openCreate}>
            <Plus className="w-4 h-4" />
            New location
          </Button>
        }
      />

      <Table>
        <THead>
          <TR>
            <TH>Path</TH>
            <TH>Name</TH>
            <TH>Description</TH>
            <TH className="w-28 text-right">Actions</TH>
          </TR>
        </THead>
        <TBody>
          {q.isLoading ? (
            <TR>
              <TD colSpan={4}>
                <LoadingRow />
              </TD>
            </TR>
          ) : q.isError ? (
            <TR>
              <TD colSpan={4}>
                <ErrorRow message="Failed to load locations." />
              </TD>
            </TR>
          ) : !q.data?.items.length ? (
            <TR>
              <TD colSpan={4}>
                <EmptyRow message="No locations yet." />
              </TD>
            </TR>
          ) : (
            q.data.items.map((row) => (
              <TR key={row.id}>
                <TD className="text-muted-foreground text-sm">{pathById.get(row.id) ?? row.name}</TD>
                <TD className="font-medium">{row.name}</TD>
                <TD className="text-muted-foreground">{row.description || '—'}</TD>
                <TD className="text-right">
                  <Button variant="ghost" size="icon" onClick={() => openEdit(row)} title="Edit">
                    <Pencil className="w-4 h-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => {
                      if (window.confirm(`Delete location “${row.name}”?`)) {
                        deleteMut.mutate(row.id);
                      }
                    }}
                    title="Delete"
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </TD>
              </TR>
            ))
          )}
        </TBody>
      </Table>

      <Dialog open={createOpen} onClose={closeDialogs} title="New location">
        <LocationFormFields
          name={name}
          setName={setName}
          desc={desc}
          setDesc={setDesc}
          parentId={parentId}
          setParentId={setParentId}
          parentOptions={parentOptions}
          pathById={pathById}
          error={formError}
        />
        <div className="flex justify-end gap-2 mt-4">
          <Button variant="outline" type="button" onClick={closeDialogs}>
            Cancel
          </Button>
          <Button
            type="button"
            disabled={!name.trim() || createMut.isPending}
            onClick={() => createMut.mutate()}
          >
            {createMut.isPending ? 'Saving…' : 'Create'}
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={!!editRow}
        onClose={closeDialogs}
        title={editRow ? `Edit — ${editRow.name}` : 'Edit location'}
      >
        <LocationFormFields
          name={name}
          setName={setName}
          desc={desc}
          setDesc={setDesc}
          parentId={parentId}
          setParentId={setParentId}
          parentOptions={parentOptions}
          pathById={pathById}
          error={formError}
        />
        <div className="flex justify-end gap-2 mt-4">
          <Button variant="outline" type="button" onClick={closeDialogs}>
            Cancel
          </Button>
          <Button
            type="button"
            disabled={!name.trim() || updateMut.isPending}
            onClick={() => updateMut.mutate()}
          >
            {updateMut.isPending ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </Dialog>
    </PageContainer>
  );
}

function LocationFormFields(props: {
  name: string;
  setName: (v: string) => void;
  desc: string;
  setDesc: (v: string) => void;
  parentId: string;
  setParentId: (v: string) => void;
  parentOptions: Location[];
  pathById: Map<string, string>;
  error: string | null;
}) {
  return (
    <div className="space-y-3">
      <Field label="Name">
        <Input value={props.name} onChange={(e) => props.setName(e.target.value)} autoFocus />
      </Field>
      <Field label="Parent (optional)" hint="Leave empty for a top-level site or room.">
        <Select value={props.parentId} onChange={(e) => props.setParentId(e.target.value)}>
          <option value="">— None —</option>
          {props.parentOptions.map((l) => (
            <option key={l.id} value={l.id}>
              {props.pathById.get(l.id) ?? l.name}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Description">
        <Textarea rows={3} value={props.desc} onChange={(e) => props.setDesc(e.target.value)} />
      </Field>
      {props.error ? <p className="text-sm text-destructive">{props.error}</p> : null}
    </div>
  );
}
