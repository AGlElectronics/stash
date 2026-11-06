# Stash

Reusable inventory module for Con Services (AGl Electronics). Tracks items, locations, departments, and sign-outs end-to-end with a full audit trail and per-item lifecycle history.

## What's in the box

- **Backend** — Go 1.25, [chi](https://github.com/go-chi/chi), PostgreSQL via [pgx](https://github.com/jackc/pgx) and [sqlc](https://sqlc.dev), [Goose](https://github.com/pressly/goose) migrations, S3-compatible object storage (MinIO in dev), [zerolog](https://github.com/rs/zerolog) structured logs.
- **Frontend** — React 19, [TanStack Router](https://tanstack.com/router) + [TanStack Query](https://tanstack.com/query), [Rsbuild](https://rsbuild.dev) (no Vite per project policy), Tailwind + shadcn-style components, PWA manifest + service worker.
- **Auth** — Two modes:
  - `dev`: `X-Dev-User` header bypass (local dev, fast iteration).
  - `oidc`: OIDC bearer JWTs verified against an issuer like Keycloak; users are JIT-provisioned and their roles are recomputed on every request from the OIDC `groups`/`roles` claim.
- **RBAC** — Hybrid: fixed Go role names (`admin`, `inventory_manager`, `dept_lead`, `volunteer`, `readonly`) and DB-managed bindings that map external IdP groups to those roles, optionally scoped to a department.
- **CI** — GitLab CI pipeline at [.gitlab-ci.yml](.gitlab-ci.yml) covering lint, unit tests, an integration test stage with Postgres, and artifact builds for both apps.

## Repository layout

```text
stash/
  backend/          # Go service (cmd/server, internal/*)
    cmd/server/     # main entry point
    internal/
      app/          # bootstrap (config, db, migrations, files, http)
      auth/         # OIDC verifier, dev bypass, RBAC user/permissions
      config/       # env loading + validation
      db/           # pgx pool + transaction helper + migration runner
      domain/       # enums and string constants for permissions/roles
      events/       # per-item lifecycle event writer
      audit/        # forensic audit log writer
      files/        # MinIO/S3 client + service
      http/         # chi router + per-resource handlers + DTOs
      service/      # business logic (transactional, audit-emitting)
      repository/   # sqlc-generated typed queries (in sqlcgen/)
    queries/        # SQL files consumed by sqlc
    migrations/     # Goose migrations (embedded into the binary)
  frontend/         # React 19 SPA (Rsbuild, PWA)
    src/api/        # typed fetch wrappers per resource
    src/auth/       # dev auth shim (sets X-Dev-User from localStorage)
    src/components/ # ui kit + feature components
    src/forms/      # React Hook Form + Zod forms
    src/hooks/      # TanStack Query hooks for reference data
    src/pages/      # one component per route
    src/router.tsx  # code-based TanStack Router config
  infra/            # Dockerfiles, nginx.conf, docker-compose.yml
  docs/             # security hardening plan, Argo CD / prod deployment notes
  deploy/kubernetes/# Kustomize starter for cluster (no seed data)
  scripts/smoke/    # PowerShell API smoke tests (local dev)
  .gitlab-ci.yml    # custom pipeline (replaces Auto-DevOps)
```

## Quick start

### 1. Local development (Docker Compose)

```sh
cp .env.example .env
docker compose -f infra/docker-compose.yml up --build
```

This brings up:

- `postgres` on `:5432` (database `stash`, user `stash`, password `stash`)
- `minio` on `:9000` (console on `:9001`, credentials `stash`/`stashstash`)
- `minio-init` (one-shot to create the `stash-dev` bucket)
- `backend` on `:8080` (runs migrations on boot)
- `frontend` on `:3000`

Visit [http://localhost:3000](http://localhost:3000). Auth is in `dev` mode — every request uses the seeded `dev-admin` user with global admin permissions.

### 2. Local development (without Docker)

Requires Go 1.25+, Node 20+, a running PostgreSQL, and a MinIO (or S3) endpoint.

```sh
# Backend
cd backend
go run ./cmd/server          # picks up env from ../.env

# Frontend (in another shell)
cd frontend
npm install
npm run dev                  # http://localhost:3000, proxies /api to :8080
```

## Configuration

All backend config is environment-driven; see [.env.example](.env.example). Required for production:

- `DATABASE_URL` — Postgres DSN.
- `S3_*` — Bucket and credentials for object storage (MinIO works fine).
- `AUTH_MODE=oidc`, `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID` — for real auth. Optional `OIDC_AUDIENCE` if it differs from the client id.
- `ACCESS_CONFIG_PATH` — path to the access config TOML file (required in OIDC mode, see below). Legacy alias: `OIDC_BINDINGS_PATH`.
- `CORS_ALLOWED_ORIGINS` — comma-separated list of allowed origins for the SPA.

## RBAC model

Permissions are simple strings (`item.create`, `signout.return`, `lostfound.view`, …) listed in [backend/internal/domain/domain.go](backend/internal/domain/domain.go).

Roles bundle permissions:

| role | scope | what it can do |
| --- | --- | --- |
| `admin` | global | everything |
| `inventory_manager` | global or dept | manage items, sign-outs, locations, categories |
| `dept_lead` | dept | sign things out and accept returns for their department |
| `volunteer` | dept | view + use existing sign-outs |
| `readonly` | global | view-only |
| `frontdesk` | global | Lost & Found only (view / create / resolve reports) |

Access control is managed **exclusively in the access config TOML file** pointed at by `ACCESS_CONFIG_PATH` — there is no access-control UI and no role data in the database. The file maps OIDC groups to roles and defines each role's permissions:

```toml
# Permission catalog (reference) at the top of infra/access-config.toml

[roles.admin]
permissions = ["item.view", "item.create", "lostfound.view", …]

[roles.frontdesk]
permissions = ["lostfound.view", "lostfound.create", "lostfound.resolve", "file.upload"]

[[bindings]]
group = "stash-admin"       # exact OIDC group / client role claim
role = "admin"
# department_id = "<uuid>"  # optional, scopes the role to one department
```

When a user authenticates via OIDC, their `groups` claim (or `roles` if your IdP uses that name) is matched against the bindings and their permissions are computed per-request — nothing is persisted, so removing a user from a group in the IdP takes effect on their next request. The file is read once at startup; restart the backend to pick up changes. In `AUTH_MODE=dev` the file is optional and dev users are admins.

## Item lifecycle and history

Every state change to an item — moves, condition changes, damage, repair, sign-out, return, retire/restore — writes a row to `item_events` inside the same transaction. The detail page at `/items/:id` renders this as a timeline with icons per event type.

A separate `audit_log` table captures the same events plus all CRUD operations on departments, categories, locations, files and bindings. It records the actor, before/after JSON snapshots, and any operator-supplied note. The Audit Log page (`/audit-log`) gives admins a filterable view.

## Sign-out flow

The hot path is the **fast sign-out** from the items list:

1. Logistics opens `/items`, types "power strip" in the search box, sees `Available 13 / 15 owned`.
2. Clicks **Sign out** on the row. A small dialog opens, pre-filled with the right item; user picks a department and types the recipient's name.
3. Click confirm → backend creates a sign-out, jumps the state machine straight to `signed_out`, decrements availability, writes the audit + event rows. Done in under a minute.

Returns are processed from the sign-out detail page (`/signouts/:id`): operators enter quantities returned / consumed / lost / damaged per line. The state machine moves to `partially_returned` until everything is settled, then to `returned`. Admins can `close` returned sign-outs to archive them.

## CSV exports

- `GET /api/export/items.csv` — full item dump.
- `GET /api/export/signouts.csv` — full sign-out dump.

Both require the `export.create` permission (admin / inventory_manager by default).

## Running tests

Generated **sqlc** code (`internal/repository/sqlcgen/`) is not committed (see `.gitignore`). Regenerate after changing `queries/*.sql` or migrations used as schema:

```sh
cd backend
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest   # once
sqlc generate
```

```sh
# Backend
cd backend
sqlc generate                # required on fresh clones before build/test/lint
go test ./...
go vet ./...
golangci-lint run            # if installed

# Frontend
cd frontend
npm run lint
npm run typecheck
npm run test
```

## Smoke tests

PowerShell scripts under [`scripts/smoke/`](scripts/smoke/) hit a running stack (Compose or local). Examples:

- [`scripts/smoke/smoke-test.ps1`](scripts/smoke/smoke-test.ps1) — base CRUD + item lifecycle (backend `:8080`).
- [`scripts/smoke/smoke-signouts.ps1`](scripts/smoke/smoke-signouts.ps1) — sign-out create / return / close + history.
- [`scripts/smoke/smoke-fullstack.ps1`](scripts/smoke/smoke-fullstack.ps1) — seed + sign-out + frontend check (`:3000` proxy).
- [`scripts/smoke/smoke-images.ps1`](scripts/smoke/smoke-images.ps1) — file upload / primary image.
- [`scripts/smoke/smoke-rbac.ps1`](scripts/smoke/smoke-rbac.ps1) — access API smoke.

From repo root: `pwsh -File scripts/smoke/smoke-test.ps1` (after starting the stack).

## Production & cluster deploy

- [docs/security-hardening-plan.md](docs/security-hardening-plan.md) — attack surface review and hardening backlog.
- [docs/deployment-argocd.md](docs/deployment-argocd.md) — Argo CD / Kubernetes checklist (**no sample inventory**; OIDC-only).
- [deploy/kubernetes/README.md](deploy/kubernetes/README.md) — minimal Kustomize base.

Build the **cluster** frontend image with `NGINX_CONF=nginx.k8s.conf` so `/api` proxies to the `stash-backend` Service (see `infra/Dockerfile.frontend`).

Object storage: production should use a **private** managed S3 bucket. Until that exists, you can point `S3_*` at a **temporary MinIO** (or any S3-compatible API) — see `deploy/kubernetes/examples/` and `docs/deployment-argocd.md`.

## Backups

In the full Con Services stack, Den's operator backs up Stash's database and
bucket every night along with everything else (see
[docs/operations.md](../docs/operations.md)).

**Run standalone, or on a database server Den's operator doesn't manage,
Stash is not backed up by anything. That is the administrator's job:**
schedule a `pg_dump --format=custom` of the Stash database and a copy of its
bucket, encrypt both, and keep them somewhere other than the server.

## Roadmap

- v0.5 — Telegram bot for mobile sign-outs ("walk to logistics with no laptop").
- v0.5 — Per-item file attachments (receipts, manuals, photos) wired to the existing presigned-upload endpoints.
- v0.5 — Notifications when expected returns slip past `expected_return_at`.
