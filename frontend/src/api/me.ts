import { apiRequest } from './client';
import type { Me } from '@/types';

export const getMe = () => apiRequest<Me>('/api/me');
