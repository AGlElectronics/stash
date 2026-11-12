# Stash

> [!WARNING]
> ## **NOT READY FOR PRODUCTION**
>
> Con Services is under active development. Expect breaking changes, missing
> features and bugs. Do not use it for real events or real data yet.

Stash is the inventory module of [Con Services](https://aglelectronics.github.io/con-services/):
items, locations and departments, sign-outs and returns, packing lists,
purchase records and requests, barcode and QR labels, and the Lost & Found
desk (with an optional Telegram bot for guests). Every change is in the
audit log and in each item's history.

**Documentation:** <https://aglelectronics.github.io/con-services/modules/stash/>

| Page | What it covers |
| --- | --- |
| [Set up Stash](https://aglelectronics.github.io/con-services/modules/stash/setup/) | Kubernetes (Helm chart and Kustomize), Docker Compose |
| [Configuration](https://aglelectronics.github.io/con-services/modules/stash/configuration/) | Every environment variable, `access-config.toml`, the web app's `/config.json` |
| [Admin guide](https://aglelectronics.github.io/con-services/modules/stash/admin-guide/) | Roles, settings, retention, Telegram, emails, before/during/after the event |
| [Volunteer and front-desk guide](https://aglelectronics.github.io/con-services/modules/stash/volunteer-guide/) | Scanning, labels, signing gear out and back in, Lost & Found |
| [API](https://aglelectronics.github.io/con-services/modules/stash/api/) | Every endpoint with `curl` examples |

## Quick start (Docker Compose, dev only)

You need Docker. From this folder:

```sh
cp .env.example .env
docker compose --env-file .env -f infra/docker-compose.yml up --build
```

Open <http://localhost:3000>. The stack runs PostgreSQL (`:5432`), SeaweedFS
as S3 (`:9000`), the backend (`:8080`, migrations run on start) and the web
app (`:3000`). Sign-in is `dev` mode: the first request creates the user
`dev-admin` as an admin. To try another role, set
`localStorage["stash.devUser"]` in the browser to `bob:volunteer` (or
`carol:frontdesk,readonly`) and reload. Dev mode is for your own computer
only; real installations use `AUTH_MODE=oidc`.

Without Docker: Go 1.27.4+, Node 22+, a PostgreSQL and an S3 endpoint, then
`cd backend && sqlc generate && go run ./cmd/server` and
`cd frontend && npm install && npm run dev`.

## Roles

Roles are global (a department lead handles every department's gear) and
defined in [`infra/access-config.toml`](infra/access-config.toml), which the
backend reads at start. The role catalog (`GET /api/role-catalog`) lists each
role as `stash_<role>`; HR assigns them, and a Keycloak group named exactly
`stash_<role>` grants the role without a binding.

| Role | What it is for |
| --- | --- |
| `stash_admin` | Everything, including settings, Den pairing and people |
| `stash_inventory_manager` | Logistics: items, sign-outs, reference data, purchases, requests, exports |
| `stash_dept_lead` | Sign-outs and returns, damage and loss, exports, requests |
| `stash_volunteer` | Look up items, run sign-outs and returns, ask for gear |
| `stash_readonly` | Read items, sign-outs and the audit log |
| `stash_frontdesk` | Lost & Found: log, resolve, notify guests |
| `stash_conops_lead` | All of Lost & Found plus audit and exports |
| `stash_conops_readonly` | Read Lost & Found reports |

Full permission list: [Configuration → access-config.toml](https://aglelectronics.github.io/con-services/modules/stash/configuration/).

## Development

```sh
# Backend (sqlc output is not committed; regenerate after changing queries/ or migrations/)
cd backend
sqlc generate
go vet ./...
STASH_TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/stash_test?sslmode=disable' go test ./...

# Frontend
cd frontend
npm run lint && npm run typecheck && npm test
```

Database tests run when `STASH_TEST_DATABASE_URL` is set; each test gets its
own schema. Optional: `STASH_TEST_S3_ENDPOINT` (+ `_ACCESS_KEY`, `_SECRET_KEY`)
for the S3 client test and `STASH_TEST_SMTP_ADDR` (for example the dev
mailpit at `localhost:1025`) for the email test. `scripts/lint.ps1` runs the
same lint as CI in Docker.

CI runs the shared pipeline of `con-services`
([`.github/workflows/ci.yml`](.github/workflows/ci.yml)).

## Smoke tests and deployment files

These live in the `con-services` repository:

- [`scripts/smoke-stash.ps1`](https://github.com/AGlElectronics/con-services/blob/main/scripts/smoke-stash.ps1) — end-to-end check of a running Stash (roles, items, sign-out lifecycle, files, Lost & Found, exports).
- [`deploy/charts/stash`](https://github.com/AGlElectronics/con-services/tree/main/deploy/charts/stash) — Helm chart.
- [`deploy/kubernetes/modules/stash`](https://github.com/AGlElectronics/con-services/tree/main/deploy/kubernetes/modules/stash) — Kustomize manifests.

## Backups

In the full Con Services stack, Den's operator backs up Stash's database and
bucket every night. Run standalone, Stash is not backed up by anything: see
[Databases and backups](https://aglelectronics.github.io/con-services/operations/databases-and-backups/).

## Changes

See [CHANGELOG.md](CHANGELOG.md).
