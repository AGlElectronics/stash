import { apiRequest } from './client';

export type SystemSettings = {
  force_barcode_on_signout: boolean;
  organization_name?: string;
  default_item_owner?: string;
  default_currency?: string;
  barcode_prefix?: string;
  barcode_width?: number;
  preserve_sacred_lostfound?: boolean;
};

export const getSettings = () => apiRequest<SystemSettings>('/api/settings');

export const patchSettings = (patch: Partial<SystemSettings>) =>
  apiRequest<SystemSettings>('/api/settings', { method: 'PATCH', body: patch });
