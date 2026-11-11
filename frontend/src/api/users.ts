import { apiRequest } from './client';
import type { Paginated } from '@/types';

export type StashUser = {
  id: string;
  display_name: string;
  email: string;
  last_login_at?: string;
  anonymised_at?: string;
  created_at: string;
};

export const listUsers = (params: { limit?: number; offset?: number } = {}) =>
  apiRequest<Paginated<StashUser>>('/api/users', { query: params });

export const anonymiseUser = (id: string) =>
  apiRequest<void>(`/api/users/${id}/anonymise`, { method: 'POST' });
