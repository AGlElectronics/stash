import { useMemo } from 'react';
import { useQueries } from '@tanstack/react-query';
import { getItemAvailability } from '@/api/items';

/** Live available counts (owned − signed-out − damaged − lost) for a set of items. */
export function useItemsAvailability(itemIds: string[]) {
  const queries = useQueries({
    queries: itemIds.map((id) => ({
      queryKey: ['item-availability', id],
      queryFn: () => getItemAvailability(id),
      staleTime: 0,
    })),
  });

  return useMemo(() => {
    const map = new Map<string, number>();
    itemIds.forEach((id, idx) => {
      const avail = queries[idx]?.data?.quantity_available;
      if (avail !== undefined) map.set(id, avail);
    });
    return map;
  }, [itemIds, queries]);
}
