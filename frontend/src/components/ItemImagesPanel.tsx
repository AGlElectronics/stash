import { useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Image as ImageIcon, Star, Trash2, Upload, Loader2, AlertTriangle } from 'lucide-react';

import {
  clearItemPrimaryImage,
  deleteFile,
  listItemImages,
  setItemPrimaryImage,
  uploadFile,
  type StoredFile,
} from '@/api/files';
import { Button } from '@/components/ui/Button';
import { ErrorRow, LoadingRow } from '@/components/ui/States';
import { useFileBlobUrl } from '@/hooks/useFileBlobUrl';

type Props = {
  itemId: string;
  primaryImageFileID?: string | null;
};

const MAX_IMAGE_BYTES = 25 * 1024 * 1024;
const ACCEPT = 'image/png,image/jpeg,image/webp,image/gif,image/avif,image/svg+xml';

export function ItemImagesPanel({ itemId, primaryImageFileID }: Props) {
  const qc = useQueryClient();
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [dragOver, setDragOver] = useState(false);

  const list = useQuery({
    queryKey: ['item-images', itemId],
    queryFn: () => listItemImages(itemId),
  });

  const upload = useMutation({
    mutationFn: ({ file, setPrimary }: { file: File; setPrimary: boolean }) =>
      uploadFile(file, {
        entity_type: 'item',
        entity_id: itemId,
        file_type: 'image',
        set_as_primary: setPrimary,
      }),
    onSuccess: () => {
      setError(null);
      qc.invalidateQueries({ queryKey: ['item-images', itemId] });
      qc.invalidateQueries({ queryKey: ['item', itemId] });
    },
    onError: (e: unknown) => {
      setError(e instanceof Error ? e.message : 'Upload failed.');
    },
  });

  const setPrimary = useMutation({
    mutationFn: (fileId: string) => setItemPrimaryImage(itemId, fileId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['item', itemId] });
    },
    onError: (e: unknown) => {
      setError(e instanceof Error ? e.message : 'Could not set primary image.');
    },
  });

  const clearPrimary = useMutation({
    mutationFn: () => clearItemPrimaryImage(itemId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['item', itemId] });
    },
  });

  const remove = useMutation({
    mutationFn: (fileId: string) => deleteFile(fileId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['item-images', itemId] });
      qc.invalidateQueries({ queryKey: ['item', itemId] });
    },
  });

  const acceptFiles = (files: FileList | File[]) => {
    const list = Array.from(files);
    if (list.length === 0) return;
    setError(null);
    let primaryAlreadyAssigned = !!primaryImageFileID;
    for (const file of list) {
      if (!file.type.startsWith('image/')) {
        setError(`${file.name}: not an image.`);
        continue;
      }
      if (file.size > MAX_IMAGE_BYTES) {
        setError(`${file.name}: larger than 25 MB.`);
        continue;
      }
      const setAsPrimary = !primaryAlreadyAssigned;
      primaryAlreadyAssigned = true;
      upload.mutate({ file, setPrimary: setAsPrimary });
    }
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted-foreground">
          PNG / JPEG / WebP / GIF, up to 25 MB. The first image you upload becomes the cover.
        </p>
        <Button
          size="sm"
          variant="outline"
          onClick={() => inputRef.current?.click()}
          disabled={upload.isPending}
        >
          {upload.isPending ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : (
            <Upload className="w-4 h-4" />
          )}
          {upload.isPending ? 'Uploading…' : 'Upload images'}
        </Button>
        <input
          ref={inputRef}
          type="file"
          accept={ACCEPT}
          multiple
          className="hidden"
          onChange={(e) => {
            if (e.target.files) acceptFiles(e.target.files);
            e.target.value = '';
          }}
        />
      </div>

      <div
        onDragOver={(e) => {
          e.preventDefault();
          setDragOver(true);
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragOver(false);
          if (e.dataTransfer.files) acceptFiles(e.dataTransfer.files);
        }}
        className={`rounded-md border border-dashed text-sm text-muted-foreground p-4 text-center transition-colors ${
          dragOver ? 'border-primary bg-primary/5 text-foreground' : 'border-border'
        }`}
      >
        Drop image files here, or click <span className="font-medium">Upload images</span>.
      </div>

      {error ? (
        <p className="text-xs text-destructive flex items-center gap-1">
          <AlertTriangle className="w-3.5 h-3.5" />
          {error}
        </p>
      ) : null}

      {list.isLoading ? (
        <LoadingRow />
      ) : list.isError ? (
        <ErrorRow message="Failed to load images." />
      ) : list.data && list.data.items.length > 0 ? (
        <ImagesGrid
          files={list.data.items}
          primaryImageFileID={primaryImageFileID ?? null}
          onSetPrimary={(id) => setPrimary.mutate(id)}
          onClearPrimary={() => clearPrimary.mutate()}
          onDelete={(file) => {
            const note = file.id === primaryImageFileID ? '\n\nThis is the primary image.' : '';
            if (window.confirm(`Delete ${file.original_filename}?${note}`)) {
              remove.mutate(file.id);
            }
          }}
        />
      ) : (
        <div className="flex flex-col items-center gap-2 py-6 text-muted-foreground">
          <ImageIcon className="w-8 h-8 opacity-60" />
          <p className="text-sm">No images yet.</p>
        </div>
      )}
    </div>
  );
}

type GridProps = {
  files: StoredFile[];
  primaryImageFileID: string | null;
  onSetPrimary: (id: string) => void;
  onClearPrimary: () => void;
  onDelete: (file: StoredFile) => void;
};

function ImagesGrid({ files, primaryImageFileID, onSetPrimary, onClearPrimary, onDelete }: GridProps) {
  // Sort: primary first, then by upload time desc.
  const ordered = useMemo(() => {
    const list = [...files];
    list.sort((a, b) => {
      if (a.id === primaryImageFileID) return -1;
      if (b.id === primaryImageFileID) return 1;
      return b.created_at.localeCompare(a.created_at);
    });
    return list;
  }, [files, primaryImageFileID]);

  return (
    <ul className="grid grid-cols-2 sm:grid-cols-3 gap-3">
      {ordered.map((file) => {
        const isPrimary = file.id === primaryImageFileID;
        return (
          <li
            key={file.id}
            className={`group relative overflow-hidden rounded-md border bg-muted/30 ${
              isPrimary ? 'border-primary ring-1 ring-primary' : 'border-border'
            }`}
          >
            <ImageThumb file={file} />

            {isPrimary ? (
              <span className="absolute top-1.5 left-1.5 inline-flex items-center gap-1 rounded bg-primary px-1.5 py-0.5 text-[10px] font-medium text-primary-foreground">
                <Star className="w-3 h-3 fill-current" />
                Primary
              </span>
            ) : null}

            <div className="absolute inset-x-0 bottom-0 flex items-center justify-between gap-1 p-1.5 bg-gradient-to-t from-black/80 to-transparent text-white opacity-0 group-hover:opacity-100 transition-opacity">
              {isPrimary ? (
                <button
                  type="button"
                  onClick={onClearPrimary}
                  className="text-[11px] underline hover:no-underline"
                  title="Remove as primary"
                >
                  Unset primary
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => onSetPrimary(file.id)}
                  className="inline-flex items-center gap-1 text-[11px] hover:underline"
                  title="Use as primary"
                >
                  <Star className="w-3 h-3" />
                  Make primary
                </button>
              )}
              <button
                type="button"
                onClick={() => onDelete(file)}
                className="inline-flex items-center gap-1 text-[11px] hover:underline"
                title="Delete image"
              >
                <Trash2 className="w-3 h-3" />
                Delete
              </button>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

function ImageThumb({ file }: { file: StoredFile }) {
  const preview = useFileBlobUrl(file.id);

  return (
    <div className="aspect-square w-full flex items-center justify-center bg-muted/30">
      {preview.data ? (
        <img
          src={preview.data}
          alt={file.original_filename}
          className="h-full w-full object-cover"
          loading="lazy"
        />
      ) : preview.isError ? (
        <span className="text-xs text-destructive px-2 text-center">Could not load preview</span>
      ) : (
        <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
      )}
    </div>
  );
}
