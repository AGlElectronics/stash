# Deploying Stash to Kubernetes with Argo CD

This guide describes how to run Stash on a cluster **without sample inventory data**, using Argo CD for GitOps. It assumes you build and push container images from CI (e.g. GitLab) and that manifests (or Helm/Kustomize) live in this repo or a dedicated deploy repo.

> Portable Kubernetes / Argo CD starters live under [`deploy/kubernetes/`](../deploy/kubernetes). Adapt issuer URLs, image registries, and network policies for your cluster — do not assume a particular event IdP or legacy org namespace.

## Goals

- **No demo inventory** — Do not run smoke scripts or manual “seed” inserts for departments/items/sign-outs in production. The database should only contain schema + RBAC catalog from migrations, real IdP users (JIT), and real operational data.
- **No dev authentication** — `AUTH_MODE` must be `oidc` in the cluster. Never expose `X-Dev-User` in production ingress.
- **Secrets out of Git** — Use Kubernetes `Secret`, Sealed Secrets, or External Secrets Operator for `DATABASE_URL`, S3 credentials, and any future keys.

## What migrations already do

- **Included (required):** roles, permissions, `role_permissions`, default OIDC group→role **examples** (`stash-admins`, `stash-admin` → global admin, etc.), `system_settings` row.
- **Removed after bootstrap:** The placeholder **Dev Admin** user (`00000000-0000-0000-0000-000000000001`) is removed by migration `20260510000003_remove_dev_seed_user.sql` so clusters do not keep a known dev account. OIDC JIT provisioning creates real users on first login.

**You should still:**

- Replace or extend **OIDC group bindings** in the admin UI (Settings) to match your real Keycloak/Azure AD groups. The seeded group names are examples, not org policy.
- **Never** run [`scripts/smoke/`](../scripts/smoke/) PowerShell scripts or compose-only seed steps against production.

## Container images

Build from repository root (see `infra/Dockerfile.backend` and `infra/Dockerfile.frontend`):

- **Backend:** distroless, runs migrations on startup (`internal/app` → `db.MigrateUp`), listens on `HTTP_ADDR` (default `:8080`).
- **Frontend:** nginx static + `infra/nginx.conf` proxying `/api` to the backend service.

Push to your registry, e.g. `registry.example.com/stash/backend:1.2.3` and `registry.example.com/stash/frontend:1.2.3`.

**Image pull: `no such host` / `ErrImagePull` on the registry hostname**

If the kubelet reports something like `lookup registry.… on 127.0.0.53:53: no such host`, the **cluster’s DNS cannot resolve** the registry FQDN you put in the Deployment/Kustomize image. **Frontend and backend Deployments typically use the same registry host**, so you will see the **same pull error twice** (once per image) until DNS or the hostname is corrected. That is not fixed by changing application code; fix **name or DNS path**:

1. **Confirm the registry URL** in GitLab: **Admin** (or project) → **Settings** → **Container Registry** (or check the registry host shown next to push instructions). Self-managed GitLab often uses a dedicated host (e.g. `registry.example.com`) or the GitLab host with a port — it may **not** match a guessed name like `registry.git.example.com`.
2. **Align manifests with CI**: in GitLab CI, `CI_REGISTRY` / `CI_REGISTRY_IMAGE` are the canonical host and image prefix after login. Use the same host in Kubernetes image references.
3. **Cluster DNS**: ensure CoreDNS (or node resolvers) can resolve that hostname — e.g. forward to your corporate DNS, split-horizon for internal zones, or deploy a **reachable** registry (mirror, pull-through cache) inside the cluster network. From a debug pod: `nslookup <registry-host>` should succeed before image pulls will work.

**Frontend image — Kubernetes vs Docker Compose**

- Compose uses `infra/nginx.conf` (upstream `backend:8080`).
- Cluster builds must pass **`--build-arg NGINX_CONF=nginx.k8s.conf`** so nginx proxies to the `stash-backend` Service (`infra/nginx.k8s.conf`).

Example:

```bash
docker build -f infra/Dockerfile.frontend \
  --build-arg NGINX_CONF=nginx.k8s.conf \
  -t registry.example.com/stash/frontend:1.2.3 .
```

**OIDC settings: web app `/config.json` vs backend runtime**

The frontend image names no domain. Its sign-in settings live in
`/config.json`, which nginx serves from `/etc/conservices/config.json`:
mount a ConfigMap there (the Helm chart's `frontend.config` and the
Kustomize manifests do this). Reference:
<https://aglelectronics.github.io/con-services/modules/stash/configuration/#web-app-settings-configjson>.

| What | Web app **`/config.json`** key | Backend **runtime** env (must match IdP) |
|------|--------------------------------|----------------------------------------|
| Issuer / authority URL (Keycloak realm root) | `oidcAuthority` — e.g. `https://sso.example.com/realms/conservices` (no trailing slash) | `OIDC_ISSUER_URL` — same issuer URL the API trusts for JWTs |
| SPA client id (public client in IdP) | `oidcClientId` — e.g. `stash-spa` | `OIDC_CLIENT_ID` — often the same SPA client id |
| Scopes for browser login | `oidcScope` — typically `openid profile email` | (not usually needed for API if using bearer tokens from same client) |
| Auth mode | `authMode: "oidc"` | `AUTH_MODE=oidc` |
| Guest claim page | `publicClaimEnabled` | `PUBLIC_LOSTFOUND_CLAIM_ENABLED` |
| API audience / resource | (only if your SPA code adds it; current app uses authority + client_id) | `OIDC_AUDIENCE` if your tokens include a resource audience claim the API checks |

Also set **`NGINX_CONF=nginx.k8s.conf`** for cluster (proxies `/api` to the `stash-backend` Service).

**Verify what the browser gets** (wrong values here mean login stays broken):

```bash
curl -s https://stash.example.com/config.json
```

The web app shows "Not configured yet" while `oidcAuthority` or
`oidcClientId` is empty.

**Backend runtime env:**

| Variable | Required | Notes |
|----------|----------|--------|
| `DATABASE_URL` | Yes | Postgres connection string |
| `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_REGION` | Yes | Real S3 **or** temporary MinIO (same variables; swap when TL provisions the bucket) |
| `S3_USE_PATH_STYLE` | Often `true` for MinIO | `false` for typical AWS; `true` for most self-hosted S3-compatible endpoints |
| `S3_PUBLIC_BASE_URL` | Optional | Only if presigned URLs must rewrite host |
| `AUTH_MODE` | `oidc` | **Required in cluster** |
| `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_AUDIENCE` | Yes for OIDC | Match your IdP SPA client |
| `APP_BASE_URL` | Recommended | Public URL of the API if used in links |
| `CORS_ALLOWED_ORIGINS` | Yes | Exact SPA origin(s), comma-separated |
| `APP_ENV` | `production` | Disables console log formatting |

## Network layout

Typical pattern:

1. **Ingress** → `stash-frontend` Service (nginx on `:80`).  
2. Nginx proxies `/api/*` to `stash-backend` Service (`:8080`).  
3. Browser calls **same origin** → no CORS for API (still set `CORS_ALLOWED_ORIGINS` to that origin for preflight safety).

Alternatively: separate ingress hosts for API and SPA (update CORS and `apiBaseUrl` in the web app's `/config.json` accordingly).

## Argo CD Application

Point Argo CD at a path that contains Kubernetes manifests or Kustomize/Helm, for example:

- This repo: `deploy/kubernetes/overlays/production` (see starter layout below), **or**
- A private “platform” repo that pins image digests and environment-specific values.

**Recommended `Application` settings:**

- **Sync policy:** automated with `Prune` only after review, or manual for first prod deploy.
- **Health checks:** Deployment + Ingress (and optional Postgres operator CR if used).
- **Hooks:** Optional `PreSync` Job for **read-only** checks; **do not** use a Job to insert sample data.

## Database

- Run **managed PostgreSQL** (RDS, Cloud SQL, CNPG, etc.) — not emptyDir.
- Migrations run **inside the backend** on each start. Use **single-replica** rollout or a short maintenance window if you ever need stricter migration control; alternatively extract migrations to a Helm pre-upgrade Job (advanced).

## Object storage

Stash only talks to storage through the **S3 API** (`S3_ENDPOINT`, `S3_BUCKET`, keys, region, path-style). That can be:

1. **A real managed bucket** (recommended for long-lived production) — e.g. AWS S3, or another provider with private ACLs and network access from the cluster.
2. **A temporary “fake” bucket** — any S3-compatible service (usually **MinIO**) until your TL provisions the real bucket. No code changes: swap env vars (and optionally migrate objects — see below).

### Temporary: MinIO (or similar) in / near the cluster

- Deploy MinIO **inside the cluster** or on a URL the cluster can reach; keep it **private** (no anonymous read, unlike the dev `docker-compose` init script).
- Point the backend at it, for example:
  - `S3_ENDPOINT=http://stash-minio.stash.svc.cluster.local:9000` (adjust name/namespace)
  - `S3_BUCKET=stash-temp` (create the bucket once, or rely on the app’s startup `MakeBucket` if credentials allow)
  - `S3_USE_PATH_STYLE=true`
  - `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` = MinIO root user (or a dedicated IAM-style user) — store in `stash-secrets` like any other S3 backend.
- A **minimal example** manifest (edit secrets, not production-hardened) lives at [`deploy/kubernetes/examples/minio-temp.yaml`](../deploy/kubernetes/examples/minio-temp.yaml).

When the **real** bucket exists, update the backend Secret / env to the new endpoint and credentials, redeploy, and optionally **copy objects** from the temp bucket with `mc mirror` or provider tooling so existing `files.storage_key` rows still resolve (keys stay the same if you mirror the bucket contents 1:1). If you start fresh on the new bucket, old file rows in Postgres may point at missing keys — only acceptable if that environment had no important uploads.

### Production checklist (real S3)

- **Private** bucket; no public ACL / anonymous list.
- `S3_USE_PATH_STYLE` typically **`false`** for AWS; **`true`** for many MinIO-style endpoints.
- Ensure the backend can reach the endpoint (VPC endpoint, egress, firewall).

## Starter manifests

See `deploy/kubernetes/README.md` for a minimal Kustomize base you can copy and customize (namespace, image names, Ingress host, Secret references).

## Post-deploy checklist

1. Open SPA → OIDC login works; first admin appears in DB via JIT.  
2. In **Settings → OIDC group bindings**, configure real groups → roles.  
3. Confirm **no** `item` / `department` rows unless created intentionally (`SELECT COUNT(*) FROM items;`).  
4. Verify bucket is private and file download works only when authenticated.  
5. Run vulnerability scan on deployed images and smoke-test **read-only** API checks from CI against staging, not production seed scripts.

## Related reading

- [security-hardening-plan.md](./security-hardening-plan.md) — attack surface and backlog.  
- `.env.example` — variable descriptions for local parity (not for copying secrets into cluster).
