import { apiRequest, apiUpload, fetchAuthed } from './client';

export type FileType = 'image' | 'receipt' | 'invoice' | 'manual' | 'warranty' | 'attachment';

export type StoredFile = {
  id: string;
  entity_type: string;
  entity_id: string;
  file_type: FileType;
  storage_key: string;
  original_filename: string;
  content_type: string;
  size_bytes: number;
  uploaded_by?: string;
  created_at: string;
};

type ListFilesParams = {
  entity_type: string;
  entity_id: string;
  file_type?: FileType;
};

export const listFiles = (params: ListFilesParams) =>
  apiRequest<{ items: StoredFile[] }>('/api/files', {
    query: params as Record<string, string | undefined>,
  });

export const deleteFile = (id: string) =>
  apiRequest<void>(`/api/files/${id}`, { method: 'DELETE' });

export type DownloadInfo = { url: string; expires_at: string };

/**
 * Get a presigned download URL (15 min). Useful for "open in new tab"
 * style flows. For inline rendering (`<img src>`) prefer fileRawURL because
 * presigned URLs are bound to a specific signed host header and do not work
 * cleanly across origins in dev.
 */
export const getFileDownloadURL = (id: string) =>
  apiRequest<DownloadInfo>(`/api/files/${id}/download`);

/**
 * Fetch the raw bytes for a file using the same auth headers as the rest of
 * /api and turn them into a blob URL the browser can drop straight into
 * `<img src>` / `<a href>`. Caller is responsible for revoking the URL when
 * the consumer unmounts.
 */
export async function fetchFileBlobURL(id: string): Promise<string> {
  const res = await fetchAuthed(`/api/files/${id}/raw`);
  if (!res.ok) {
    throw new Error(`failed to fetch file (${res.status})`);
  }
  const blob = await res.blob();
  return URL.createObjectURL(blob);
}

export type UploadOptions = {
  entity_type: string;
  entity_id: string;
  file_type?: FileType;
  set_as_primary?: boolean;
};

/**
 * Upload a single file via the backend (multipart). The backend streams to
 * S3 and registers the metadata in one call so the browser never has to
 * hit the storage host directly (no CORS dance in dev).
 */
export function uploadFile(file: File, opts: UploadOptions): Promise<StoredFile> {
  const fd = new FormData();
  fd.append('file', file, file.name);
  fd.append('entity_type', opts.entity_type);
  fd.append('entity_id', opts.entity_id);
  fd.append('file_type', opts.file_type ?? 'image');
  if (opts.set_as_primary) fd.append('set_as_primary', 'true');
  return apiUpload<StoredFile>('/api/files/upload', fd);
}

// Item-level primary image helpers.

export const setItemPrimaryImage = (itemId: string, fileId: string) =>
  apiRequest<void>(`/api/items/${itemId}/primary-image`, {
    method: 'POST',
    body: { file_id: fileId },
  });

export const clearItemPrimaryImage = (itemId: string) =>
  apiRequest<void>(`/api/items/${itemId}/primary-image`, { method: 'DELETE' });

/** Per-item helper: list only image files. */
export const listItemImages = (itemId: string) =>
  listFiles({ entity_type: 'item', entity_id: itemId, file_type: 'image' });
