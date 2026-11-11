import { ApiError, apiRequest, apiUpload, fetchAuthed } from './client';

export type ImportRow = {
  row: number;
  action: 'create' | 'update' | 'error';
  item_id?: string;
  name?: string;
  errors?: string[];
};

export type ImportResult = {
  dry_run: boolean;
  applied: boolean;
  created: number;
  updated: number;
  failed: number;
  rows: ImportRow[];
};

/**
 * POST /api/items/import. A failed real import answers 422 with the report
 * in error.details; this returns that report instead of throwing.
 */
export async function importItemsCsv(file: File, dryRun: boolean): Promise<ImportResult> {
  const fd = new FormData();
  fd.append('file', file, file.name);
  try {
    return await apiUpload<ImportResult>(`/api/items/import${dryRun ? '?dry_run=true' : ''}`, fd);
  } catch (err) {
    const details = err instanceof ApiError ? err.details : undefined;
    if (details && typeof details === 'object' && 'rows' in details) return details as ImportResult;
    throw err;
  }
}

export type PlannedReport = { key: string; title: string; description: string };

export const listPlannedReports = () =>
  apiRequest<{ items: PlannedReport[] }>('/api/export/reports');

/** Download an export (needs export.create); returns false when refused. */
export async function downloadExport(path: string, filename: string): Promise<boolean> {
  const res = await fetchAuthed(path);
  if (!res.ok) return false;
  const url = URL.createObjectURL(await res.blob());
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
  return true;
}
