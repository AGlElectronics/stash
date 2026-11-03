# Security hardening plan & attack surface

This document inventories how Stash is exposed today, prioritizes hardening work, and ties each item to concrete mitigations. Use it as a backlog for pre-production and ongoing reviews.

## Architecture (trust boundaries)

| Zone | Component | Notes |
|------|-----------|--------|
| Browser | React SPA | OIDC code flow + Bearer token to API; dev mode uses `X-Dev-User` |
| Edge | Ingress / reverse proxy | TLS termination, optional WAF, rate limits |
| App | Go API (`:8080`) | Chi router, CORS, auth middleware, RBAC on routes |
| Data | PostgreSQL | sqlc + parameterized queries |
| Objects | S3-compatible (MinIO / AWS) | Presigned URLs + backend-mediated upload/stream |

## Attack surface summary

### 1. Authentication & session

| Risk | Severity | Current state | Recommended actions |
|------|----------|---------------|----------------------|
| **Dev auth in production** | Critical | `AUTH_MODE=dev` trusts `X-Dev-User` (or default user) | **Never** set `AUTH_MODE=dev` in cluster; enforce via CI/CD policy + config validation |
| **OIDC misconfiguration** | High | Wrong issuer / audience allows token confusion | Pin `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_AUDIENCE`; rotate client secrets; short access token TTL at IdP |
| **Token in browser** | Medium | Access token in memory/sessionStorage (oidc-client-ts) | Prefer short-lived access tokens; rely on silent renew; document XSS impact (CSP below) |

### 2. Authorization (RBAC)

| Risk | Severity | Current state | Recommended actions |
|------|----------|---------------|----------------------|
| **Over-privileged routes** | Medium | Some routes may be authenticated-only without per-permission checks | Audit `router.go` groups; align file uploads with `file.upload` where intended |
| **JIT provisioning** | Low–Med | OIDC users upserted on login; roles from group bindings | Lock down who can edit `access.manage`; audit `oidc_group_role_bindings` regularly |

### 3. API abuse

| Risk | Severity | Current state | Recommended actions |
|------|----------|---------------|----------------------|
| **No rate limiting** | Medium | Nothing in-app | Ingress rate limits, WAF, or API gateway (per-IP / per-user) |
| **Large payloads** | Low | Nginx `client_max_body_size 50m`; backend multipart limit | Tune per environment; consider stricter limits for anonymous paths (none today) |
| **CORS** | Medium | `CORS_ALLOWED_ORIGINS` allowlist | Set **exact** production origins only; no `*` with credentials |

### 4. File / object storage

| Risk | Severity | Current state | Recommended actions |
|------|----------|---------------|----------------------|
| **Bucket policy** | High (dev) | Docker `minio-init` sets **public download** on dev bucket | **Do not** use public buckets in prod; private bucket + presigned GET or authenticated `/api/files/{id}/raw` only |
| **Upload size / type** | Medium | Image vs attachment caps; `image/*` check for image file type | Keep; consider magic-byte validation for sensitive deployments |
| **Path traversal in metadata** | Low | `entity_type` used in object keys | Sanitize / allowlist `entity_type` in service layer if ever user-controlled |
| **IDOR on files** | Medium | Any authenticated user can request download/raw if they know UUID | **Hardening backlog:** enforce “actor may access entity” before streaming (or scope presigned URLs to short TTL + audit) |

### 5. Data layer

| Risk | Severity | Current state | Recommended actions |
|------|----------|---------------|----------------------|
| **SQL injection** | Low | sqlc + pgx parameters | Continue parameterized SQL only; lint migrations |
| **Secrets in env** | High | DB URL, S3 keys, OIDC in env | Kubernetes Secrets / External Secrets Operator; no secrets in Git; rotate |

### 6. Supply chain & build

| Risk | Severity | Recommended actions |
|------|----------|----------------------|
| Dependencies | Medium | `npm audit`, `govulncheck`, Dependabot/Renovate |
| Images | Medium | Pin base image digests; scan in CI (Trivy, etc.) |
| SBOM | Low | Generate CycloneDX / SPDX for releases |

### 7. Observability & forensics

| Risk | Severity | Recommended actions |
|------|----------|----------------------|
| **Sensitive logs** | Medium | Ensure tokens / passwords never logged (review zerolog fields) |
| **Audit log** | Low | Already writing audit events; protect `audit.view` and DB backups |

### 8. Browser security headers

| Risk | Severity | Recommended actions |
|------|----------|----------------------|
| XSS | Medium | CSP for SPA (nonce or strict `default-src`); sanitize any future HTML |
| Clickjacking | Low | `X-Frame-Options` or CSP `frame-ancestors` on ingress |

## Phased roadmap (suggested)

1. **P0 — Production baseline**  
   - `AUTH_MODE=oidc` only; private S3 bucket; TLS everywhere; secrets via Secret manager; CORS allowlist; remove dev seed user from DB (see migration `20260510000003_remove_dev_seed_user.sql`).

2. **P1 — Abuse resistance**  
   - Ingress rate limiting; optional file access checks (IDOR); gate destructive file operations on `file.upload` + entity ownership rules.

3. **P2 — Assurance**  
   - Periodic penetration test; dependency scanning in CI; CSP headers.

## Review cadence

- **Quarterly:** Re-read this doc, check new routes/migrations, verify prod config.  
- **On change:** Auth, file handling, or CORS changes require explicit security review.
