import { apiRequest } from './client';
import type { Paginated, Signout, SignoutWithItems } from '@/types';

export async function listSignouts(query: {
  q?: string;
  status?: string;
  to_department_id?: string;
  from_department_id?: string;
  limit?: number;
  offset?: number;
} = {}) {
  return apiRequest<Paginated<Signout>>(
    '/api/signouts',
    { query: query as Record<string, string | number | boolean | undefined | null> },
  );
}

export async function getSignout(id: string) {
  return apiRequest<SignoutWithItems>(`/api/signouts/${id}`);
}

export type CreateSignoutBody = {
  to_department_id: string;
  from_department_id?: string;
  from_location_id?: string;
  purpose?: string;
  expected_return_at?: string;
  received_by_user_id?: string;
  received_by_name?: string;
  notes?: string;
  confirm?: boolean;
  items: Array<{
    item_id: string;
    quantity: number;
    condition_on_signout?: string;
    notes?: string;
    scanned_codes?: string[];
  }>;
};

export async function createSignout(body: CreateSignoutBody) {
  return apiRequest<SignoutWithItems>('/api/signouts', { method: 'POST', body });
}

export async function confirmSignout(id: string) {
  return apiRequest<SignoutWithItems>(`/api/signouts/${id}/confirm`, { method: 'POST' });
}

export type ReturnLine = {
  signout_item_id: string;
  returned?: number;
  consumed?: number;
  lost?: number;
  damaged?: number;
  condition_on_return?: string;
};

export async function returnSignout(id: string, lines: ReturnLine[]) {
  return apiRequest<SignoutWithItems>(`/api/signouts/${id}/return`, {
    method: 'POST',
    body: { lines },
  });
}

export async function cancelSignout(id: string, note?: string) {
  return apiRequest<SignoutWithItems>(`/api/signouts/${id}/cancel`, {
    method: 'POST',
    body: { note: note ?? '' },
  });
}

export async function closeSignout(id: string) {
  return apiRequest<SignoutWithItems>(`/api/signouts/${id}/close`, { method: 'POST' });
}
