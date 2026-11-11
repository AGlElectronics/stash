import { describe, expect, it, vi, beforeEach } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { renderWithQuery, meWith } from '@/test/render';
import type { LostFoundReport } from '@/api/lostfound';

const api = vi.hoisted(() => ({
  getTelegramStatus: vi.fn(),
  notifyLostFoundGuest: vi.fn(),
  getMe: vi.fn(),
}));
vi.mock('@/api/lostfound', () => ({
  getTelegramStatus: api.getTelegramStatus,
  notifyLostFoundGuest: api.notifyLostFoundGuest,
}));
vi.mock('@/api/me', () => ({ getMe: api.getMe }));

import { LostFoundTelegramPanel } from './LostFoundTelegramPanel';

const report = (linked: boolean): LostFoundReport => ({
  id: 'r1',
  report_number: 'LF-0042',
  type: 'found',
  status: 'open',
  item_name: 'Blue backpack',
  description: '',
  location: '',
  date_reported: '2026-08-14T10:00:00Z',
  telegram_linked: linked,
  created_at: '',
  updated_at: '',
});

describe('LostFoundTelegramPanel', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    api.getTelegramStatus.mockResolvedValue({ enabled: true, configured: true, bot_username: 'stash_lf_bot' });
  });

  it('tells the desk how the guest links the report', async () => {
    api.getMe.mockResolvedValue(meWith('lostfound.view'));
    renderWithQuery(<LostFoundTelegramPanel report={report(false)} />);
    expect(await screen.findByText('/start LF0042')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '@stash_lf_bot' })).toHaveAttribute('href', 'https://t.me/stash_lf_bot?start=LF0042');
    expect(screen.queryByRole('button', { name: /notify guest/i })).not.toBeInTheDocument();
  });

  it('lets people with lostfound.notify message a linked guest', async () => {
    api.getMe.mockResolvedValue(meWith('lostfound.view', 'lostfound.notify'));
    api.notifyLostFoundGuest.mockResolvedValue({ sent: true });
    renderWithQuery(<LostFoundTelegramPanel report={report(true)} />);
    fireEvent.click(await screen.findByRole('button', { name: /notify guest/i }));
    fireEvent.change(screen.getByLabelText('Message'), { target: { value: 'Come to the desk' } });
    fireEvent.click(screen.getByRole('button', { name: 'Send' }));
    await waitFor(() => expect(api.notifyLostFoundGuest).toHaveBeenCalledWith('r1', 'Come to the desk'));
    expect(await screen.findByText(/Message sent at/)).toBeInTheDocument();
  });

  it('hides the button without the permission and everything when the bot is off', async () => {
    api.getMe.mockResolvedValue(meWith('lostfound.view'));
    const { unmount } = renderWithQuery(<LostFoundTelegramPanel report={report(true)} />);
    expect(await screen.findByText('Guest linked')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /notify guest/i })).not.toBeInTheDocument();
    unmount();

    api.getTelegramStatus.mockResolvedValue({ enabled: false, configured: true });
    const { container } = renderWithQuery(<LostFoundTelegramPanel report={report(true)} />);
    await waitFor(() => expect(api.getTelegramStatus).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });
});
