import { apiRequest } from './client';
import type { Paginated } from '@/types';

/** Lost & Found API (`/api/lostfound`). */

export type LostFoundType = 'lost' | 'found';

/**
 * open     -> still being looked for / waiting to be claimed
 * returned -> handed back to its owner
 * closed   -> end-of-con outcome: unclaimed, disposed, donated, ...
 */
export type LostFoundStatus = 'open' | 'returned' | 'closed';

export type LostFoundReport = {
  id: string;
  /** Human-friendly unique number, e.g. "LF-0042". */
  report_number: string;
  type: LostFoundType;
  status: LostFoundStatus;
  item_name: string;
  description: string;
  /** Free text ("left near gym") — intentionally not a Stash location id. */
  location: string;
  photo_file_id?: string;
  date_reported: string;
  /** Staff member who entered the report (from the OIDC session). */
  reported_by_user_id?: string;
  reported_by_name?: string;
  /** Guest the report was taken for — front desk logs on behalf of attendees. */
  contact_name?: string;
  contact_info?: string;
  /** Person the item was handed back to (status = returned). */
  returned_to?: string;
  resolved_by_user_id?: string;
  resolved_by_name?: string;
  resolved_at?: string;
  resolution_note?: string;
  /** The guest linked this report on Telegram (the chat id stays on the server). */
  telegram_linked?: boolean;
  created_at: string;
  updated_at: string;
};

export type TelegramStatus = {
  enabled: boolean;
  configured: boolean;
  bot_username?: string;
};

export const getTelegramStatus = () => apiRequest<TelegramStatus>('/api/lostfound/telegram');

/** Message the guest on Telegram; an empty message sends the default text. */
export const notifyLostFoundGuest = (id: string, message: string) =>
  apiRequest<{ sent: boolean }>(`/api/lostfound/${id}/notify`, { method: 'POST', body: { message } });

export type ListLostFoundParams = {
  q?: string;
  type?: LostFoundType;
  status?: LostFoundStatus;
  limit?: number;
  offset?: number;
};

export const listLostFound = (params: ListLostFoundParams = {}) =>
  apiRequest<Paginated<LostFoundReport>>('/api/lostfound', {
    query: params as Record<string, string | number | boolean | undefined | null>,
  });

export const getLostFoundReport = (id: string) =>
  apiRequest<LostFoundReport>(`/api/lostfound/${id}`);

/** Exact lookup by report number (e.g. LF-0003) — used by the scan dialog. */
export const lookupLostFoundByReportNumber = (reportNumber: string) =>
  apiRequest<LostFoundReport>(
    `/api/lostfound/by-number/${encodeURIComponent(reportNumber.trim().toUpperCase())}`
  );

export type CreateLostFoundBody = {
  type: LostFoundType;
  item_name: string;
  description?: string;
  location?: string;
  photo_file_id?: string;
  date_reported?: string;
  contact_name?: string;
  contact_info?: string;
};

export const createLostFoundReport = (body: CreateLostFoundBody) =>
  apiRequest<LostFoundReport>('/api/lostfound', { method: 'POST', body });

export type UpdateLostFoundBody = Partial<CreateLostFoundBody> & {
  status?: LostFoundStatus;
  returned_to?: string;
  resolution_note?: string;
  /** Set to empty string to remove the photo. */
  photo_file_id?: string;
};

export const updateLostFoundReport = (id: string, body: UpdateLostFoundBody) =>
  apiRequest<LostFoundReport>(`/api/lostfound/${id}`, { method: 'PATCH', body });

export const deleteLostFoundReport = (id: string) =>
  apiRequest<void>(`/api/lostfound/${id}`, { method: 'DELETE' });
