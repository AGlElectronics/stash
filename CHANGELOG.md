# Changelog

All notable changes to Stash are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

Needs Den: 1.0.0 or later (Den MAJOR 1).

First release, `1.0.0`. Released versions are tagged `vMAJOR.MINOR.PATCH`;
the images `ghcr.io/aglelectronics/stash-backend` and `stash-frontend` are
published as `MAJOR.MINOR.PATCH-BUILD`, where `BUILD` counts rebuilds of
the same version. Rebuilds get no entry here.

### Added

- Inventory: items with a barcode, a location and a managing department,
  barcode labels, moves between locations, and damage and loss records.
- Sign-outs and returns: which department took how many, when, and when it
  should be back; returns record what came back, was used up, lost or
  damaged. A sign-out can be created and confirmed in one step, with
  optional barcode scanning.
- Lost & Found desk: lost reports and handed-in finds with photos and
  report numbers, matching and hand-back, and an optional public claim
  page for guests (`PUBLIC_LOSTFOUND_CLAIM_ENABLED`).
- Roles and permissions from `access-config.toml`, an audit log, a
  per-item history and CSV exports. Roles are global: a department lead
  handles every department's gear. `GET /api/role-catalog` lists each
  role once as `stash_<role>` (for example `stash_volunteer`), and a
  Keycloak group with exactly that name grants the role without a
  binding. `GET /api/me` returns `roles` (catalog keys) and `permissions`.
- Settings → Privacy and notifications: Lost & Found retention, the last
  day of the convention, anonymising inactive staff, the Telegram bot
  switch, overdue-return emails and the SMTP server. While Stash is paired
  with Den, Den's organisation settings `stash_con_end_date`,
  `stash_lostfound_retention_mode`, `stash_lostfound_retention_days`,
  `stash_lostfound_retention_hours`, `stash_smtp_host`, `stash_smtp_port`,
  `stash_smtp_username`, `stash_smtp_from` and `stash_smtp_tls` override
  the values here and are shown as "Managed in Den".
- `GET /api/export/lostfound.csv`: the Lost & Found datasheet, built on the
  server. It needs `export.create` and `lostfound.view`; the button on the
  Lost & Found page now shows for people with `export.create`.
- The audit log records settings changes (`settings.update`), Den
  pair/sync/unpair (`den.pair`, `den.sync`, `den.unpair`) and every CSV
  export (`export.items`, `export.signouts`, `export.lostfound`). Its
  entity filter now offers `lostfound`, `file`, `settings`, `den` and
  `export`.
- Settings: protecting report LF-0001 from edits and removal is an easter
  egg that stays off unless an admin turns it on.
- Dev sign-in (`AUTH_MODE=dev`) can use roles other than admin:
  `X-Dev-User: bob:volunteer` or `bob:frontdesk,readonly`. A plain name is
  still an admin.
- `DEN_MODE=required` keeps the module locked until it is paired with Den:
  the API answers `503 den_pairing_required` (except health, brand, sign-in
  and the Den connection routes) and the public `GET /api/den/state`
  reports `{"mode","paired"}`. The default `standalone` runs without Den;
  any other value stops the backend at startup.
- The web app shows a "Connect this module to Den" page (admins) or
  "Waiting for Den" (everyone else) while the backend runs with
  `DEN_MODE=required` and is not paired yet.
- `TRUSTED_PROXY_CIDRS` (optional): comma-separated CIDRs of the proxies in
  front of the backend whose `X-Forwarded-For` entries are believed. The
  default, `10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.0/8,::1/128,fc00::/7`,
  covers the Gateway, nginx and Caddy; change it only for a proxy outside
  the private ranges, or set `none` when no proxy is in front. See
  [Client IP addresses](https://aglelectronics.github.io/con-services/operations/production/#client-ip-addresses).
- The backend can serve HTTPS. Set `TLS_CERT_FILE` and `TLS_KEY_FILE` to a
  PEM certificate and private key; the backend re-reads both every minute,
  so a certificate renewed by cert-manager is used without a restart. With
  neither set it serves plain HTTP, which is what Docker Compose
  development uses. See
  [Internal TLS](https://aglelectronics.github.io/con-services/operations/internal-tls/).
- The web app reads its installation settings (sign-in mode, issuer, client
  id, scopes, API address and whether the guest claim page is shown) from
  `/config.json` when it starts, so the published `stash-frontend` image
  works for every installation. nginx serves the file from
  `/etc/conservices/config.json` with `Cache-Control: no-store`, so a change
  shows on the next page load. Without a mounted file the web app shows
  "Not configured yet"; a broken file gives a clear error page. See
  [Web app settings](https://aglelectronics.github.io/con-services/modules/stash/configuration/#web-app-settings-configjson).
- Kubernetes: Helm chart `deploy/charts/stash` `1.0.0` in `con-services`,
  released with the tag `stash/v1.0.0`, and Kustomize manifests in
  `deploy/kubernetes/modules/stash`. Internal TLS is on by default:
  cert-manager issues `stash-backend-tls` and `stash-frontend-tls`, the
  backend and the web app serve HTTPS inside the cluster (web app Service
  on port 443, backend on 8080, both named `https`), and every in-cluster
  address uses `https://`. The cluster needs cert-manager, trust-manager
  and the platform's `conservices-internal-ca` issuer; without them set
  `internalTLS.enabled: false`. See
  [Set up Stash](https://aglelectronics.github.io/con-services/modules/stash/setup/).
- Development: `infra/docker-compose.yml` with SeaweedFS as the S3 server
  (`localhost:9000`, bucket `stash-data-dev`), and
  `infra/frontend-config/compose.json` (dev sign-in) mounted as the web
  app's `/config.json`; set `FRONTEND_CONFIG` in `.env` to use another
  file. Building from source needs Go 1.27.3 or newer.
- CI on every pull request (tests, lint, Trivy scans) and signed images with
  an SBOM published to GHCR on version tags (linux/amd64; linux/arm64
  optional).

### Security

- Only people with `department.manage`, `category.manage` or
  `location.manage` can create departments, categories and locations,
  also by typing a new name in the item form. People who can only create
  or edit items pick from the existing entries.
- A `department_id` on a binding in `access-config.toml` stops the backend
  at startup with "roles are global, remove the line".
- Files: `POST /api/files` only accepts a `storage_key` inside the item's
  or report's own folder (the key `presign-upload` returned), and deleting
  a file only removes objects from that folder. Files can only be attached
  to an item or Lost & Found report that exists.
- Files: downloads are sent with `X-Content-Type-Options: nosniff` and a
  sandbox `Content-Security-Policy`. Only PNG, JPEG, GIF, WebP, AVIF and
  BMP images are shown inline; everything else (PDF, SVG, HTML, …) is a
  download. Image uploads are checked by their content, not their name.
- Uploads larger than 51 MiB are refused with `413 payload_too_large`
  before they are read.
- A Lost & Found report's photo must be a photo uploaded to that report.
- `PATCH /api/settings` rejects unknown keys and invalid values with
  `422 validation_failed` and a message per key in `details`.
- The audit log hides guest names, contact details, notes and photo
  filenames from Lost & Found entries for people without
  `lostfound.view`. `limit` is capped at 500 and a negative `offset`
  counts as 0.
- CSV exports (server and browser) put an apostrophe in front of text
  starting with `=`, `+`, `-`, `@`, a tab or a carriage return, so a
  spreadsheet does not run it as a formula.
- The public claim page (`/api/public/lostfound`) only shows the report
  number, item name and the day it was reported. Description, location
  and photo are no longer public (the photo route is gone), and search
  matches item names only.

### Fixed

- `GET /api/audit-log` returns `before` and `after` as JSON objects
  instead of base64 strings.
- The backend reads the client address from `X-Forwarded-For` only when
  the connection comes from a trusted proxy (`TRUSTED_PROXY_CIDRS`), and
  ignores `X-Real-IP` and `True-Client-IP`.
- On Kubernetes the database URL Den shows ends in
  `?sslmode=verify-full&sslrootcert=/etc/conservices-internal-ca/ca.crt`;
  the platform's PostgreSQL refuses connections without TLS.
- No image runs as root. The web app runs on
  `nginxinc/nginx-unprivileged:1.30.6-alpine` as UID 101 and listens on
  port 8080. All images are built with Go 1.27.3 and pass the CI image
  scan (Trivy, HIGH and CRITICAL issues that have a fix).
- A production build never falls back to dev sign-in: a `/config.json`
  without `authMode` means OIDC.
