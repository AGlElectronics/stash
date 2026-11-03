import type { Item } from '@/types';

/** Optimistic on-hand count before sign-outs are loaded (owned − damaged − lost). */
export function itemOnHand(item: Pick<Item, 'quantity_owned' | 'quantity_damaged' | 'quantity_lost'>): number {
  return Math.max(0, item.quantity_owned - item.quantity_damaged - item.quantity_lost);
}

/** Prefer server `quantity_available`; fall back to on-hand while loading. */
export function itemAvailable(
  item: Pick<Item, 'quantity_owned' | 'quantity_damaged' | 'quantity_lost'>,
  live?: number,
): number {
  return live ?? itemOnHand(item);
}
