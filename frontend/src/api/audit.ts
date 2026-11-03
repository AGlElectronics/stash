import { apiRequest } from './client';

export type AuditEntry = {
  id: string;
  actor_user_id?: string;
  /** Keycloak sub at time of action — stable even if nickname/email changes. */
  actor_oidc_subject?: string;
  /** Nickname / display label at time of action. */
  actor_display_name?: string;
  action: string;
  entity_type: string;
  entity_id: string;
  before: unknown;
  after: unknown;
  note: string;
  created_at: string;
};

export async function listAuditLog(query: {
  action?: string;
  entity_type?: string;
  entity_id?: string;
  limit?: number;
  offset?: number;
} = {}) {
  return apiRequest<{ items: AuditEntry[] }>('/api/audit-log', {
    query: query as Record<string, string | number | boolean | undefined | null>,
  });
}
