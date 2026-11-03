import type { Location } from '@/types';

/** Full path like `Parent / Child / Shelf` for each location id. */
export function buildLocationPathMap(locations: Location[]): Map<string, string> {
  const byId = new Map(locations.map((l) => [l.id, l]));
  const memo = new Map<string, string>();

  function pathFor(id: string): string {
    if (memo.has(id)) return memo.get(id)!;
    const l = byId.get(id);
    if (!l) {
      memo.set(id, '');
      return '';
    }
    if (!l.parent_id) {
      memo.set(id, l.name);
      return l.name;
    }
    const p = pathFor(l.parent_id);
    const s = p ? `${p} / ${l.name}` : l.name;
    memo.set(id, s);
    return s;
  }

  for (const l of locations) {
    pathFor(l.id);
  }
  return memo;
}
