import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  redirect,
} from '@tanstack/react-router';
import { Layout } from '@/components/Layout';
import { getUserManager, isOidcAuthMode } from '@/auth/oidc';
import { CategoriesPage } from '@/pages/CategoriesPage';
import { ClaimPage } from '@/pages/ClaimPage';
import { DashboardPage } from '@/pages/DashboardPage';
import { DepartmentsPage } from '@/pages/DepartmentsPage';
import { ItemDetailPage } from '@/pages/ItemDetailPage';
import { ItemEditPage } from '@/pages/ItemEditPage';
import { ItemLabelPage } from '@/pages/ItemLabelPage';
import { ItemNewPage } from '@/pages/ItemNewPage';
import { AuditLogPage } from '@/pages/AuditLogPage';
import { ItemsListPage } from '@/pages/ItemsListPage';
import { ItemsByDepartmentPage } from '@/pages/ItemsByDepartmentPage';
import { LocationsPage } from '@/pages/LocationsPage';
import { LoginPage } from '@/pages/LoginPage';
import { LostFoundDetailPage } from '@/pages/LostFoundDetailPage';
import { LostFoundEditPage } from '@/pages/LostFoundEditPage';
import { LostFoundLabelPage } from '@/pages/LostFoundLabelPage';
import { LostFoundListPage } from '@/pages/LostFoundListPage';
import { LostFoundNewPage } from '@/pages/LostFoundNewPage';
import { OidcCallbackPage } from '@/pages/OidcCallbackPage';
import { SettingsPage } from '@/pages/SettingsPage';
import { UsersPage } from '@/pages/UsersPage';
import { ScanCodePage } from '@/pages/ScanCodePage';
import { ImportExportPage } from '@/pages/ImportExportPage';
import { RequestsPage } from '@/pages/RequestsPage';
import { PackingListDetailPage, PackingListsPage } from '@/pages/PackingListsPage';
import { SignoutDetailPage } from '@/pages/SignoutDetailPage';
import { SignoutNewPage } from '@/pages/SignoutNewPage';
import { SignoutsListPage } from '@/pages/SignoutsListPage';

const rootRoute = createRootRoute({
  component: () => <Outlet />,
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  validateSearch: (search: Record<string, unknown>): { redirect: string } => {
    const r = search.redirect;
    if (typeof r === 'string' && r.startsWith('/')) {
      return { redirect: r };
    }
    return { redirect: '/dashboard' };
  },
  component: LoginPage,
});

const oidcCallbackRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/auth/callback',
  component: OidcCallbackPage,
});

async function requireSession() {
  if (!isOidcAuthMode()) {
    return;
  }
  const mgr = getUserManager();
  if (!mgr) {
    throw redirect({ to: '/login', search: { redirect: '/dashboard' } });
  }
  let user = await mgr.getUser();
  if (user?.expired) {
    try {
      user = await mgr.signinSilent();
    } catch {
      user = null;
    }
  }
  if (!user) {
    const path = typeof window !== 'undefined' ? `${window.location.pathname}${window.location.search}` : '/dashboard';
    const redirectTo = path.startsWith('/login') || path.startsWith('/auth/') ? '/dashboard' : path;
    throw redirect({ to: '/login', search: { redirect: redirectTo } });
  }
}

const appLayoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'app',
  component: Layout,
  beforeLoad: requireSession,
});

const indexRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/dashboard' });
  },
});

const dashboardRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/dashboard',
  component: DashboardPage,
});

const itemsListRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/items',
  // `q` lets the dashboard quick-search deep-link into a pre-filled list.
  validateSearch: (search: Record<string, unknown>): { q?: string } => {
    const q = search.q;
    return typeof q === 'string' && q.length > 0 ? { q } : {};
  },
  component: ItemsListPage,
});

const itemsByDepartmentRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/items/by-department',
  component: ItemsByDepartmentPage,
});

const itemNewRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/items/new',
  validateSearch: (search: Record<string, unknown>): { asset_tag?: string; clone_from?: string } => {
    const t = search.asset_tag;
    const c = search.clone_from;
    return {
      ...(typeof t === 'string' && t.length > 0 ? { asset_tag: t } : {}),
      ...(typeof c === 'string' && c.length > 0 ? { clone_from: c } : {}),
    };
  },
  component: ItemNewPage,
});

const itemDetailRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/items/$id',
  component: ItemDetailPage,
});

const itemEditRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/items/$id/edit',
  component: ItemEditPage,
});

const itemLabelRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/items/$id/label',
  beforeLoad: requireSession,
  component: ItemLabelPage,
});

// QR labels link here (/scan?code=…); without a code it goes to the
// dashboard, where the sidebar's scan dialog lives.
const scanRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/scan',
  validateSearch: (search: Record<string, unknown>): { code?: string } => ({
    code: typeof search.code === 'string' ? search.code : undefined,
  }),
  beforeLoad: ({ search }) => {
    if (!search.code) throw redirect({ to: '/dashboard' });
  },
  component: ScanCodePage,
});

const signoutsRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/signouts',
  component: SignoutsListPage,
});

const signoutNewRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/signouts/new',
  component: SignoutNewPage,
});

const signoutDetailRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/signouts/$id',
  component: SignoutDetailPage,
});

const lostFoundListRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/lost-and-found',
  component: LostFoundListPage,
});

const lostFoundNewRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/lost-and-found/new',
  component: LostFoundNewPage,
});

const lostFoundDetailRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/lost-and-found/$id',
  validateSearch: (search: Record<string, unknown>): { fresh?: boolean } => {
    const f = search.fresh;
    return f === true || f === 'true' || f === '1' ? { fresh: true } : {};
  },
  component: LostFoundDetailPage,
});

const lostFoundEditRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/lost-and-found/$id/edit',
  component: LostFoundEditPage,
});

const lostFoundLabelRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/lost-and-found/$id/label',
  beforeLoad: requireSession,
  component: LostFoundLabelPage,
});

const claimRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/claim/$reportNumber',
  component: ClaimPage,
});

const claimSearchRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/claim',
  component: ClaimPage,
});

const departmentsRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/departments',
  component: DepartmentsPage,
});

const locationsRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/locations',
  component: LocationsPage,
});

const categoriesRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/categories',
  component: CategoriesPage,
});

const auditLogRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/audit-log',
  component: AuditLogPage,
});

const settingsRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/settings',
  component: SettingsPage,
});

const requestsRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/requests',
  component: RequestsPage,
});

const packingListsRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/packing-lists',
  component: PackingListsPage,
});

const packingListDetailRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/packing-lists/$id',
  component: PackingListDetailPage,
});

const importExportRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/import-export',
  component: ImportExportPage,
});

const usersRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: '/users',
  component: UsersPage,
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  oidcCallbackRoute,
  itemLabelRoute,
  lostFoundLabelRoute,
  claimRoute,
  claimSearchRoute,
  appLayoutRoute.addChildren([
    indexRoute,
    dashboardRoute,
    itemsListRoute,
    itemsByDepartmentRoute,
    itemNewRoute,
    itemDetailRoute,
    itemEditRoute,
    scanRoute,
    signoutsRoute,
    signoutNewRoute,
    signoutDetailRoute,
    lostFoundListRoute,
    lostFoundNewRoute,
    lostFoundDetailRoute,
    lostFoundEditRoute,
    departmentsRoute,
    locationsRoute,
    categoriesRoute,
    auditLogRoute,
    settingsRoute,
    usersRoute,
    importExportRoute,
    requestsRoute,
    packingListsRoute,
    packingListDetailRoute,
  ]),
]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
