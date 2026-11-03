import { useEffect, useState } from 'react';
import { useRouter, useSearch } from '@tanstack/react-router';
import { PackageOpen } from 'lucide-react';
import { isOidcAuthMode, getUserManager, signInWithKeycloak } from '@/auth/oidc';
import { useBrand } from '@/brand/BrandProvider';
import { Button } from '@/components/ui/Button';
import { Card, CardContent } from '@/components/ui/Card';

export function LoginPage() {
  const router = useRouter();
  const { redirect } = useSearch({ from: '/login' });
  const { brand } = useBrand();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOidcAuthMode()) {
      router.navigate({ to: '/dashboard' }).catch(() => undefined);
      return;
    }
    const mgr = getUserManager();
    if (!mgr) {
      setError('OIDC is enabled but VITE_OIDC_AUTHORITY / VITE_OIDC_CLIENT_ID are missing from the build.');
      return;
    }
    mgr.getUser().then((user) => {
      if (user && !user.expired) {
        router.navigate({ to: redirect }).catch(() => undefined);
      }
    });
  }, [router, redirect]);

  const returnPath = redirect;
  const carbon = brand.colors.carbon;
  const primary = brand.colors.primary;

  return (
    <div
      className="relative min-h-screen flex flex-col items-center justify-center p-4 overflow-hidden"
      style={{ backgroundColor: carbon }}
    >
      <div
        aria-hidden
        className="pointer-events-none absolute -top-32 left-1/2 -translate-x-1/2 w-[40rem] h-[40rem] rounded-full opacity-25"
        style={{ background: `radial-gradient(closest-side, ${primary}, transparent)` }}
      />
      <div className="relative w-full max-w-md space-y-8">
        <div className="flex flex-col items-center gap-3 text-center">
          <span className="rounded-pill p-4" style={{ backgroundColor: `${primary}26` }}>
            <PackageOpen className="w-10 h-10" style={{ color: primary }} />
          </span>
          <div>
            <span className="block font-heading text-5xl font-extrabold tracking-tight leading-none text-white">
              {brand.productName}
            </span>
            <p className="text-base font-medium text-white/60 mt-2">
              {brand.organizationName
                ? `${brand.organizationName} · ${brand.loginSubtitle}`
                : brand.loginSubtitle}
            </p>
          </div>
        </div>

        <Card className="border-white/10 shadow-pop">
          <CardContent className="space-y-4 p-6">
            <div>
              <h1 className="font-heading font-bold text-lg">Welcome back!</h1>
              <p className="text-sm text-muted-foreground mt-1">{brand.loginAccountHint}</p>
            </div>
            {error ? (
              <p className="text-sm text-destructive" role="alert">
                {error}
              </p>
            ) : null}
            <Button
              className="w-full"
              size="lg"
              disabled={!!error}
              onClick={() => {
                setError(null);
                signInWithKeycloak(returnPath).catch((e: unknown) => {
                  setError(e instanceof Error ? e.message : 'Sign-in failed');
                });
              }}
            >
              {brand.loginButtonLabel}
            </Button>
            <p className="text-xs text-muted-foreground text-center">
              Local development without an identity provider? Build with{' '}
              <code className="rounded bg-muted px-1 py-0.5">VITE_AUTH_MODE=dev</code> (default).
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
