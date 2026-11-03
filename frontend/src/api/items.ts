import { apiRequest } from './client';
import type {
  EventList,
  Item,
  ItemAvailability,
  Paginated,
} from '@/types';

export type ListItemsParams = {
  q?: string;
  category_id?: string;
  managed_dept_id?: string;
  needed_dept_id?: string;
  parent_container_id?: string;
  parent_group_id?: string;
  location_id?: string;
  /** When true with location_id, include items at child locations (shelves under a room). */
  include_sublocations?: boolean;
  item_type?: string;
  status?: string;
  include_retired?: boolean;
  limit?: number;
  offset?: number;
};

export const listItems = (params: ListItemsParams = {}) =>
  apiRequest<Paginated<Item>>('/api/items', {
    query: params as Record<string, string | number | boolean | undefined | null>,
  });

export const getItem = (id: string) => apiRequest<Item>(`/api/items/${id}`);

export const lookupItemByBarcode = (code: string) =>
  apiRequest<Item>(`/api/items/by-barcode/${encodeURIComponent(code)}`);

export const peekNextBarcode = () =>
  apiRequest<{ next: string; format: string }>(`/api/items/barcodes/peek-next`);

export const assignBarcode = (id: string) =>
  apiRequest<{ item: Item; code: string }>(`/api/items/${id}/assign-barcode`, { method: 'POST' });

export const setItemBarcode = (id: string, code: string) =>
  apiRequest<Item>(`/api/items/${id}/set-barcode`, { method: 'POST', body: { code } });

export const getItemAvailability = (id: string) =>
  apiRequest<ItemAvailability>(`/api/items/${id}/availability`);

export const getItemHistory = (
  id: string,
  params: { event_type?: string; signout_id?: string; limit?: number; offset?: number } = {}
) =>
  apiRequest<EventList>(`/api/items/${id}/history`, {
    query: params as Record<string, string | number | boolean | undefined | null>,
  });

export type CreateItemBody = {
  name: string;
  description?: string;
  category_id?: string;
  category_name?: string;
  item_type: 'asset' | 'consumable' | 'container' | 'grouped';
  size?: string;
  quantity_owned: number;
  unit?: string;
  condition?: string;
  status?: string;
  location_id?: string;
  location_name?: string;
  parent_container_id?: string;
  parent_group_id?: string;
  managed_by_department_id?: string;
  managed_by_department_name?: string;
  needed_by_department_id?: string;
  needed_by_department_name?: string;
  serial_number?: string;
  asset_tag?: string;
  qr_code?: string;
  owner?: string;
  purchase_price?: string;
  purchase_currency?: string;
  purchase_date?: string;
  expiry_date?: string;
  supplier?: string;
  purchase_url?: string;
  replacement_cost?: string;
  minimum_quantity?: number;
  notes?: string;
};

export const createItem = (body: CreateItemBody) =>
  apiRequest<Item>('/api/items', { method: 'POST', body });

export type UpdateItemBody = Partial<CreateItemBody> & {
  clear_category?: boolean;
  clear_location?: boolean;
  clear_container?: boolean;
  clear_group?: boolean;
  clear_managed_by?: boolean;
  clear_needed_by?: boolean;
  clear_purchase_price?: boolean;
  clear_purchase_date?: boolean;
  clear_expiry_date?: boolean;
  clear_replacement_cost?: boolean;
};

export const updateItem = (id: string, body: UpdateItemBody) =>
  apiRequest<Item>(`/api/items/${id}`, { method: 'PATCH', body });

// Lifecycle endpoints
export const moveItem = (id: string, body: {
  to_location_id?: string;
  clear_location?: boolean;
  to_container_id?: string;
  clear_container?: boolean;
  note?: string;
}) => apiRequest<Item>(`/api/items/${id}/move`, { method: 'POST', body });

export const adjustItem = (id: string, body: { delta: number; note?: string }) =>
  apiRequest<Item>(`/api/items/${id}/adjust-quantity`, { method: 'POST', body });

export const damageItem = (id: string, body: { quantity: number; note?: string }) =>
  apiRequest<Item>(`/api/items/${id}/damage`, { method: 'POST', body });

export const repairItem = (
  id: string,
  body: { quantity: number; note?: string; related_event_id?: string }
) => apiRequest<Item>(`/api/items/${id}/repair`, { method: 'POST', body });

export const lostItem = (id: string, body: { quantity: number; note?: string }) =>
  apiRequest<Item>(`/api/items/${id}/lost`, { method: 'POST', body });

export const foundItem = (id: string, body: { quantity: number; note?: string }) =>
  apiRequest<Item>(`/api/items/${id}/found`, { method: 'POST', body });

export const retireItem = (id: string, body: { note?: string } = {}) =>
  apiRequest<Item>(`/api/items/${id}/retire`, { method: 'POST', body });

export const restoreItem = (id: string, body: { note?: string } = {}) =>
  apiRequest<Item>(`/api/items/${id}/restore`, { method: 'POST', body });
