import { useQuery } from '@tanstack/react-query';
import { getMe } from '@/api/me';

/**
 * Current user identity + effective permissions (computed server-side from
 * the OIDC role -> permission config). Refetched on focus so group changes
 * show up quickly; permission checks in the UI are UX-only — the backend
 * enforces everything.
 */
export function useMe() {
  const query = useQuery({
    queryKey: ['me'],
    queryFn: getMe,
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  });

  const permissions = query.data?.permissions ?? [];

  /** True when the user has the permission. While loading, returns false. */
  const can = (permission: string) => permissions.includes(permission);

  /** True when the user has at least one of the given permissions. */
  const canAny = (...perms: string[]) => perms.some((p) => permissions.includes(p));

  return { ...query, me: query.data, can, canAny };
}

/** Permission names used for UI gating. Mirrors the backend catalog. */
export const PERMS = {
  itemView: 'item.view',
  itemCreate: 'item.create',
  itemEdit: 'item.edit',
  signoutView: 'signout.view',
  signoutCreate: 'signout.create',
  departmentManage: 'department.manage',
  locationManage: 'location.manage',
  categoryManage: 'category.manage',
  auditView: 'audit.view',
  exportCreate: 'export.create',
  accessManage: 'access.manage',
  // Lost & Found — backend implements these (see docs/backend-plan-frontend-redesign.md)
  lostfoundView: 'lostfound.view',
  lostfoundCreate: 'lostfound.create',
  lostfoundEdit: 'lostfound.edit',
  lostfoundResolve: 'lostfound.resolve',
  lostfoundReopen: 'lostfound.reopen',
  lostfoundDelete: 'lostfound.delete',
} as const;
