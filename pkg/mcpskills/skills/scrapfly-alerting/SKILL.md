---
name: scrapfly-alerting
description: >-
  Threshold-based alerting on Scrapfly account metrics. Wire up rules like
  "alert me when scrape success rate drops below 95% for 10 minutes" and
  receive notifications via email, webhook, or in-app. Use when the user
  asks to "set up an alert", "monitor my scrape success rate", "tell me
  when my scrapes start failing", "alert on unblocker block rate", "alert
  on ASP block rate", "watch a specific domain", or any task involving the
  Scrapfly alerting product.
  Covers the three call paths: REST API, MCP `alert_*` tools, and the
  `scrapfly alert` CLI subcommand. Open beta, available to all accounts.
---

# Scrapfly Alerting

Scrapfly Alerting lets customers attach **threshold rules to project
metrics** (success rate, block rate, error count, cost, latency, ...) and
receive notifications when a rule sustains a breach over a configurable
window. Three call paths share the same backend: REST API on
`api.scrapfly.io`, MCP `alert_*` tools, and the `scrapfly alert` CLI
subcommand. Pick whichever fits the caller — the same payload shapes,
same state machine, same notification channels.

This skill is meant to be read by an AI agent that has been asked to set
up an alert or manage existing alerts on a Scrapfly account. The golden
rule is: **never invent a threshold**. Always ground the choice in real
account behavior, then validate against history before committing.

## When to use

- The user says "alert me if X drops" / "tell me when Y goes above Z"
- The user wants to monitor a specific domain, project, or product
- The user wants to inspect existing alerts (`list`, `get`, snooze a
  noisy alert, unsnooze after a maintenance window, etc.)
- A monitoring report wants to know which alerts are firing right now
  (`count-active`)

## Prerequisites

- Alerting is in open beta and available to all accounts.
- A valid Scrapfly API key (`scp-live-...`). The MCP and CLI surfaces
  act in the project of the api-key, the only project a key reaches
  alerts in: an alert of another project is listed, created or changed
  with that project's API key, never by passing its `--project-uuid` /
  `project_uuid`.

## The golden workflow

This is the same flow used by all three call paths. Run it in order;
each step grounds the next.

1. **Discover account baseline** — fetch the current per-product
   aggregates so you know what "normal" looks like.
2. **(Optional) Discover per-target baseline** — when the user named a
   domain, fetch its breakdown.
3. **List metric families** — the Alerting registry tells you the legal
   `metric_id` values and their `allowed_dimensions`. Never invent a
   metric ID; always pick from the registry. Unblocker errors are
   `scrape.unblocker_errors_per_min`. An existing alert may still carry a
   metric ID the listing no longer offers — `scrape.asp_errors_per_min` is
   the pre-rename ID of that same metric. Those alerts keep working and
   keep their ID; do not try to "fix" them, and do not reuse a
   superseded ID for a new alert.
4. **Preview the threshold against history** — replay the unsaved rule
   over the lookback window. The preview returns "would-have-fired"
   markers without persisting anything. Aim for **0–2 fires over 24h**:
   silent = rule too lax, more than ~2 = rule too noisy.
5. **Create the alert** — two-step: first call returns a dry-run
   payload so the user can confirm, then re-call with `confirm=true`
   (or the CLI's interactive confirm) to commit.

Concrete worked example: "alert me when my scrape success rate drops
noticeably below current baseline" → discover baseline 65.93% → choose
threshold 55% (10pt below normal) → preview shows `fired_count: 0` over
last 24h → create with email channel. The agent never guessed; every
number came from real data.

## Path A — MCP tools

When the agent is connected to the Scrapfly MCP server, ten alert tools
plus two monitoring tools cover the full surface. Read tools have no
side-effect; write tools require `confirm=true` to commit.

| Tool | Mode | Purpose |
|------|------|---------|
| `monitoring_get_metrics` | read | Per-product aggregates: pick `product` (scrape, screenshot, extraction, crawl, browser), `period` (`last5m`, `last1h`, `last24h`, `last7d`, `subscription`), and optionally `aggregation` (`account`, `project`, `target`). |
| `monitoring_get_target_metrics` | read | Per-domain breakdown for one site. `domain` is required; `group_subdomain` rolls subdomains together. |
| `alert_metric_families` | read | Discover legal `metric_id` values + their `allowed_dimensions`. Call before `alert_create`. |
| `alert_list` | read | List the alerts of the api-key's project with optional filters (`state`, `metric_id`). |
| `alert_get` | read | Full alert row including the per-alert HMAC signing key for webhook channels. |
| `alert_preview` | read | Replay an unsaved rule against history. Use to validate the threshold before `alert_create`. |
| `alert_create` | write (confirm) | Persist a new alert in the api-key's project. Leave `project_uuid` empty. |
| `alert_update` | write (confirm) | Patch fields. A rule-changing edit auto-snoozes for `sustained_minutes`; cosmetic, channel and lifecycle edits do not. |
| `alert_delete` | write (confirm) | Permanent. |
| `alert_snooze` | write (confirm) | Mute notifications. Pass `minutes:N` OR `until_resolved:true`. |
| `alert_unsnooze` | write (confirm) | Clear active snooze. |
| `alert_test` | write (confirm) | Fire a synthetic notification on every configured channel without touching state. |

MCP example (the data-driven workflow from above, expressed as tool
calls):

```jsonc
// Step 1 — baseline
{"tool":"monitoring_get_metrics","parameters":{"product":"scrape","period":"last7d","aggregation":["account"]}}

// Step 2 — discover the registry
{"tool":"alert_metric_families","parameters":{}}

// Step 3 — preview the chosen threshold against last-24h history
{"tool":"alert_preview","parameters":{
  "metric_id":"scrape.success_rate",
  "comparator":"lt","threshold":55,
  "sustained_minutes":5,"range_minutes":1440
}}

// Step 4 — dry-run create (no confirm)
{"tool":"alert_create","parameters":{
  "name":"Scrape Success Rate Low",
  "metric_id":"scrape.success_rate",
  "comparator":"lt","threshold":55,
  "sustained_minutes":5,
  "notify_channels":[{"kind":"email","target":"ops@example.com"}]
}}

// Step 5 — commit
{"tool":"alert_create","parameters":{ ...same..., "confirm":true }}
```

### Critical pitfalls

- **`info_account.account.account_id` is NOT the project UUID.** Do not
  pass it as `project_uuid` — that returns
  `ERR::ALERT::PROJECT_NOT_FOUND`. Leave `project_uuid` and
  `project_name` empty: the server uses the api-key's project, the only
  one the key reaches. For an alert of another project, use that
  project's API key.
- **Always preview before creating.** `alert_preview` uses the same
  state machine the live evaluator runs, so the historical
  count is exactly what production would have produced under the same
  rule. A rule that fired 15 times in the last 24h will spam the
  customer.
- **Two-step create is intentional.** The first call returns
  `DRY RUN — POST /alert would be invoked with: { ... }` so the user
  can review before paying notification cost.

## Path B — CLI

The `scrapfly` CLI exposes the same surface as `scrapfly alert ...`
subcommands. Output is the standard `{success, product, data|error}`
JSON envelope so you can pipe it through `jq`.

```bash
# Discover what's available
scrapfly alert metric-families

# Tune the threshold against history
scrapfly alert preview \
  --metric scrape.success_rate \
  --comparator lt \
  --threshold 95 \
  --sustained-minutes 10 \
  --range-minutes 1440

# Create the alert (channels are repeatable)
scrapfly alert create \
  --name "Scrape Success Rate Low" \
  --metric scrape.success_rate \
  --comparator lt \
  --threshold 95 \
  --sustained-minutes 10 \
  --channel email:ops@example.com \
  --channel webhook:https://httpbin.dev/post

# Or from a version-controlled file (IaC-style)
scrapfly alert create --file ./alerts/success_rate.json

# Manage existing alerts
scrapfly alert list --state triggered
scrapfly alert get <alert-uuid>
scrapfly alert series <alert-uuid> --range-minutes 1440
scrapfly alert update <alert-uuid> --threshold 90
scrapfly alert snooze <alert-uuid> --minutes 30
scrapfly alert snooze <alert-uuid> --until-resolved
scrapfly alert unsnooze <alert-uuid>
scrapfly alert test <alert-uuid>
scrapfly alert delete <alert-uuid>
```

Full flag reference: `scrapfly alert create --help` (or any subcommand
plus `--help`). The flags map 1:1 to the REST payload below.

## Path C — REST API

Direct HTTP if you're not using MCP or the CLI. Same endpoints,
same payloads.

```http
POST /alert?key=scp-live-...
Content-Type: application/json

{
  "name": "Scrape Success Rate Low",
  "metric_id": "scrape.success_rate",
  "comparator": "lt",
  "threshold": 95,
  "sustained_minutes": 10,
  "notify_channels": [
    {"kind": "email", "target": "ops@example.com"},
    {"kind": "webhook", "target": "https://httpbin.dev/post"}
  ]
}
```

Server-side defaults fill in when zero values are sent:
- `eval_cadence_seconds`: 300
- `sustained_minutes`: the metric family's recommended default (or 5)
- `evaluation_window_m`: `sustained_minutes`
- `no_data_policy`: `ignore`
- `renotify_minutes`: 0 (notify once, never re-notify)
- `recovery_minutes`: 0 (instant recovery)

Other endpoints follow the same shape: `GET /alert`, `GET
/alert/:uuid`, `GET /alert/metric-families`, `GET
/alert/count-active`, `GET /alert/:uuid/series`, `POST
/alert/preview`, `PUT /alert/:uuid`, `DELETE /alert/:uuid`, `POST
/alert/:uuid/snooze`, `POST /alert/:uuid/unsnooze`, `POST
/alert/:uuid/test`.

## Notification channels

Each channel has a `kind` and a `target`:

| Kind | Target | Notes |
|------|--------|-------|
| `email` | RFC 5322 address | Delivered via the Scrapfly mailer. Subject carries the alert name and the breach value. |
| `webhook` | HTTPS URL | POST with HMAC-SHA256 signature in `X-Scrapfly-Webhook-Signature`. Per-alert key generated on create; read it from `alert_get` or the create response. |
| `inapp` | empty string | Surfaces in the dashboard's bell-icon notification panel. |

The dedup layer suppresses re-fires that fall in the same minute
bucket. The renotify cadence (`renotify_minutes`, default 0, meaning notify
once and never re-notify) controls
how often a sustained breach re-notifies, capped at 6 / day per alert.

## State machine

Alerts transition between these lifecycle states (`alert_list` filters
on `state`):

- `ok` — no breach.
- `pending` — breach observed but hasn't yet sustained for
  `sustained_minutes`.
- `triggered` — breach sustained; notifications fired.
- `recovering` — non-breaching but hasn't yet sustained
  `recovery_minutes` of clean data (default 0 = instant recovery).
- `no_data` — the eval window had zero rows. Behavior controlled by
  `no_data_policy`.
- `snoozed` — operator-muted via `alert_snooze`. Resumes on
  unsnooze or the `until_resolved` sentinel auto-clears at the next OK
  transition.

`count-active` sums `triggered + pending + recovering` — the
"needs operator attention" set. Don't include `snoozed` or `no_data`.

## Common error codes

- `ERR::ALERT::UNKNOWN_METRIC` — `metric_id` not in the registry. Re-call `alert_metric_families`.
- `ERR::ALERT::INVALID_DIMENSIONS` — passed a key that isn't in the metric family's `allowed_dimensions`.
- `ERR::ALERT::INVALID_THRESHOLD` — non-finite number, or comparator not in `gt|lt|gte|lte|eq|neq`.
- `ERR::ALERT::INVALID_SUSTAINED_WINDOW` — `sustained_minutes` outside 1–1440.
- `ERR::ALERT::CHANNEL_UNREACHABLE` — zero channels OR channel kind not in `email|webhook|inapp` OR a `test` call couldn't deliver to its channel.
- `ERR::ALERT::PROJECT_NOT_FOUND` — `project_uuid` is not the api-key's project: the agent passed `account_id` by mistake, or another project of the account, which a key never reaches. Fix: omit the field, and use the other project's API key for its alerts.
- `ERR::ALERT::NOT_FOUND` — alert doesn't exist OR isn't yours. The server collapses both cases to prevent enumeration. Don't try to distinguish.
- `ERR::ALERT::EVAL_TIMEOUT` — the preview/series query took too long. Reduce `range_minutes`.
- `ERR::ALERT::INTERNAL` — generic server-side failure; check the response body's `reason` field.

## Related skills

- [`scrapfly-cli`](../scrapfly-cli/) — the CLI binary the `scrapfly alert` subcommand ships with.
- [`scrapfly-agent-rules`](../scrapfly-agent-rules/) — cross-tool golden rules including the monitoring→preview→create flow.
- [`scrapfly-scraper`](../scrapfly-scraper/) — the underlying scrape product; metric IDs like `scrape.success_rate` come from there.

## Reference

- Concept guide: https://scrapfly.io/docs/alerting/getting-started
- Metric registry: https://scrapfly.io/docs/alerting/metric-reference
- State machine: https://scrapfly.io/docs/alerting/state-machine
- Webhook payload + signing: https://scrapfly.io/docs/alerting/webhook-payload
- Error reference: https://scrapfly.io/docs/alerting/error-reference
- Anti-spam (dedup, daily cap, renotify): https://scrapfly.io/docs/alerting/anti-spam
