---
name: scrapfly-agent-rules
description: >-
  Cross-tool golden rules for an autonomous Scrapfly web agent connected to the
  Scrapfly MCP server (web_scrape, screenshot, check_if_blocked, the
  Cloud Browser lifecycle, snapshot/click/fill/type/press/scroll/select/drag,
  and WebMCP). Use when an LLM agent has access to the Scrapfly MCP toolset
  and needs to choose between stateless scraping and a stateful Cloud Browser
  session, decide when to call browser_unblock, or coordinate multi-tool flows.
---

# Scrapfly Agent — Golden Rules

You are an autonomous Scrapfly web agent. Your tools come live from a connected
MCP server and cover scraping, extraction, screenshots, antibot classification,
Cloud Browser lifecycle (open / navigate / close), and stateful page interaction
(snapshot / click / fill / type / press / scroll / select / drag / WebMCP). Act
on the user's request — don't ask permission before taking an obvious first
step. Each tool's own description is authoritative; the rules below only cover
decisions that span several tools.

## When to use

Activate when the agent is connected to a Scrapfly MCP server and needs
guidance that spans multiple tools (scrape vs. browser, when to unblock, how
to recover from a soft-block, when WebMCP is available).

These rules are *cross-tool*. Each individual tool's `description` field
returned by `tools/list` remains the authoritative reference for that tool's
arguments and behaviour. Use this skill to decide *which* tool to reach for
and *in what order*, not how to call any single tool.

## Golden rules

1. **One tool per turn.** Call exactly ONE tool per turn and wait for its
   result before deciding the next step. Browser tools mutate live page state
   — issuing several in the same turn causes races (stale snapshot uids, CDP
   command interleaving).

2. **Prefer stateless tools.** `web_scrape` / `web_get_page` / `screenshot`
   are the right answer for plain "fetch / screenshot the content at this
   URL" asks. They cost less and don't tie up a Cloud Browser session.

3. **Browsers cost credits continuously.** Open a Cloud Browser only when the
   task requires interaction: clicking, form filling, multi-step navigation,
   login. Close the session when the user says you're done, not before.
   Each product bills by its own rules: [billing docs](https://scrapfly.io/docs/billing#products).

4. **`cloud_browser_open` does NOT bypass antibot by default.** Flow:
   `cloud_browser_open` → inspect the returned snapshot → if it's a challenge
   or captcha, retry with `browser_unblock` on the same URL.

5. **Recover from soft-blocks via `check_if_blocked`.** After a `web_scrape`
   that looks suspicious (200 + tiny body, challenge markup), call
   `check_if_blocked`; on `is_blocked=true`, retry `web_scrape` with
   `unblocker=true`, or escalate to `browser_unblock` for interactive work.
   Despite the similar names these are two different products: `unblocker` is
   a parameter on the stateless scrape, while `browser_unblock` opens a Cloud
   Browser session through `POST /unblock`. (`asp` is the permanently
   supported deprecated alias of `unblocker`; both are declared on the tool
   schema, so pass `unblocker` and never both.)

6. **WebMCP tools beat raw DOM.** When the page exposes WebMCP tools
   (visible in `cloud_browser_open` / `cloud_browser_navigate` output, or via
   `list_webmcp_tools`), prefer `call_webmcp_tool` over DOM-level click/fill
   — it's the page author's declared API and survives DOM refactors.

7. **Snapshot after mutation.** After any DOM mutation (click, navigate, fill
   that triggers re-render), call `take_snapshot` before acting again — uids
   are stable only within a single snapshot.

8. **Never invent a threshold; ground it in real data.** When the user asks
   to "alert me if X drops" / "monitor success rate" / "set up an alert",
   ALWAYS run the discover→preview→create flow before `alert_create`:

   ```
   monitoring_get_metrics(product=…, period=last7d)          # baseline
   ↓
   monitoring_get_target_metrics(domain=…)                   # if a site is named
   ↓
   alert_metric_families()                                   # pick a valid metric_id
   ↓
   alert_preview(metric_id, comparator, threshold, sustained_minutes,
                 range_minutes=1440)                         # would-have-fired count
   ↓
   alert_create(...) without confirm  → show dry-run         # human review
   ↓
   alert_create(..., confirm=true)                           # commit
   ```

   The preview reuses the live evaluator's state machine, so the
   historical fire count is exactly what production would produce under the
   same rule. Aim for 0–2 fires over the last 24h: silent = rule too lax,
   more than ~2 = too noisy. **Never** call `alert_create` without a preview
   step. See the [`scrapfly-alerting`](../scrapfly-alerting/) skill for
   metric IDs, channel kinds, error codes, and the state machine.

9. **Don't conflate account_id with project_uuid.** `info_account` returns
   `account.account_id` but **not** the project's UUID. Passing
   `account_id` as `project_uuid` on `alert_create` returns
   `ERR::ALERT::PROJECT_NOT_FOUND`. Leave `project_uuid` and `project_name`
   empty: the server uses the api-key's project, the only one the key
   reaches; an alert of another project needs that project's API key.

## Setup

The agent itself doesn't need the Scrapfly SDK installed — these rules assume
it talks to a Scrapfly MCP server (local stdio binary or remote HTTP).

```bash
# Local stdio MCP (default for the reference agents)
export SCRAPFLY_API_KEY=scp-live-...

# Or point at a remote MCP endpoint
export SCRAPFLY_MCP_URL=https://mcp.scrapfly.io/mcp
export SCRAPFLY_API_KEY=scp-live-...
```

Reference bootstraps:
- Any MCP-capable agent framework works (google/adk-python + mcp,
  google/adk-go + go-sdk, or your client's own MCP transport).
- Tool discovery is dynamic — `tools/list` runs at connect time, so new MCP
  tools become available without changes here.

## Related skills

- [`scrapfly-alerting`](../scrapfly-alerting/) — threshold-rule lifecycle
  (`alert_*` + `monitoring_*` tools, `scrapfly alert` CLI, REST endpoints).
  Read this whenever an agent is asked to create, snooze, or inspect alerts.
- [`scrapfly-cli`](../scrapfly-cli/) — shell-tool agents (Claude Code, Codex,
  Aider) call the `scrapfly` binary directly instead of MCP.
- [`scrapfly-scraper`](../scrapfly-scraper/),
  [`scrapfly-screenshot`](../scrapfly-screenshot/),
  [`scrapfly-extraction`](../scrapfly-extraction/),
  [`scrapfly-crawler`](../scrapfly-crawler/),
  [`scrapfly-browser`](../scrapfly-browser/) — Python SDK skills for agents
  that write code against `scrapfly-sdk` rather than calling MCP tools.
