import { useEffect, useMemo, useRef, useState } from 'react';
import { ImagePlus, Trash2, Upload } from 'lucide-react';

import { Button } from '@/components/ui/Button';

const MAX_IMAGE_BYTES = 25 * 1024 * 1024;
const ACCEPT = 'image/png,image/jpeg,image/webp,image/gif,image/avif,image/svg+xml';

type Props = {
  files: File[];
  onChange: (next: File[]) => void;
};

/**
 * Queue image files before an item exists. Parent uploads after POST /items
 * succeeds (first file can be marked primary).
 */
export function PendingItemPhotos({ files, onChange }: Props) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [dragOver, setDragOver] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const previews = useMemo(
    () => files.map((f) => ({ file: f, url: URL.createObjectURL(f) })),
    [files],
  );

  useEffect(() => {
    return () => {
      previews.forEach((p) => URL.revokeObjectURL(p.url));
    };
  }, [previews]);

  function pushFiles(list: FileList | File[]) {
    const next: File[] = [...files];
    for (const file of Array.from(list)) {
      if (!file.type.startsWith('image/')) {
        setError(`${file.name} is not an image.`);
        continue;
      }
      if (file.size > MAX_IMAGE_BYTES) {
        setError(`${file.name} is larger than 25 MB.`);
        continue;
      }
      setError(null);
      next.push(file);
    }
    onChange(next);
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <div>
          <p className="text-sm font-medium text-foreground">Photos (optional)</p>
          <p className="text-xs text-muted-foreground mt-0.5">
            Add now or later on the item page. The first photo becomes the cover.
          </p>
        </div>
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => inputRef.current?.click()}
        >
          <Upload className="w-4 h-4" />
          Add files
        </Button>
        <input
          ref={inputRef}
          type="file"
          accept={ACCEPT}
          multiple
          className="hidden"
          onChange={(e) => {
            if (e.target.files?.length) pushFiles(e.target.files);
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
          if (e.dataTransfer.files?.length) pushFiles(e.dataTransfer.files);
        }}
        className={`rounded-md border border-dashed text-sm text-muted-foreground p-4 text-center transition-colors ${
          dragOver ? 'border-primary bg-primary/5 text-foreground' : 'border-border'
        }`}
      >
        <ImagePlus className="w-6 h-6 mx-auto mb-2 opacity-70" />
        Drop images here or use <span className="font-medium">Add files</span>.
      </div>

      {error ? <p className="text-xs text-destructive">{error}</p> : null}

      {previews.length > 0 ? (
        <ul className="flex flex-wrap gap-2">
          {previews.map((p, idx) => (
            <li
              key={`pending-photo-${idx}`}
              className="relative group w-20 h-20 rounded-md border border-border overflow-hidden bg-muted/30"
            >
              <img src={p.url} alt="" className="w-full h-full object-cover" />
              {idx === 0 ? (
                <span className="absolute top-1 left-1 text-[9px] font-medium bg-primary text-primary-foreground px-1 rounded">
                  Cover
                </span>
              ) : null}
              <button
                type="button"
                onClick={() => onChange(files.filter((_, i) => i !== idx))}
                className="absolute inset-0 flex items-center justify-center bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity"
                aria-label={`Remove ${p.file.name}`}
              >
                <Trash2 className="w-5 h-5 text-white" />
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
