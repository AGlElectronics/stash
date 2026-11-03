import { ApiError } from '@/api/client';
import { createCategory, listCategories } from '@/api/categories';
import { createDepartment, listDepartments } from '@/api/departments';
import { createLocation, listLocations } from '@/api/locations';
import type { ItemFormValues } from '@/forms/ItemForm';
import { buildLocationPathMap } from '@/lib/locationPath';

type Named = { id: string; name: string };

export function normalizeCreatablePick(pick: { id?: string; name: string }): { id?: string; name: string } {
  const name = pick.name.trim();
  if (!name && pick.id) return { id: pick.id, name: '' };
  if (!name) return { name: '' };
  if (pick.id) return { id: pick.id, name };
  return { name };
}

/** Commit in-flight creatable text before submit (Save can fire before blur). */
export function commitCreatablePick(
  pick: { id?: string; name: string },
  options: Named[]
): { id?: string; name: string } {
  const name = pick.name.trim();
  if (!name) return pick.id ? { id: pick.id, name: '' } : { name: '' };
  if (pick.id) {
    const byId = options.find((o) => o.id === pick.id);
    if (byId && byId.name.toLowerCase() === name.toLowerCase()) {
      return { id: pick.id, name: byId.name };
    }
  }
  const exact = options.find((o) => o.name.toLowerCase() === name.toLowerCase());
  if (exact) return { id: exact.id, name: exact.name };
  return { name };
}

/**
 * Maps a free-text / optional-id pick to an existing id or creates a new record.
 * Handles race duplicates (409) by refetching the list and matching by name.
 *
 * Pass a shared `cache` (lowercase name → id) when resolving multiple picks in
 * one submit — e.g. managed_by + needed_by departments — so parallel creates do
 * not fight each other.
 */
export async function resolveCreatableRef(
  pick: { id?: string; name: string },
  fetchList: () => Promise<Named[]>,
  create: (name: string) => Promise<Named>,
  cache?: Map<string, string>
): Promise<string | undefined> {
  const term = pick.name.trim();
  if (!term) {
    // Label may not be loaded yet (edit form save before reference data fills in).
    if (pick.id) return pick.id;
    return undefined;
  }

  const cacheKey = term.toLowerCase();
  const cached = cache?.get(cacheKey);
  if (cached) return cached;

  const matchIn = (list: Named[]): string | undefined => {
    // Only trust pick.id when the stored name still matches that record.
    if (pick.id) {
      const byId = list.find((x) => x.id === pick.id);
      if (byId && byId.name.toLowerCase() === cacheKey) return pick.id;
    }
    return list.find((x) => x.name.toLowerCase() === cacheKey)?.id;
  };

  let list = await fetchList();
  const first = matchIn(list);
  if (first) {
    cache?.set(cacheKey, first);
    return first;
  }

  try {
    const created = await create(term);
    cache?.set(cacheKey, created.id);
    return created.id;
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) {
      // Another resolver may have created this name milliseconds ago — refetch.
      for (let attempt = 0; attempt < 3; attempt++) {
        if (attempt > 0) {
          await new Promise((r) => setTimeout(r, 50 * attempt));
        }
        list = await fetchList();
        const again = matchIn(list);
        if (again) {
          cache?.set(cacheKey, again);
          return again;
        }
      }
    }
    throw e;
  }
}

function locationOptionsWithPaths(): Promise<Named[]> {
  return listLocations().then((r) => {
    const paths = buildLocationPathMap(r.items);
    return r.items.map((l) => ({ id: l.id, name: paths.get(l.id) ?? l.name }));
  });
}

/** Read live combobox text (Save can beat React Hook Form state). */
export function creatableTextFromDom(inputId: string): string {
  if (typeof document === 'undefined') return '';
  const el = document.getElementById(inputId);
  return el instanceof HTMLInputElement ? el.value.trim() : '';
}

export function creatablePickFromDom(
  inputId: string,
  pick: { id?: string; name: string },
  options: Named[]
): { id?: string; name: string } {
  const domText = creatableTextFromDom(inputId);
  const name = domText || pick.name.trim();
  if (!name) return pick.id ? { id: pick.id, name: '' } : { name: '' };
  return commitCreatablePick({ ...pick, name }, options);
}

/** Map creatable form picks to item API ref fields (ids + names for server-side resolve). */
export function itemFormRefFields(
  values: Pick<ItemFormValues, 'category' | 'managed_by_department' | 'needed_by_department' | 'location'>
) {
  const categoryName = values.category.name.trim();
  const managedName = values.managed_by_department.name.trim();
  const neededName = values.needed_by_department.name.trim();
  const locationName = values.location.name.trim();

  return {
    category_id: values.category.id || undefined,
    category_name: categoryName || undefined,
    managed_by_department_id: values.managed_by_department.id || undefined,
    managed_by_department_name: managedName || undefined,
    needed_by_department_id: values.needed_by_department.id || undefined,
    needed_by_department_name: neededName || undefined,
    location_id: values.location.id || undefined,
    location_name: locationName || undefined,
  };
}

/** Resolve category, location, and both department picks for item create/edit. */
export async function resolveItemFormRefs(
  values: Pick<ItemFormValues, 'category' | 'managed_by_department' | 'needed_by_department' | 'location'>
) {
  const deptCache = new Map<string, string>();

  const category = normalizeCreatablePick(values.category);
  const location = normalizeCreatablePick(values.location);
  const managedBy = normalizeCreatablePick(values.managed_by_department);
  const neededBy = normalizeCreatablePick(values.needed_by_department);

  const [category_id, location_id] = await Promise.all([
    resolveCreatableRef(category, () => listCategories().then((r) => r.items), (name) =>
      createCategory({ name })
    ),
    // Match location labels the same way the CreatableSelect displays them (full paths).
    resolveCreatableRef(location, locationOptionsWithPaths, (name) => createLocation({ name })),
  ]);

  const managed_by_department_id = await resolveCreatableRef(
    managedBy,
    () => listDepartments().then((r) => r.items),
    (name) => createDepartment({ name }),
    deptCache
  );
  const needed_by_department_id = await resolveCreatableRef(
    neededBy,
    () => listDepartments().then((r) => r.items),
    (name) => createDepartment({ name }),
    deptCache
  );

  return { category_id, managed_by_department_id, needed_by_department_id, location_id };
}
