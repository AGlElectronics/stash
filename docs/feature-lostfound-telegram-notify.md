# Possible Feature — Lost & Found Telegram Notifications

**Status:** idea / not scheduled  
**Audience:** front desk + ConOps during a con  
**Related:** `lost_found_reports`, `frontend/src/api/lostfound.ts`, MinIO photos

## Problem

When ConOps finds an item that matches a guest’s **lost** report, the guest only learns about it if they come back to the desk or someone contacts them manually. Front desk already captures `contact_name` and `contact_info`, but there is no outbound notification channel from Stash.

A Telegram bot could let staff send “we may have your item” messages (optionally with a photo) in one click after a human confirms a match.

## What Stash already has

| Existing piece | Relevance |
|---|---|
| `contact_name`, `contact_info` on L&F reports | Guest identity at intake; not enough alone for Telegram |
| `photo_file_id` on found reports | Can attach to outbound message |
| Status workflow (`open` → `returned` / `closed`) | Notify only while pickup is still relevant |
| Audit log + OIDC permissions | Who sent what; gate the action |
| MinIO file storage | Source for `sendPhoto` |

## Telegram constraint (must-read)

A bot **cannot** message arbitrary people. The guest must opt in by **starting the bot** (or otherwise giving Stash a **`chat_id`**).

Storing `@username` in `contact_info` is **not reliable** for programmatic sends. The durable link is `telegram_chat_id` on the report (or a side table keyed by report id).

### Suggested desk flow

```
1. Staff creates LOST report (LF-0042) at front desk
2. Guest scans QR / opens t.me/YourLFBot?start=LF0042
3. Bot records chat_id ↔ report
4. Later: staff matches a FOUND item, opens report, clicks "Notify guest"
5. Backend sends message (+ photo) via Telegram Bot API
```

Still “from Stash directly” — one-time guest opt-in at intake.

## Reasonable scope

### MVP (recommended)

Staff-triggered notify after human match — **not** fully automatic send from description similarity alone.

| Component | Notes |
|---|---|
| **Bot** | Create via @BotFather; `TELEGRAM_BOT_TOKEN` in secrets |
| **Webhook / long poll** | Handle `/start LF0042` (or deep-link payload); persist `telegram_chat_id` |
| **DB** | e.g. `telegram_chat_id`, `telegram_linked_at`, optional `notify_opt_in` on `lost_found_reports` |
| **Service** | `NotifyGuest(ctx, reportID)` → `sendMessage` + optional `sendPhoto` (image from MinIO) |
| **API** | `POST /api/lostfound/{id}/notify-telegram` |
| **Permission** | e.g. `lostfound.notify` in access config |
| **UI** | **Notify guest** on L&F detail when linked + status is `open` + type is `lost` (or staff picks which report to notify) |
| **Audit** | Log actor, report id, timestamp, success/failure |

**Rough effort:** ~2–4 days for someone familiar with the codebase (bot link + button + photo + audit).

### Message template (example)

> Hi! We may have your item (*{item_name}*).  
> Report **{report_number}** — please come to front desk with ID to claim it.  
> [photo if available]

Keep PII minimal; use report number as the join key they already know.

### Later (optional)

- **Match suggestions** — fuzzy compare lost ↔ found descriptions; staff still confirms before send  
- **Auto-send** — only if matches are extremely conservative; wrong “we found your thing” is worse than no message  
- **Guest reply** — “reply YES to confirm pickup” (needs webhook handling)  
- **i18n** — DE/EN templates for the deploying org
- **Disable after con** — purge chat IDs / disable bot for GDPR minimization  

## Not in scope for MVP

- Cold-messaging from phone number or @username without bot `/start`  
- Fully automatic notify with no staff click  
- Replacing front desk pickup verification  

## Ops & policy

- **Consent** at desk: “OK if we Telegram you when we find it?”  
- **Secrets:** bot token in K8s secret / `.env`, never in git  
- **Network:** backend egress to `api.telegram.org` (Cilium policy in prod)  
- **Rate limits:** Telegram API limits; con-scale volume is fine  
- **Feature flag:** e.g. `LOSTFOUND_TELEGRAM_ENABLED=false` until bot is ready  

## Alternatives considered

| Channel | Pros | Cons |
|---|---|---|
| **Telegram** | Free, photos, common at furry cons | Requires chat_id linking |
| Email | Universal | Many guests won’t give email; deliverability |
| SMS (Twilio) | Works on phone | Per-message cost; weak photo story |
| Discord | Some communities use it | Same opt-in / linking problem |

Telegram is a strong effort-vs-delight ratio for on-site lost & found.

## Open questions

1. One org-wide bot or per-event bot name?
2. Link only on **lost** reports, or also on **found** reports (owner looking for their bag)?  
3. Notify from the **lost** report, the **found** report, or a explicit “link these two reports” action?  
4. Retain `telegram_chat_id` after the con or delete on schedule?  
5. Include public claim URL in message (currently env-gated off) or desk-only pickup instructions?  

## Verdict

**Worth doing** as a small follow-up feature: high guest delight, modest backend + UI work, fits existing L&F model. Define MVP as **desk opt-in + staff-confirmed notify**; treat auto-matching as a separate, later experiment.
