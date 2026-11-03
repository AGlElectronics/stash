import { useQuery } from '@tanstack/react-query';
import { getSettings, type SystemSettings } from '@/api/settings';

export function useSettings() {
  return useQuery<SystemSettings>({
    queryKey: ['system-settings'],
    queryFn: getSettings,
    staleTime: 30_000,
  });
}
