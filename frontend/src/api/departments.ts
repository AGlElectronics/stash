import { apiRequest } from './client';
import type { Department } from '@/types';

export const listDepartments = () =>
  apiRequest<{ items: Department[] }>('/api/departments');

export const getDepartment = (id: string) =>
  apiRequest<Department>(`/api/departments/${id}`);

export const createDepartment = (body: { name: string; description?: string }) =>
  apiRequest<Department>('/api/departments', { method: 'POST', body });

export const updateDepartment = (
  id: string,
  body: { name?: string; description?: string }
) => apiRequest<Department>(`/api/departments/${id}`, { method: 'PATCH', body });

export const deleteDepartment = (id: string) =>
  apiRequest<void>(`/api/departments/${id}`, { method: 'DELETE' });
