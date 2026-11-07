import { runtimeConfig } from '@/runtime-config';

/** Guest-facing /claim lookup — off by default until enabled at deploy time. */
export function isPublicClaimEnabled(): boolean {
  return runtimeConfig().publicClaimEnabled;
}
