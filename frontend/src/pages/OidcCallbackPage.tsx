import { useEffect, useState } from 'react';
import { Link, useRouter } from '@tanstack/react-router';
import { Loader2, PackageOpen } from 'lucide-react';
import { getUserManager, isOidcAuthMode } from '@/auth/oidc';
import { oidcCallbackUrl } from '@/auth/captureCallbackUrl';
import { Card, CardContent } from '@/components/ui/Card';

export function OidcCallbackPage() {
  const router = useRouter();
  const [message, setMessage] = useState('Completing sign-in…');
  const pending = message === 'Completing sign-in…';

  useEffect(() => {
    if (!isOidcAuthMode()) {
      router.navigate({ to: '/dashboard' }).catch(() => undefined);
      return;
    }
    const mgr = getUserManager();
    if (!mgr) {
      setMessage('OIDC is not configured.');
      return;
    }
    const url = oidcCallbackUrl();
    console.debug('[stash] OIDC callback URL:', url);
    mgr
      .signinRedirectCallback(url)
      .then((user) => {
        console.debug('[stash] OIDC callback success, user:', user.profile?.preferred_username);
        const target = typeof user.state === 'string' && user.state.startsWith('/') ? user.state : '/dashboard';
        return router.navigate({ to: target });
      })
      .catch((e: unknown) => {
        console.error('[stash] OIDC callback error:', e);
        setMessage(e instanceof Error ? e.message : 'Sign-in callback failed');
      });
  }, [router]);

  return (
    <div className="relative min-h-screen flex items-center justify-center bg-awo-carbon p-4">
      <Card className="w-full max-w-md border-white/10 shadow-pop">
        <CardContent className="p-6 flex flex-col items-center text-center gap-3">
          <span className="rounded-pill bg-[#e44763]/15 p-3">
            <PackageOpen className="w-7 h-7 text-awo-amaranth" />
          </span>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            {pending ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
            <span className={pending ? '' : 'text-destructive'}>{message}</span>
          </div>
          {!pending ? (
            <Link to="/login" search={{ redirect: '/dashboard' }} className="text-sm text-primary underline">
              Back to login
            </Link>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
