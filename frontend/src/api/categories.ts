import { apiRequest } from './client';
import type { Category } from '@/types';

export const listCategories = () =>
  apiRequest<{ items: Category[] }>('/api/categories');

export const getCategory = (id: string) =>
  apiRequest<Category>(`/api/categories/${id}`);

export const createCategory = (body: { name: string; description?: string }) =>
  apiRequest<Category>('/api/categories', { method: 'POST', body });

export const updateCategory = (
  id: string,
  body: { name?: string; description?: string }
) => apiRequest<Category>(`/api/categories/${id}`, { method: 'PATCH', body });

export const deleteCategory = (id: string) =>
  apiRequest<void>(`/api/categories/${id}`, { method: 'DELETE' });
