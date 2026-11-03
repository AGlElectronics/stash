# Backend Plan — Follow-up to the Frontend Redesign

Companion plan to the Stash frontend redesign. The frontend defines the API contracts below; this document specifies the backend work. User-facing decisions are restated here so this document stands alone.

## Context

- Backend: Go (`backend/` — `cmd`, `internal`, `migrations`, `queries`, sqlc), Postgres, MinIO file storage, Keycloak OIDC.
- Existing API surface used by the frontend: `/api/items`, `/api/signouts`, `/api/departments`, `/api/locations`, `/api/categories`, `/api/audit-log`, `/api/settings`, `/api/me`, `/api/health`, `/api/access/*`, item image upload endpoints.
- The deployment repo (`argo deployments awoocluster/stash`) already mounts a read-only `base/stash-oidc-bindings.yaml` into the backend pod, and `/api/access/bindings` already reports `read_only: true` in that mode. That mechanism is the starting point for the new access model.

## 1. Access model: config file only, no access-control UI or CRUD

User decision: roles, permissions, and OIDC mappings are managed **exclusively in config files**. The frontend deletes all access-control UI and its `src/api/access.ts` module entirely.

- Define a config file format mapping **OIDC role/group claims → Stash permissions** (optionally department-scoped, preserving today's department-scoped role semantics). The existing mounted-YAML bindings file is the model; extend it to be the only source of truth.
- `/api/me` remains the single access endpoint the frontend consumes: it must return effective `permissions: string[]` (and memberships/`primary_department_id`) computed from the config file + the user's token claims.
- **Remove** the DB-backed role assignment paths: group-binding CRUD endpoints, per-user role override endpoints, `GET /api/access/*` (roles, permissions, role-permissions, bindings, users-with-roles). Drop or archive the backing tables (`user_roles` / binding tables) via migration.
- Add new permissions to the catalog: `lostfound.view`, `lostfound.create`, `lostfound.resolve` (named to match the existing `item.view` / `signout.view` convention; the frontend's `PERMS` map in `frontend/src/hooks/useMe.ts` already uses these exact strings).
- Config reload behavior: at minimum on pod restart (current behavior); hot-reload optional.
- Update the deployment repo: extend `base/stash-oidc-bindings.yaml` (or replacement config) with the role→permission mapping, including front desk / ConOps roles for Lost & Found.

## 2. Lost & Found API (new)

Replaces the ConOps AppSheet/Google Sheets app. The frontend is already built against this contract (`frontend/src/api/lostfound.ts`); implement it exactly or coordinate changes with the frontend.

### Data model — `lost_found_reports`

| Field | Notes |
|---|---|
| `id` | UUID |
| `report_number` | Human-friendly unique sequence, e.g. `LF-0042` (like `signout_number`) |
| `type` | `lost` \| `found` |
| `status` | `open` \| `returned` \| `closed` — `returned` = handed back to owner; `closed` = unclaimed/disposed/donated at end of con. New reports default to `open` |
| `item_name`, `description` | Text |
| `photo_file_id` | Optional; reuse the existing MinIO file/image upload pattern from items |
| `location` | Free text ("left near gym") — intentionally NOT a Stash `locations` FK |
| `date_reported` | Defaults to now |
| `reported_by_user_id` | Staff user from OIDC session, auto-captured |
| `contact_name`, `contact_info` | Optional — the guest the report is logged on behalf of (front desk enters reports for attendees) |
| `returned_to` | Name of the person who claimed the item (set when status → `returned`) |
| `resolved_by_user_id`, `resolved_at`, `resolution_note` | Resolution metadata |
| `created_at`, `updated_at` | Timestamps |

### Endpoints

- `GET /api/lostfound` — list; filters: `type`, `status`, `q` (matches report_number/name/description/location); paginated `{ items, total }` like other lists.
- `POST /api/lostfound` — create (requires `lostfound.create`).
- `GET /api/lostfound/:id` — detail (requires `lostfound.view`).
- `PATCH /api/lostfound/:id` — edit fields and status transitions, including reopen (`returned`/`closed` → `open`); status changes require `lostfound.resolve`.
- Photo upload: reuse the existing item-image upload endpoints/pattern (or a generic file endpoint) so the frontend can attach `photo_file_id`.

### Behavior

- All mutations recorded in the audit log (who did what when — replaces the AppSheet USEREMAIL logfile).
- Permission enforcement server-side: users without `lostfound.view` get 403; the frontend hides the section but enforcement lives here.
- Export: end-of-con datasheet is generated client-side as CSV from the list endpoint; no dedicated export endpoint needed (a server-side CSV export is optional future work).

## 3. Dashboard aggregate endpoint (optional but recommended)

The redesigned dashboard currently derives everything client-side, including fetching items with `limit: 1000` to compute stats. Add `GET /api/dashboard` returning per-viewer aggregates so the frontend can drop the bulk fetches:

- Overdue sign-outs count (status `signed_out`/`partially_returned` with `expected_return_at` past due) + recent active sign-outs.
- Partially returned count.
- Low stock count (`quantity_available < minimum_quantity`), scoped by department where relevant.
- Damaged items count (repair queue).
- Open Lost & Found report count (only if viewer has `lostfound.view`).

Coordinate the response shape with the frontend dashboard once it lands; until then the client-side derivation keeps working.

## 4. Cleanup

- Remove server code for the deleted access endpoints and their sqlc queries.
- Migration plan for dropping binding/override tables (after confirming the config file covers all production mappings currently in the DB/YAML).
- Keep `/api/settings` (`force_barcode_on_signout`) and `/api/health` unchanged — the frontend moves health display into Settings but the endpoint stays as-is.

## Decisions log (from the frontend planning session)

- Access control: config files only, zero UI (not even read-only views).
- Lost & Found visibility: the entire section is visible only to roles granted `lostfound.*` permissions in the config (front desk/ConOps); other users never see it.
- Lost & Found locations are free text, not Stash location records.
- Statuses `open`/`returned`/`closed` chosen over the AppSheet's flat "Resolved" to satisfy the front desk lead's lost/found/returned classification and end-of-con accounting.
- No mock data in the frontend; until these endpoints exist the Lost & Found pages show graceful empty/error states.
