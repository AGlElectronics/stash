import { runtimeConfig } from '@/runtime-config';
/**
 * Public (unauthenticated) Lost & Found API. These endpoints do not require
 * a session — they're meant for guests scanning a QR label.
 */

const apiBaseUrl = () =>
  (typeof process !== 'undefined' ? process.env.STASH_API_BASE_URL : undefined) ??
  runtimeConfig().apiBaseUrl;

/** What guests may see: no description, location or photo. */
export type PublicLostFoundReport = {
  report_number: string;
  item_name: string;
  /** Day only, YYYY-MM-DD. */
  date_reported: string;
};

export async function getPublicReport(reportNumber: string): Promise<PublicLostFoundReport> {
  const url = `${apiBaseUrl()}/api/public/lostfound/${encodeURIComponent(reportNumber)}`;
  const res = await fetch(url, { headers: { Accept: 'application/json' } });
  if (!res.ok) {
    const body = await res.text();
    const parsed = body ? JSON.parse(body) : { error: { message: 'Not found' } };
    throw new Error(parsed.error?.message ?? `HTTP ${res.status}`);
  }
  return res.json();
}

export type SearchPublicLostFoundParams = {
  q?: string;
  limit?: number;
  offset?: number;
};

export type PublicLostFoundList = {
  items: PublicLostFoundReport[];
  total: number;
};

export async function searchPublicReports(params: SearchPublicLostFoundParams = {}): Promise<PublicLostFoundList> {
  const url = new URL(`${apiBaseUrl()}/api/public/lostfound`, window.location.origin);
  if (params.q) url.searchParams.set('q', params.q);
  if (params.limit) url.searchParams.set('limit', String(params.limit));
  if (params.offset) url.searchParams.set('offset', String(params.offset));

  const res = await fetch(url.toString(), { headers: { Accept: 'application/json' } });
  if (!res.ok) {
    const body = await res.text();
    const parsed = body ? JSON.parse(body) : { error: { message: 'Search failed' } };
    throw new Error(parsed.error?.message ?? `HTTP ${res.status}`);
  }
  return res.json();
}
