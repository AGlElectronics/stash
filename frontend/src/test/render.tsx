import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import type { ReactElement } from 'react';

/** Render with a fresh React Query client (no retries, no caching between tests). */
export function renderWithQuery(ui: ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

/** A /api/me answer with the given permissions. */
export function meWith(...permissions: string[]) {
  return { id: 'me-1', display_name: 'Tester', email: 't@example.org', oidc_subject: 'sub', roles: [], permissions };
}
