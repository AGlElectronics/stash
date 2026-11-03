import { useEffect, useState } from 'react';
import { Link, Outlet, useRouterState } from '@tanstack/react-router';
import { isOidcAuthMode, signOutKeycloak } from '@/auth/oidc';
import { ThemeToggle } from '@/components/ThemeToggle';
import { ScanDialog } from '@/components/ScanDialog';
import { useMe, PERMS } from '@/hooks/useMe';
import {
  Boxes,
  FolderTree,
  LayoutDashboard,
  LogOut,
  Menu,
  PackageOpen,
  ScanLine,
  Search,
  Send,
  Settings,
  Users,
  X,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useBrand } from '@/brand/BrandProvider';

type NavItem = {
  to: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  /** Permissions, any of which makes the item visible. Empty = always visible. */
  perms: string[];
};

type NavGroup = { label?: string; items: NavItem[] };

const NAV_GROUPS: NavGroup[] = [
  {
    items: [{ to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard, perms: [] }],
  },
  {
    label: 'Inventory',
    items: [
      { to: '/items', label: 'Items', icon: Boxes, perms: [PERMS.itemView] },
      { to: '/items/by-department', label: 'By department', icon: FolderTree, perms: [PERMS.itemView] },
    ],
  },
  {
    label: 'Operations',
    items: [
      { to: '/signouts', label: 'Sign-outs', icon: Send, perms: [PERMS.signoutView] },
      { to: '/lost-and-found', label: 'Lost & Found', icon: Search, perms: [PERMS.lostfoundView] },
      { to: '/departments', label: 'Departments', icon: Users, perms: [PERMS.itemView, PERMS.departmentManage] },
    ],
  },
];

function NavLinks({ onNavigate }: { onNavigate?: () => void }) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { canAny } = useMe();

  const showSettings = canAny(
    PERMS.locationManage,
    PERMS.categoryManage,
    PERMS.auditView,
    PERMS.departmentManage
  );

  return (
    <>
      {NAV_GROUPS.map((group, gi) => {
        const visible = group.items.filter(
          (item) => item.perms.length === 0 || canAny(...item.perms)
        );
        if (visible.length === 0) return null;
        return (
          <div key={group.label ?? gi}>
            {group.label ? (
              <p className="px-3 pt-4 pb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                {group.label}
              </p>
            ) : null}
            <div className="space-y-0.5">
              {visible.map((item) => {
                const isActive =
                  pathname === item.to ||
                  (pathname.startsWith(item.to + '/') &&
                    // `/items/by-department` must not light up the plain Items entry.
                    !NAV_GROUPS.some((g) =>
                      g.items.some((o) => o.to !== item.to && o.to.startsWith(item.to + '/') && pathname.startsWith(o.to))
                    ));
                return (
                  <Link
                    key={item.to}
                    to={item.to}
                    onClick={onNavigate}
                    className={cn(
                      'flex items-center gap-2.5 px-3 py-2 rounded-pill text-sm font-medium whitespace-nowrap transition-colors',
                      isActive
                        ? 'bg-accent text-accent-foreground'
                        : 'text-muted-foreground hover:bg-secondary hover:text-foreground'
                    )}
                  >
                    <item.icon className="w-4 h-4 shrink-0" />
                    {item.label}
                  </Link>
                );
              })}
            </div>
          </div>
        );
      })}
      {showSettings ? (
        <div>
          <p className="px-3 pt-4 pb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
            Admin
          </p>
          <Link
            to="/settings"
            onClick={onNavigate}
            className={cn(
              'flex items-center gap-2.5 px-3 py-2 rounded-pill text-sm font-medium whitespace-nowrap transition-colors',
              pathname.startsWith('/settings') ||
                pathname.startsWith('/locations') ||
                pathname.startsWith('/categories') ||
                pathname.startsWith('/audit-log')
                ? 'bg-accent text-accent-foreground'
                : 'text-muted-foreground hover:bg-secondary hover:text-foreground'
            )}
          >
            <Settings className="w-4 h-4 shrink-0" />
            Settings
          </Link>
        </div>
      ) : null}
    </>
  );
}

function Wordmark() {
  const { brand } = useBrand();
  return (
    <div className="flex items-center gap-2.5">
      <span className="rounded-pill bg-primary-soft p-1.5">
        <PackageOpen className="w-5 h-5 text-primary" />
      </span>
      <span className="leading-none">
        <span className="block font-heading text-xl font-extrabold tracking-tight">
          {brand.productName}
        </span>
        {brand.organizationName ? (
          <span className="block text-[11px] font-medium text-muted-foreground mt-0.5">
            {brand.organizationName}
          </span>
        ) : null}
      </span>
    </div>
  );
}

export function Layout() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { can } = useMe();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [scanOpen, setScanOpen] = useState(false);

  // Close the mobile drawer whenever navigation happens.
  useEffect(() => {
    setDrawerOpen(false);
  }, [pathname]);

  const canScan = can(PERMS.itemView);

  const footer = (
    <div className="mt-auto p-2 border-t border-border space-y-0.5">
      <ThemeToggle className="rounded-pill" />
      {isOidcAuthMode() ? (
        <button
          type="button"
          className="w-full flex items-center gap-2 text-left text-sm px-3 py-2 rounded-pill text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors"
          onClick={() => {
            signOutKeycloak().catch(() => undefined);
          }}
        >
          <LogOut className="w-4 h-4 shrink-0" />
          Sign out
        </button>
      ) : null}
    </div>
  );

  return (
    <div className="min-h-screen w-full flex flex-col md:flex-row bg-background text-foreground">
      {/* Mobile top bar */}
      <header className="md:hidden sticky top-0 z-40 flex items-center gap-2 px-3 py-2.5 border-b border-border bg-card">
        <button
          type="button"
          className="p-2 rounded-pill hover:bg-secondary transition-colors"
          onClick={() => setDrawerOpen(true)}
          aria-label="Open menu"
        >
          <Menu className="w-5 h-5" />
        </button>
        <Wordmark />
        {canScan ? (
          <button
            type="button"
            onClick={() => setScanOpen(true)}
            className="ml-auto inline-flex items-center gap-1.5 rounded-pill bg-primary text-primary-foreground px-3.5 py-2 text-sm font-semibold"
          >
            <ScanLine className="w-4 h-4" />
            Scan
          </button>
        ) : null}
      </header>

      {/* Mobile drawer */}
      {drawerOpen ? (
        <div className="md:hidden fixed inset-0 z-50 bg-black/50 backdrop-blur-[2px]" onClick={() => setDrawerOpen(false)}>
          <aside
            className="absolute inset-y-0 left-0 w-72 max-w-[85vw] bg-card border-r border-border flex flex-col"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="px-4 py-4 flex items-center justify-between border-b border-border">
              <Wordmark />
              <button
                type="button"
                className="p-2 rounded-pill hover:bg-secondary transition-colors"
                onClick={() => setDrawerOpen(false)}
                aria-label="Close menu"
              >
                <X className="w-5 h-5" />
              </button>
            </div>
            <nav className="p-2 flex-1 overflow-y-auto">
              <NavLinks onNavigate={() => setDrawerOpen(false)} />
            </nav>
            {footer}
          </aside>
        </div>
      ) : null}

      {/* Desktop sidebar */}
      <aside className="hidden md:flex shrink-0 md:w-60 md:min-h-screen border-r border-border bg-card flex-col">
        <div className="px-4 py-4 border-b border-border">
          <Wordmark />
        </div>
        {canScan ? (
          <div className="px-3 pt-3">
            <button
              type="button"
              onClick={() => setScanOpen(true)}
              className="w-full inline-flex items-center justify-center gap-2 rounded-pill bg-primary text-primary-foreground px-4 py-2 text-sm font-semibold transition-all duration-150 hover:bg-primary-hover hover:-translate-y-px"
            >
              <ScanLine className="w-4 h-4" />
              Scan barcode
            </button>
          </div>
        ) : null}
        <nav className="p-2 md:flex-1 overflow-y-auto">
          <NavLinks />
        </nav>
        {footer}
      </aside>

      <main className="flex-1 min-w-0 min-h-0 flex flex-col w-full md:min-h-screen">
        <Outlet />
      </main>

      <ScanDialog open={scanOpen} onClose={() => setScanOpen(false)} />
    </div>
  );
}
