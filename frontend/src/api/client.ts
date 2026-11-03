// Typed fetch wrapper. Always sends credentials in dev mode (X-Dev-User
// header) and a Bearer token in OIDC mode (token resolved by AuthProvider).

const API_BASE_URL =
  (typeof process !== 'undefined' ? process.env.STASH_API_BASE_URL : undefined) ??
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ??
  '';

export type ApiErrorBody = {
  error: {
    code: string;
    message: string;
    details?: unknown;
  };
};

export class ApiError extends Error {
  status: number;
  code: string;
  details?: unknown;

  constructor(status: number, body: ApiErrorBody) {
    super(body.error.message);
    this.status = status;
    this.code = body.error.code;
    this.details = body.error.details;
  }
}

type AuthHeaders = Record<string, string>;
let authHeadersResolver: () => Promise<AuthHeaders> = async () => ({});

export function setAuthHeadersResolver(fn: () => Promise<AuthHeaders>) {
  authHeadersResolver = fn;
}

type RequestOpts = {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE';
  body?: unknown;
  signal?: AbortSignal;
  query?: Record<string, string | number | boolean | undefined | null>;
  headers?: Record<string, string>;
};

export async function apiRequest<T>(path: string, opts: RequestOpts = {}): Promise<T> {
  const url = new URL(path.startsWith('http') ? path : `${API_BASE_URL}${path}`, window.location.origin);
  if (opts.query) {
    for (const [k, v] of Object.entries(opts.query)) {
      if (v === undefined || v === null) continue;
      url.searchParams.set(k, String(v));
    }
  }

  const auth = await authHeadersResolver();
  const headers: Record<string, string> = {
    Accept: 'application/json',
    ...(opts.body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    ...auth,
    ...(opts.headers ?? {}),
  };

  const res = await fetch(url.toString(), {
    method: opts.method ?? 'GET',
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    signal: opts.signal,
    headers,
    credentials: 'include',
  });

  if (res.status === 204) {
    return undefined as T;
  }

  const text = await res.text();
  const data = text ? JSON.parse(text) : null;

  if (!res.ok) {
    throw new ApiError(res.status, data as ApiErrorBody);
  }
  return data as T;
}

/**
 * fetchAuthed is a thin wrapper around fetch that injects the same auth
 * headers / credentials apiRequest uses. Returns the raw Response so the
 * caller can decide how to read it (e.g. .blob() for binary).
 */
export async function fetchAuthed(path: string, init: RequestInit = {}): Promise<Response> {
  const url = path.startsWith('http') ? path : `${API_BASE_URL}${path}`;
  const auth = await authHeadersResolver();
  const headers = new Headers(init.headers as HeadersInit | undefined);
  for (const [k, v] of Object.entries(auth)) {
    headers.set(k, v);
  }
  return fetch(url, { ...init, headers, credentials: 'include' });
}

/**
 * Upload a multipart/form-data payload using the same auth/credential
 * plumbing as apiRequest. Used for file uploads where we want the browser
 * to set its own Content-Type (with boundary).
 */
export async function apiUpload<T>(
  path: string,
  formData: FormData,
  opts: { signal?: AbortSignal } = {}
): Promise<T> {
  const url = new URL(
    path.startsWith('http') ? path : `${API_BASE_URL}${path}`,
    window.location.origin
  );

  const auth = await authHeadersResolver();
  const headers: Record<string, string> = {
    Accept: 'application/json',
    ...auth,
  };

  const res = await fetch(url.toString(), {
    method: 'POST',
    body: formData,
    signal: opts.signal,
    headers,
    credentials: 'include',
  });

  if (res.status === 204) {
    return undefined as T;
  }

  const text = await res.text();
  const data = text ? JSON.parse(text) : null;

  if (!res.ok) {
    throw new ApiError(res.status, data as ApiErrorBody);
  }
  return data as T;
}
