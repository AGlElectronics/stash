import { useQuery } from '@tanstack/react-query';
import { listCategories } from '@/api/categories';
import { listDepartments } from '@/api/departments';
import { listLocations } from '@/api/locations';

export function useCategories() {
  return useQuery({
    queryKey: ['categories'],
    queryFn: listCategories,
    staleTime: 60_000,
  });
}

export function useDepartments() {
  return useQuery({
    queryKey: ['departments'],
    queryFn: listDepartments,
    staleTime: 60_000,
  });
}

export function useLocations() {
  return useQuery({
    queryKey: ['locations'],
    queryFn: listLocations,
    staleTime: 60_000,
  });
}
