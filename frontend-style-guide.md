# Stash frontend style guide

Visual and interaction guidance for the **Stash** inventory module (Con Services / AGl Electronics).

This file used to describe a single-event corporate identity pack. Stash is now a **reusable, brandable** module: deploy-time look-and-feel comes from `frontend/public/brand.json` (standalone) or from Den after pairing. Do not hard-code an event name, logo path, or IdP realm into application code.

## Principles

- Clean structure, strong hierarchy, readable density for warehouse / desk use.
- Brand color and product name come from the brand pack (`BrandProvider`), not scattered literals.
- Prefer Montserrat (display) + Inter (UI) unless a brand pack overrides fonts later.
- Rounded controls, clear focus states, no generic purple SaaS chrome.

## Brand pack

| Field | Purpose |
| --- | --- |
| `productName` | App wordmark / document title |
| `organizationName` | Org label in chrome / exports |
| `logoUrl` | Mark shown on login and shell |
| `colors.primary` / `carbon` / `shadow` | Theme tokens applied via CSS variables |
| `login*` / `publicDeskName` / `claim*` | Copy for auth and public lost-&-found claim |

See `docs/adr/0001-branding-and-den-pairing.md`.

## Barcodes

Generated asset tags use settings `barcode_prefix` + `barcode_width` (default `ST` + 6). UI placeholders should match the configured prefix, not a hard-coded event code.
