import { useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Download, FileText, Paperclip, Trash2 } from 'lucide-react';
import { ApiError, fetchAuthed } from '@/api/client';
import { deleteFile, listFiles, uploadFile, type FileType, type StoredFile } from '@/api/files';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Select } from '@/components/ui/Input';
import { PERMS, useMe } from '@/hooks/useMe';
import { downloadBlob } from '@/lib/csvExport';
import { formatDateTime } from '@/lib/utils';

const ATTACHMENT_TYPES: { value: Exclude<FileType, 'image'>; label: string }[] = [
  { value: 'receipt', label: 'Receipt' },
  { value: 'invoice', label: 'Invoice' },
  { value: 'manual', label: 'Manual' },
  { value: 'warranty', label: 'Warranty' },
  { value: 'attachment', label: 'Other' },
];

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

/** Receipts, invoices, manuals, warranties and other documents of an item. */
export function ItemAttachmentsPanel({ itemId }: { itemId: string }) {
  const qc = useQueryClient();
  const { can, canAny } = useMe();
  const canWrite = can(PERMS.fileUpload) && canAny(PERMS.itemCreate, PERMS.itemEdit);
  const [fileType, setFileType] = useState<Exclude<FileType, 'image'>>('receipt');
  const inputRef = useRef<HTMLInputElement>(null);
  const queryKey = ['item-attachments', itemId];

  const files = useQuery({
    queryKey,
    queryFn: () => listFiles({ entity_type: 'item', entity_id: itemId }),
    select: (d) => d.items.filter((f) => f.file_type !== 'image'),
  });

  const upload = useMutation({
    mutationFn: (file: File) => uploadFile(file, { entity_type: 'item', entity_id: itemId, file_type: fileType }),
    onSuccess: () => qc.invalidateQueries({ queryKey }),
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteFile(id),
    onSuccess: () => qc.invalidateQueries({ queryKey }),
  });

  async function download(f: StoredFile) {
    const res = await fetchAuthed(`/api/files/${f.id}/raw`);
    if (res.ok) downloadBlob(f.original_filename || 'file', await res.blob());
  }

  const error = upload.error ?? remove.error;

  return (
    <div className="space-y-3">
      {files.data?.length ? (
        <ul className="divide-y divide-border rounded-md border border-border">
          {files.data.map((f) => (
            <li key={f.id} className="flex items-center gap-3 px-3 py-2 text-sm">
              <FileText className="w-4 h-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1 truncate" title={f.original_filename}>
                {f.original_filename}
              </span>
              <Badge variant="secondary">
                {ATTACHMENT_TYPES.find((t) => t.value === f.file_type)?.label ?? f.file_type}
              </Badge>
              <span className="hidden sm:inline text-xs text-muted-foreground whitespace-nowrap">
                {formatSize(f.size_bytes)} · {formatDateTime(f.created_at)}
              </span>
              <Button variant="ghost" size="sm" onClick={() => void download(f)} aria-label={`Download ${f.original_filename}`}>
                <Download className="w-4 h-4" />
              </Button>
              {canWrite ? (
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-destructive"
                  disabled={remove.isPending}
                  onClick={() => {
                    if (window.confirm(`Delete ${f.original_filename}?`)) remove.mutate(f.id);
                  }}
                  aria-label={`Delete ${f.original_filename}`}
                >
                  <Trash2 className="w-4 h-4" />
                </Button>
              ) : null}
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-sm text-muted-foreground">{files.isLoading ? 'Loading…' : 'No documents yet.'}</p>
      )}

      {canWrite ? (
        <div className="flex flex-wrap items-center gap-2">
          <Select
            aria-label="Document type"
            value={fileType}
            onChange={(e) => setFileType(e.target.value as Exclude<FileType, 'image'>)}
            className="w-auto"
          >
            {ATTACHMENT_TYPES.map((t) => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </Select>
          <input
            ref={inputRef}
            type="file"
            className="hidden"
            accept=".pdf,.png,.jpg,.jpeg,.webp,.txt,.doc,.docx,.xls,.xlsx,.odt,.ods"
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (file) upload.mutate(file);
              e.target.value = '';
            }}
          />
          <Button variant="outline" onClick={() => inputRef.current?.click()} disabled={upload.isPending}>
            <Paperclip className="w-4 h-4" />
            {upload.isPending ? 'Uploading…' : 'Attach document'}
          </Button>
          <span className="text-xs text-muted-foreground">Up to 50 MB.</span>
        </div>
      ) : null}
      {error ? (
        <p role="alert" className="text-sm text-destructive">
          {error instanceof ApiError ? error.message : 'Something went wrong.'}
        </p>
      ) : null}
    </div>
  );
}
