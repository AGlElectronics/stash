import { describe, expect, it, vi, beforeEach } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { renderWithQuery } from '@/test/render';
import type * as Router from '@tanstack/react-router';
import { ApiError } from '@/api/client';

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  getSignout: vi.fn(),
  returnSignout: vi.fn(),
  closeSignout: vi.fn(),
  confirmSignout: vi.fn(),
  cancelSignout: vi.fn(),
  listItems: vi.fn(),
}));
vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof Router>()),
  useParams: () => ({ id: 'so-1' }),
  useNavigate: () => mocks.navigate,
}));
vi.mock('@/api/signouts', () => ({
  getSignout: mocks.getSignout,
  returnSignout: mocks.returnSignout,
  closeSignout: mocks.closeSignout,
  confirmSignout: mocks.confirmSignout,
  cancelSignout: mocks.cancelSignout,
}));
vi.mock('@/api/items', () => ({ listItems: mocks.listItems }));
vi.mock('@/hooks/useReferenceData', () => ({
  useDepartments: () => ({ data: { items: [{ id: 'dept-1', name: 'Tech' }] } }),
}));

import { SignoutDetailPage } from './SignoutDetailPage';

const line = {
  id: 'line-1', signout_id: 'so-1', item_id: 'item-1',
  quantity_signed_out: 3, quantity_returned: 0, quantity_consumed: 0, quantity_lost: 0, quantity_damaged: 0,
};
function signout(status: string, settled = false) {
  return {
    id: 'so-1', signout_number: 'SO-2026-00042', status, to_department_id: 'dept-1',
    received_by_name: 'Alex', purpose: 'Stage B',
    items: [settled ? { ...line, quantity_returned: 2, quantity_damaged: 1 } : line],
  };
}

describe('SignoutDetailPage', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    mocks.listItems.mockResolvedValue({ items: [{ id: 'item-1', name: 'XLR cable 10 m' }], total: 1 });
  });

  it('returns units with a damaged one', async () => {
    mocks.getSignout.mockResolvedValueOnce(signout('signed_out')).mockResolvedValue(signout('returned', true));
    mocks.returnSignout.mockResolvedValue(signout('returned', true));
    renderWithQuery(<SignoutDetailPage />);

    expect(await screen.findByText('XLR cable 10 m')).toBeInTheDocument();
    const [ret, , , dmg] = screen.getAllByRole('spinbutton');
    fireEvent.change(ret, { target: { value: '2' } });
    fireEvent.change(dmg, { target: { value: '1' } });
    fireEvent.click(screen.getByRole('button', { name: /submit return/i }));

    await waitFor(() =>
      expect(mocks.returnSignout).toHaveBeenCalledWith('so-1', [
        { signout_item_id: 'line-1', returned: 2, consumed: 0, lost: 0, damaged: 1 },
      ])
    );
    expect(await screen.findByRole('button', { name: /close/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /submit return/i })).not.toBeInTheDocument();
  });

  it('asks for quantities before submitting an empty return', async () => {
    mocks.getSignout.mockResolvedValue(signout('signed_out'));
    renderWithQuery(<SignoutDetailPage />);
    fireEvent.click(await screen.findByRole('button', { name: /submit return/i }));
    expect(await screen.findByText('Enter quantities to return on at least one line.')).toBeInTheDocument();
    expect(mocks.returnSignout).not.toHaveBeenCalled();
  });

  it('shows the server error when a return is refused', async () => {
    mocks.getSignout.mockResolvedValue(signout('signed_out'));
    mocks.returnSignout.mockRejectedValue(
      new ApiError(422, { error: { code: 'validation_failed', message: 'settled quantity would exceed signed-out quantity' } })
    );
    renderWithQuery(<SignoutDetailPage />);
    await screen.findByText('XLR cable 10 m');
    fireEvent.change(screen.getAllByRole('spinbutton')[0], { target: { value: '3' } });
    fireEvent.click(screen.getByRole('button', { name: /submit return/i }));
    expect(await screen.findByText('settled quantity would exceed signed-out quantity')).toBeInTheDocument();
  });

  it('closes a returned sign-out and goes back to the list', async () => {
    mocks.getSignout.mockResolvedValue(signout('returned', true));
    mocks.closeSignout.mockResolvedValue(signout('closed', true));
    renderWithQuery(<SignoutDetailPage />);
    fireEvent.click(await screen.findByRole('button', { name: /close/i }));
    await waitFor(() => expect(mocks.navigate).toHaveBeenCalledWith({ to: '/signouts' }));
    expect(mocks.closeSignout).toHaveBeenCalledWith('so-1');
  });
});
