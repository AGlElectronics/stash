import { apiRequest } from './client';
import type { Signout } from '@/types';

export type Purchase = {
  id: string;
  item_id: string;
  purchased_on: string;
  quantity: number;
  unit_price?: string;
  currency: string;
  supplier: string;
  reference: string;
  added_to_stock: boolean;
  note: string;
  created_at: string;
};

export type PurchaseInput = {
  purchased_on?: string;
  quantity: number;
  unit_price?: string;
  currency?: string;
  supplier?: string;
  reference?: string;
  note?: string;
  add_to_stock: boolean;
};

export const listPurchases = (itemId: string) =>
  apiRequest<{ items: Purchase[] }>(`/api/items/${itemId}/purchases`);

export const recordPurchase = (itemId: string, body: PurchaseInput) =>
  apiRequest<Purchase>(`/api/items/${itemId}/purchases`, { method: 'POST', body });

export type RequestStatus = 'open' | 'approved' | 'ordered' | 'fulfilled' | 'rejected' | 'cancelled';

export type ItemRequest = {
  id: string;
  title: string;
  item_id?: string;
  item_name?: string;
  quantity: number;
  department_id?: string;
  department_name?: string;
  needed_by?: string;
  reason: string;
  status: RequestStatus;
  requested_by?: string;
  requested_by_name?: string;
  decided_at?: string;
  decision_note: string;
  created_at: string;
};

export type RequestInput = {
  title: string;
  item_id?: string;
  quantity: number;
  department_id?: string;
  needed_by?: string;
  reason?: string;
};

export const listRequests = (status = '') =>
  apiRequest<{ items: ItemRequest[] }>('/api/requests', { query: { status: status || undefined } });

export const createRequest = (body: RequestInput) =>
  apiRequest<ItemRequest>('/api/requests', { method: 'POST', body });

export const decideRequest = (id: string, status: RequestStatus, note = '') =>
  apiRequest<ItemRequest>(`/api/requests/${id}/decide`, { method: 'POST', body: { status, note } });

export type PackingListItem = {
  item_id: string;
  item_name: string;
  asset_tag?: string;
  quantity: number;
  packed: number;
  note: string;
  available: number;
};

export type PackingList = {
  id: string;
  name: string;
  department_id?: string;
  department_name?: string;
  event_date?: string;
  notes: string;
  signout_id?: string;
  item_count: number;
  packed_count: number;
  items?: PackingListItem[];
};

export type PackingListInput = {
  name: string;
  department_id?: string;
  event_date?: string;
  notes?: string;
};

export type PackingLineInput = { item_id: string; quantity: number; packed: number; note: string };

export const listPackingLists = () => apiRequest<{ items: PackingList[] }>('/api/packing-lists');
export const getPackingList = (id: string) => apiRequest<PackingList>(`/api/packing-lists/${id}`);
export const createPackingList = (body: PackingListInput) =>
  apiRequest<PackingList>('/api/packing-lists', { method: 'POST', body });
export const deletePackingList = (id: string) =>
  apiRequest<void>(`/api/packing-lists/${id}`, { method: 'DELETE' });
export const setPackingItems = (id: string, items: PackingLineInput[]) =>
  apiRequest<PackingList>(`/api/packing-lists/${id}/items`, { method: 'PUT', body: { items } });
export const packingListToSignout = (id: string) =>
  apiRequest<Signout>(`/api/packing-lists/${id}/signout`, { method: 'POST', body: {} });
