import { apiRequest } from './client';
import type { Location } from '@/types';

export const listLocations = () =>
  apiRequest<{ items: Location[] }>('/api/locations');

export const getLocation = (id: string) =>
  apiRequest<Location>(`/api/locations/${id}`);

export const createLocation = (body: {
  name: string;
  description?: string;
  parent_id?: string;
}) => apiRequest<Location>('/api/locations', { method: 'POST', body });

export const updateLocation = (
  id: string,
  body: { name?: string; description?: string; parent_id?: string; clear_parent?: boolean }
) => apiRequest<Location>(`/api/locations/${id}`, { method: 'PATCH', body });

export const deleteLocation = (id: string) =>
  apiRequest<void>(`/api/locations/${id}`, { method: 'DELETE' });
