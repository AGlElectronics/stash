# ADR 0001 — Branding source and Den pairing

## Status

Accepted (clone / ecosystem workstream)

## Context

Stash must run standalone with local branding, and later mate to Den with only a
base URL + pairing token. When Den-backed, org styling is set once in Den;
modules pull metadata and media through Den’s API. Each module owns one S3
bucket; modules never read another module’s bucket. MinIO is local/dev only and
must never be used in production.

## Decision

1. Frontend loads `/brand.json` for `source=standalone` via `BrandProvider`.
2. After pairing, `source=den`: fetch branding JSON from Den with the module
   bearer credential; fetch logo/media via Den media endpoints (Den reads its
   own bucket).
3. `SystemSettings` holds server-side defaults (owner, currency, barcode format,
   sacred LF gate) separately from visual brand packs.
4. Production object storage is real S3-compatible; Compose MinIO is dev-only.

## Consequences

- Org-specific strings move into `public/brand.json` / theme overlay.
- Pairing UI and Den media proxy land with the Den scaffold; Stash already has
  the brand source switch hook (`setBrandSource`).
