# Changelog

All notable changes to Stash are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [1.0.0] - 2026-11-12

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
  Keycloak group with that name grants the role without a binding.
  `GET /api/me` returns `roles` (catalog keys) and `permissions`.
- `GET /api/role-catalog` follows the role catalog contract 1.0: HR reads
  it with `HR_SERVICE_TOKEN` as a Bearer token (32 characters or more;
  staff can read it signed in), and it has `revision` with an `ETag`
  (`If-None-Match` answers `304`), `group_display`, `dynamic` and
  `privileged: true` for roles that can manage access or people, or export
  data (`admin`, `inventory_manager`, `dept_lead`, `conops_lead`). It is rate limited (`429 rate_limited` with `Retry-After`).
- Group matching settings: `OIDC_GROUPS_CLAIM` (default `groups`),
  `OIDC_GROUP_MATCH` (`full` or `last`) and `OIDC_GROUP_PREFIX` (a parent
  group such as `/con2026` removed before matching). Names are compared
  case-insensitively, ignoring one leading `/`.
- Stash uses the shared Con Services code (`groupmatch`, `denlink`,
  `authbase`, copied from `con-services` `shared/go` with
  `scripts/sync-shared`). See
  [Shared code](https://aglelectronics.github.io/con-services/concepts/shared-code/).
- When Den rejects Stash's credential (Stash was removed in Den, or the
  credential revoked), Stash marks the link **revoked** and keeps it so
  across restarts. `GET /api/den/state` and `GET /api/den/` report
  `"revoked": true`, `POST /api/den/sync` answers
  `409 den_credential_revoked`, and admins see "Den rejected this module's
  credential" with a form to pair again. Stash never pairs again by
  itself; pairing again clears the state. The migration
  `20261112000001_den_link_revoked` adds the column.
- Pairing URLs may only point at Den's host: `DEN_INTERNAL_URL` (set on
  Kubernetes) or the hosts in `DEN_ALLOWED_HOSTS`, over `https://` outside
  development.
- QR labels: the item label page can print a QR code instead of a barcode.
  It holds a link to `/scan?code=<code>`, so any phone camera opens the
  item in Stash; the in-app scanner (camera or USB) reads it too. Labels
  show the organisation logo again (as a grayscale image; turn it off per
  print).
- `/scan?code=<code>` opens the item or Lost & Found report with that code.
- Import & export page. `POST /api/items/import` (CSV as the body or the
  `file` form field, up to 5 MiB and 5,000 rows) creates items from rows
  without an `id` and changes the items named by `id`; empty cells leave a
  field unchanged, a different `quantity_owned` is recorded as an
  adjustment. `?dry_run=true` checks every row and saves nothing; a real
  import saves all rows or none (`422` with a per-row report). The items
  export can be imported again as it is.
- Ready-made exports at `GET /api/export/reports/{report}.csv`: `low-stock`,
  `damaged`, `lost`, `expiring` (within 60 days), `retired` and
  `overdue-signouts` (`GET /api/export/reports` lists them). They need
  `export.create` and are audited as `export.report`.
- Purchase records: Item page → Purchases lists what was bought, when, from
  whom and for how much (`GET`/`POST /api/items/{id}/purchases`); a
  purchase can add its quantity to stock. Recording needs `item.adjust`.
- Item requests (`/requests`, `/api/requests`): anyone with the new
  `request.create` (volunteer, dept_lead, inventory_manager, admin) asks for
  gear; people with the new `request.manage` (inventory_manager, admin)
  approve, order, fulfil or reject it, and the requester can cancel their
  own open request. Every step is audited.
- Event packing lists (`/packing-lists`, `/api/packing-lists`): list what
  goes to an event or department, track how much is packed against what is
  available, and turn the list into a draft sign-out in one step. Viewing
  needs `signout.view`, changing `signout.create`.
- Item page → Documents: attach, download and delete receipts, invoices,
  manuals, warranties and other files (up to 50 MB each).
- Settings → Privacy and notifications: Lost & Found retention, the last
  day of the convention, anonymising inactive staff, the Telegram bot
  switch, overdue-return emails and the SMTP server. While Stash is paired
  with Den, Den's module settings (Den → Admin → Module settings) override
  the values here and are shown as "Managed in Den": `stash.lost_found`
  (`kept_after_con` with `days`, default 30, or `deleted_after_closing`
  with `hours`, default 48), `retention.inactive_users_days` and the SMTP
  server (`smtp.host`, `port`, `username`, `security`, `from`,
  `from_name`). The last day of the convention stays a Stash setting.
- `GET /api/export/lostfound.csv`: the Lost & Found datasheet, built on the
  server. It needs `export.create` and `lostfound.view`; the button on the
  Lost & Found page now shows for people with `export.create`.
- The audit log records settings changes (`settings.update`), Den
  pair/sync/unpair (`den.pair`, `den.sync`, `den.unpair`) and every CSV
  export (`export.items`, `export.signouts`, `export.lostfound`). Its
  entity filter now offers `lostfound`, `file`, `settings`, `den` and
  `export`.
- Lost & Found retention: once an hour Stash deletes reports, their photos
  and their Telegram link when retention has passed, either
  `lostfound_retention_hours` (default 720) after a report was returned,
  closed or removed, or `lostfound_retention_days` (default 30) after
  `con_end_date` for every report. Earlier audit entries about a deleted
  report keep the action but lose the guest's data; the deletion is
  audited as `lostfound.purge`.
- People (`/users`, `GET /api/users`, `POST /api/users/{id}/anonymise`,
  permission `user.manage`): anonymise someone who left. Their name, email
  and IdP subject disappear from Stash, the audit log and item history
  (shown as "Former user"). With `user_inactive_days` set, people who have
  not signed in for that long are anonymised automatically.
- Lost & Found Telegram bot (Stash's own): set `TELEGRAM_BOT_TOKEN` and
  turn it on in Settings. A guest sends `/start LF0042` (or the report
  number) to link the report, `/stop` to unlink. On a linked report,
  people with the new `lostfound.notify` permission (`frontdesk`,
  `conops_lead`, `admin`) can press **Notify guest**
  (`POST /api/lostfound/{id}/notify`). Every link, unlink and message is
  audited; the chat id never appears in the API or the audit log, and it
  is deleted with the report. `GET /api/lostfound/telegram` reports
  whether the bot is on and its @username.
- Overdue returns: the sidebar shows overdue sign-outs
  (`GET /api/notifications`), and with `overdue_email_enabled` Stash emails
  the receiver and the `overdue_notify_emails` list at most once a day
  per sign-out (audited as `signout.overdue_reminder`). The SMTP server
  comes from Settings or Den; the password is the new `SMTP_PASSWORD`
  secret.
- Background jobs run on exactly one replica, elected with a PostgreSQL
  advisory lock; another replica takes over within 30 seconds.
  `JOBS_ENABLED=false` turns them off on a replica.
- Settings: protecting report LF-0001 from edits and removal is an easter
  egg that stays off unless an admin turns it on.
- Dev sign-in (`AUTH_MODE=dev`) can use roles other than admin:
  `X-Dev-User: bob:volunteer` or `bob:frontdesk,readonly`. A plain name is
  still an admin.
- `DEN_MODE=required` keeps the module locked until it is paired with Den:
  the API answers `503 den_pairing_required` (except health, brand, sign-in
  and the Den connection routes) and the public `GET /api/den/state`
  reports `{"mode","paired","revoked"}`. The default `standalone` runs without Den;
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
- Helm chart: `networkPolicy.telegram` (on by default) lets the backend
  reach `api.telegram.org` on 443 for the Lost & Found bot, and
  `networkPolicy.smtp` (off by default; set `fqdns` or `cidrs`) lets it
  reach your SMTP server on 587 and 465 for overdue emails. With
  `networkPolicy.gateway.cilium: true` both become `toFQDNs` rules in the
  CiliumNetworkPolicy `stash-backend-external`; otherwise `ipBlock` rules.
  `existingSecret` may also hold the optional keys `TELEGRAM_BOT_TOKEN`
  and `SMTP_PASSWORD`. See
  [Network policies](https://aglelectronics.github.io/con-services/operations/network-policies/).
- Development: `infra/docker-compose.yml` with SeaweedFS as the S3 server
  (`localhost:9000`, bucket `stash-data-dev`), and
  `infra/frontend-config/compose.json` (dev sign-in) mounted as the web
  app's `/config.json`; set `FRONTEND_CONFIG` in `.env` to use another
  file. Building from source needs Go 1.27.3 or newer.
- CI on every pull request (tests, lint, Trivy scans) and signed images with
  an SBOM published to GHCR on version tags (linux/amd64; linux/arm64
  optional).

### Changed

- The Helm chart admits Den's backend (`networkPolicy.den`, on by default) so Den can read the role catalog and portal to-dos. Kustomize already did.
- `.env.example` lists `SHUTDOWN_TIMEOUT` (default 15s).
- CI passes `den-major: "1"`. A release whose MAJOR is not 1 is refused once the shared `ci-v1` tag points at the workflow that has that input.
- Standalone Compose uses PostgreSQL `postgres:16.15-alpine`.
- The Helm chart description says CI renders and lints the chart; the first production install is still pending.
These matter if you ran a pre-release build of Stash.

- `/silent-callback.html` may be framed by the Stash web app itself, so a
  reload can sign in again through the existing Keycloak session. Every
  other page still sends `frame-ancestors 'none'` and `X-Frame-Options:
  DENY`. `EXTRA_FRAME_SRC` stays empty. Both header snippets keep
  `camera=(self)` for barcode scanning.
- Overdue reminder emails use Den's SMTP password when Den sets
  `smtp.host` and `smtp.password_set`. Stash fetches it when it sends and
  does not store it. `SMTP_PASSWORD` is used only when Den does not set a
  mail server. When Den sets the server and holds no password, the message
  is sent with no password.
- The shared `denlink` copy can fetch a module-setting secret from Den,
  list the organisation's paired modules, and check a call Den makes into
  Stash. Den errors in the log now start with a lowercase letter. The
  sentence on the Den connection page is unchanged.
- **BREAKING:** the backend accepts only **access tokens** whose audience
  (`aud`) contains `OIDC_AUDIENCE`, now `stash-api` by default; ID tokens
  are refused. The web app sends the access token. Before upgrading, add
  the audience mapper `stash-api` to the `stash-spa` client in Keycloak
  (new realms from `con-services` have it), and set
  `OIDC_AUDIENCE=stash-api` or remove the setting. See
  [Audiences per API](https://aglelectronics.github.io/con-services/concepts/authentication/#audiences-per-api).
- **BREAKING:** groups come only from the claim named by
  `OIDC_GROUPS_CLAIM`; Keycloak `roles` are no longer merged in. Give
  people their Stash roles through groups (`stash_<role>` or a binding in
  `access-config.toml`).
- **BREAKING:** `AUTH_MODE` now defaults to `oidc` and `APP_ENV` to
  `production`, and `AUTH_MODE=dev` is refused unless
  `APP_ENV=development`. A development setup that relied on the old
  defaults sets both explicitly (`.env.example` and the Compose files do).
- **BREAKING:** the stored Den credential is encrypted (AES-256-GCM) with
  `DEN_LINK_ENCRYPTION_KEY` (32 random bytes, base64; one key per
  installation). Outside `APP_ENV=development` the backend refuses to start
  without it. Add it to `stash-secrets` before upgrading; an existing
  plaintext credential is encrypted on its first read, so no re-pairing is
  needed. Rotate with `DEN_LINK_ENCRYPTION_KEY_PREVIOUS`. See
  [Secrets](https://aglelectronics.github.io/con-services/operations/secrets/#den-link-encryption-key).
- **BREAKING:** `GET /api/role-catalog` needs `HR_SERVICE_TOKEN` as a
  Bearer token or a staff sign-in (it was public). Give HR the same
  `HR_SERVICE_TOKEN`. `scope` is now `organization`.
- **BREAKING:** Den's organisation settings `stash_con_end_date`,
  `stash_lostfound_retention_*` and `stash_smtp_*` are no longer read.
  Set the same values in Den → Admin → Module settings (Lost & Found
  retention, inactive users, SMTP), and the last day of the convention in
  Stash → Settings.
- The web app keeps sign-in tokens in memory only, no longer in
  `localStorage`. After a reload it signs in again silently through the
  Keycloak session; a sign-out or another person signing in in another
  tab still reaches every tab through the session monitor.
- Request bodies are limited to 1 MiB (settings to 64 KiB; uploads keep
  their own limit); a larger one is refused with `413 request_too_large`.
- API responses carry `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY` and
  `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`,
  and CORS exposes `X-Request-ID`. Outside dev sign-in the backend drops
  `X-Dev-User` and `X-Dev-Groups` from every request, and CORS allows them
  only in dev sign-in.
- The public Lost & Found lookup is rate limited per client address
  (`429 rate_limited` with `Retry-After`); the client address comes from
  `X-Forwarded-For` only behind a proxy in `TRUSTED_PROXY_CIDRS`.
- The web app's nginx sends a Content Security Policy and the other browser
  security headers from `con-services` `shared/nginx` (not on `/api/`),
  with `camera=(self)` for barcode scanning. Set `KEYCLOAK_ORIGIN` on the
  web app container to the sign-in server (for example
  `https://auth.con.example.org`), or the browser blocks sign-in;
  `DEN_ORIGIN`, `S3_ORIGIN`, `EXTRA_CONNECT_SRC` and `EXTRA_FRAME_SRC` are
  optional (`EXTRA_FRAME_SRC` stays empty). The inline scripts of
  `index.html` and `silent-callback.html` moved into `theme-init.js` and
  `silent-callback.js`. See
  [Nginx security headers](https://aglelectronics.github.io/con-services/concepts/shared-code/#nginx-security-headers).
- The Helm chart in `con-services` (`deploy/charts/stash`, still `1.0.0`)
  and the Kustomize manifests set `OIDC_AUDIENCE=stash-api` and the
  group-matching variables, and name `DEN_LINK_ENCRYPTION_KEY` on its own
  so a Secret without it stops the pod (optional only while `APP_ENV` is
  `development`). The web app gets `KEYCLOAK_ORIGIN` (empty
  `frontend.securityHeaders.keycloakOrigin` means the sign-in address's
  scheme and host), `DEN_ORIGIN`, `S3_ORIGIN` and `EXTRA_CONNECT_SRC`.
  `EXTRA_FRAME_SRC` stays empty. The chart and the Kustomize nginx config
  allow framing only of `/silent-callback.html`, by Stash itself.

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

### Removed

- The GitLab pipeline (`.gitlab-ci.yml`); CI runs on GitHub with the shared
  `con-services` pipeline.
- In-repo planning documents (Argo CD notes, security plan, backend plan,
  Telegram proposal, original project plan). What is still true is on the
  documentation site.
- Offline mode is no longer on the roadmap: the service worker only caches
  the app itself.

### Fixed

- The web app image's fallback security-header files, used when
  `/etc/nginx/snippets` cannot be written, now strip `${S3_ORIGIN}` as
  well. A digit in that name used to leave the token in the policy, and
  nginx then refused to start.
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
