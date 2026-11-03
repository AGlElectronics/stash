/** Guest-facing /claim lookup — off by default until enabled at deploy time. */
export function isPublicClaimEnabled(): boolean {
  return import.meta.env.VITE_PUBLIC_CLAIM_ENABLED === 'true';
}
