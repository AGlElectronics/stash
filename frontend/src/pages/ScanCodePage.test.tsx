import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import type * as Router from '@tanstack/react-router';
import { ApiError } from '@/api/client';

const mocks = vi.hoisted(() => ({
  search: { code: '' } as { code?: string },
  navigate: vi.fn(),
  lookupItemByBarcode: vi.fn(),
  lookupLostFoundByReportNumber: vi.fn(),
}));
vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof Router>()),
  useSearch: () => mocks.search,
  useNavigate: () => mocks.navigate,
}));
vi.mock('@/components/ui/Button', () => ({
  LinkButton: ({ children }: { children: ReactNode }) => <a href="/dashboard">{children}</a>,
}));
vi.mock('@/api/items', () => ({ lookupItemByBarcode: mocks.lookupItemByBarcode }));
vi.mock('@/api/lostfound', () => ({ lookupLostFoundByReportNumber: mocks.lookupLostFoundByReportNumber }));

import { ScanCodePage } from './ScanCodePage';

const notFound = new ApiError(404, { error: { code: 'not_found', message: 'not found' } });

describe('ScanCodePage', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('opens the item a QR label points to', async () => {
    mocks.search = { code: 'ST000042' };
    mocks.lookupItemByBarcode.mockResolvedValue({ id: 'item-1' });
    render(<ScanCodePage />);
    await waitFor(() =>
      expect(mocks.navigate).toHaveBeenCalledWith({ to: '/items/$id', params: { id: 'item-1' }, replace: true })
    );
    expect(mocks.lookupItemByBarcode).toHaveBeenCalledWith('ST000042');
  });

  it('opens a Lost & Found report by its number', async () => {
    mocks.search = { code: 'LF-0042' };
    mocks.lookupLostFoundByReportNumber.mockResolvedValue({ id: 'lf-1' });
    render(<ScanCodePage />);
    await waitFor(() =>
      expect(mocks.navigate).toHaveBeenCalledWith({ to: '/lost-and-found/$id', params: { id: 'lf-1' }, replace: true })
    );
    expect(mocks.lookupItemByBarcode).not.toHaveBeenCalled();
  });

  it('says when nothing has the code', async () => {
    mocks.search = { code: 'XX9' };
    mocks.lookupItemByBarcode.mockRejectedValue(notFound);
    render(<ScanCodePage />);
    expect(await screen.findByText('Nothing in Stash has the code XX9.')).toBeInTheDocument();
    expect(mocks.navigate).not.toHaveBeenCalled();
  });

  it('asks for a code when the link has none', async () => {
    mocks.search = {};
    render(<ScanCodePage />);
    expect(await screen.findByText(/This link has no code/)).toBeInTheDocument();
  });
});
