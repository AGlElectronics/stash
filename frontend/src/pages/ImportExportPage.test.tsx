import { describe, expect, it, vi, beforeEach } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { renderWithQuery, meWith } from '@/test/render';

const api = vi.hoisted(() => ({
  importItemsCsv: vi.fn(),
  listPlannedReports: vi.fn(),
  downloadExport: vi.fn(),
  getMe: vi.fn(),
}));
vi.mock('@/api/importExport', () => api);
vi.mock('@/api/me', () => ({ getMe: api.getMe }));

import { ImportExportPage } from './ImportExportPage';

const file = new File(['name\nTent\n'], 'items.csv', { type: 'text/csv' });

describe('ImportExportPage', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    api.listPlannedReports.mockResolvedValue({ items: [{ key: 'low-stock', title: 'Low stock', description: 'd' }] });
  });

  it('only allows importing after a clean check', async () => {
    api.getMe.mockResolvedValue(meWith('item.create', 'export.create'));
    api.importItemsCsv
      .mockResolvedValueOnce({
        dry_run: true, applied: false, created: 1, updated: 0, failed: 1,
        rows: [{ row: 3, action: 'error', name: 'Bad', errors: ['purchase_date must be a date like 2026-08-14'] }],
      })
      .mockResolvedValueOnce({ dry_run: true, applied: false, created: 1, updated: 0, failed: 0, rows: [] })
      .mockResolvedValueOnce({ dry_run: false, applied: true, created: 1, updated: 0, failed: 0, rows: [] });

    renderWithQuery(<ImportExportPage />);
    fireEvent.change(await screen.findByLabelText('CSV file'), { target: { files: [file] } });
    const importBtn = screen.getByRole('button', { name: /^import$/i });
    expect(importBtn).toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: /check file/i }));
    expect(await screen.findByText(/purchase_date must be a date/)).toBeInTheDocument();
    expect(importBtn).toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: /check file/i }));
    await screen.findByText(/Ready: 1 to create/);
    expect(importBtn).toBeEnabled();
    fireEvent.click(importBtn);
    await screen.findByText(/Saved: 1 created/);
    expect(api.importItemsCsv).toHaveBeenLastCalledWith(file, false);
  });

  it('shows exports only with export.create', async () => {
    api.getMe.mockResolvedValue(meWith('item.edit'));
    renderWithQuery(<ImportExportPage />);
    await screen.findByText('Import items from CSV');
    await waitFor(() => expect(screen.queryByText('Exports')).not.toBeInTheDocument());
  });

  it('downloads a planned report', async () => {
    api.getMe.mockResolvedValue(meWith('export.create'));
    api.downloadExport.mockResolvedValue(true);
    renderWithQuery(<ImportExportPage />);
    fireEvent.click(await screen.findByRole('button', { name: /low stock/i }));
    await waitFor(() =>
      expect(api.downloadExport).toHaveBeenCalledWith('/api/export/reports/low-stock.csv', expect.stringMatching(/^low-stock-/))
    );
    expect(screen.queryByText('Import items from CSV')).not.toBeInTheDocument();
  });
});
