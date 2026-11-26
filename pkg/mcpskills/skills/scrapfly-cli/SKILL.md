---
name: scrapfly-cli
description: >-
  Use the Scrapfly CLI (`scrapfly`) to scrape web pages, capture screenshots,
  extract structured data with AI, crawl entire sites, and drive a cloud
  browser over CDP from a shell. JSON envelope on stdout for every command.
  Use when the user asks to "scrape a URL", "screenshot a page", "extract
  data from HTML", "crawl a site", "drive a browser with Playwright/CDP",
  "bypass anti-bot", or any task involving the `scrapfly` command.
---

# Scrapfly: Web Scraping, Rendering, and Browser Automation

Scrapfly gives AI agents and applications access to the web through five
products: scraping, screenshots, AI extraction, recursive crawling, and a
remote browser that you can drive directly over CDP, through a
persistent session daemon, or hand to an LLM agent.

This CLI, `scrapfly`, is the single surface over all five products plus a
Playwright-MCP-style agent mode. It speaks JSON by default so LLM callers
can pipe it directly.

## Installation

Pre-built binaries (macOS universal, Linux amd64/arm64, Windows amd64) on
the [Releases page](https://github.com/scrapfly/scrapfly-cli/releases). Or
from source:

```bash
go install github.com/scrapfly/scrapfly-cli/cmd/scrapfly@latest
```

## Six Usage Paths

**Path A (Fetch content):** `scrapfly scrape <url>` - the default. Covers
every `ScrapeConfig` field from the SDK:

- Render & navigate: `--render-js`, `--wait-for-selector`, `--rendering-wait`,
  `--rendering-stage`, `--auto-scroll`, `--js-file FILE`, `--js-scenario-file FILE`
- Anti-bot: `--unblocker` (`--asp` is the deprecated alias and still works),
  `--cost-budget N`, `--proxy-pool`, `--country`, `--geolocation`
- Format & extraction: `--format markdown|clean_html|text|raw` + `--format-option`;
  `--extraction-prompt`, `--extraction-model`, `--extraction-template`,
  `--extraction-template-file FILE` (ephemeral inline template)
- Request shape: `--method`, `--header k=v`, `--cookie n=v`, `--body`,
  `--body-file`, `--data-file FILE` (structured body as JSON map), `--lang`,
  `--os`, `--browser-brand`
- Sessions & caching: `--session`, `--session-sticky-proxy`, `--cache`,
  `--cache-ttl N`, `--cache-clear`, `--retry=true|false`
- Screenshots on scrape: `--screenshot` (fullpage shortcut),
  `--screenshot-named name=selector` (repeatable), `--screenshot-flag FLAG`
- Observability: `--debug`, `--ssl`, `--dns`, `--correlation-id`, `--tag`,
  `--webhook NAME`, `--timing`
- Batch: multiple positional URLs or `--from-file PATH|-` + `--concurrency`
  (ndjson on stdout)
- Pipe-friendly: `--proxified` streams the raw upstream body with no JSON
  envelope

**Path B (Binary / structured outputs):** dedicated commands for focused
products.

- `scrapfly screenshot <url>` - JPG (default), PNG, WebP or GIF of the page or a selector.
  Full `ScreenshotConfig` surface: `--format`, `--capture`, `--resolution`,
  `--country`, `--option`, `--wait-for-selector`, `--rendering-wait`,
  `--auto-scroll`, `--js-file`, `--cache`, `--cache-ttl`, `--cache-clear`,
  `--webhook`, `--vision-deficiency`.
- `scrapfly extract --file page.html --content-type text/html --prompt "..."` - run AI/template
  extraction on a document you already fetched. Full `ExtractionConfig`:
  body via `--file` or stdin; `--content-type`, `--url`, `--charset`,
  `--prompt | --model | --template | --template-file` (one of four
  extraction modes), `--compression`, `--webhook`, `--request-timeout`.
- `scrapfly crawl run <url> --max-pages N` - recursive crawls with status
  polling, content streaming, WARC/HAR artifacts. Post-download:
  `crawl parse har-list|har-get|warc-list|warc-get` reads the artifact
  directly (no separate tooling). See the Crawler section below.

**Path C (Interactive browser - one-shot):** mint a CDP WebSocket URL to
hand to Playwright, Puppeteer, or `browser-use`:

```bash
# plain session
scrapfly browser --pretty

# pre-navigate through /unblock so the target's anti-bot is bypassed
scrapfly browser https://web-scraping.dev/products --unblock --country us --pretty
```

`browser --unblock` and `scrape --unblocker` are two different products with
confusingly similar names: `--unblock` opens a Cloud Browser session through
`POST /unblock`, while `--unblocker` is the anti-bot bypass on a stateless
`scrape` or `crawl`.

**Path D (Interactive browser - persistent daemon):** keep one CDP session
open across many CLI calls. Cookies, tabs, and AXTree refs all persist for
the daemon's lifetime. `--session` is needed only on `start`; subsequent
calls auto-resolve via `~/.scrapfly/sessions/.current`.

```bash
scrapfly browser --session demo --session-timeout 1800 start &
scrapfly browser navigate https://web-scraping.dev/login
scrapfly browser wait 'input[name=username]' --visible
scrapfly browser fill 'input[name=username]' user123
scrapfly browser fill 'input[name=password]' password
scrapfly browser click 'button[type=submit]'
scrapfly browser content --raw > page.html         # fully rendered HTML
scrapfly browser eval 'document.body.innerText'
scrapfly browser close                             # daemon + remote release
```

On a Scrapfly browser, `content` returns the fully rendered HTML with
iframe content inlined unless `--skip-iframes` is set.

**Path E (LLM agent):** `scrapfly agent "<task>" --url <target>` runs an
autonomous loop. The model picks tool calls (open, snapshot, click, fill,
scroll, screenshot, eval, unblock, done), the CLI executes them against
the Browser, and the result feeds back until the model calls `done`.
Providers are pluggable: Anthropic (default), OpenAI, Gemini, or local
(Ollama / OpenAI-compatible). AXTree-based observations keep the token
budget small.

**Path F (REST API only):** skip the CLI, call
`https://api.scrapfly.io/{scrape,screenshot,extraction,crawl}` directly
with `?key=<API_KEY>`. Official SDKs: Python, TypeScript, Go, Rust. See
[docs.scrapfly.io](https://scrapfly.io/docs).

**Path G (Alerting on account metrics):** `scrapfly alert ...` defines
threshold rules on Scrapfly's monitoring metrics (success rate, block
rate, error count, cost, latency, ...) and routes notifications via
email, webhook, or in-app. Open beta, available to all accounts.

```bash
# Discover legal metrics
scrapfly alert metric-families

# Tune the threshold against the last 24h
scrapfly alert preview \
  --metric scrape.success_rate --comparator lt --threshold 95 \
  --sustained-minutes 10 --range-minutes 1440

# Create
scrapfly alert create \
  --name "Scrape Success Rate Low" \
  --metric scrape.success_rate --comparator lt --threshold 95 \
  --sustained-minutes 10 \
  --channel email:ops@web-scraping.dev \
  --channel webhook:https://httpbin.dev/post
```

`alert list / get / update / snooze / unsnooze / test / delete / series
/ count-active` cover the rest of the lifecycle. Per-alert HMAC signing
keys are generated server-side and exposed via `alert get` for webhook
verification. Detailed reference: the
[scrapfly-alerting](../scrapfly-alerting/) skill.

**Path H (Scheduling recurring jobs):** `scrapfly schedule` plus the
per-product subgroups (`scrapfly scrape schedule`,
`scrapfly screenshot schedule`, `scrapfly crawl schedule`) define cron- or
interval-based recurring fires whose results are delivered to a webhook.

```bash
# Daily scrape schedule (config payload from a file, '-' for stdin)
scrapfly scrape schedule create --cron "0 6 * * *" \
  --webhook my-hook --config ./scrape.json

# Interval schedule with an inline payload
scrapfly crawl schedule create --interval 6 --unit hour \
  --webhook my-hook --config-inline '{"url":"https://web-scraping.dev/products"}'

# Cross-product lifecycle (top-level group)
scrapfly schedule list [--status ACTIVE|PAUSED|CANCELLED]
scrapfly schedule get <id>
scrapfly schedule update <id> --set-recurrence --cron "0 */2 * * *"
scrapfly schedule pause <id>     # / resume <id>
scrapfly schedule execute <id>   # fire now, ignoring next_scheduled_date
scrapfly schedule delete <id>    # terminal, cannot be resumed
```

`--webhook` is required (it's where each fire's result lands).
`--cron` wins over `--interval`/`--unit`. `--retry-on-failure
--max-retries N` (0..3) and `--allow-concurrency` tune fire behavior.

**Path I (BYOP Exit Peer):** `scrapfly exit-peer` installs and runs the
Bring-Your-Own-Proxy connector that routes Scrapfly traffic through your
own network egress over an mTLS tunnel.

```bash
scrapfly exit-peer install            # download binary + per-customer cert bundle
scrapfly exit-peer run --name lab-01  # run the connector in the foreground
scrapfly exit-peer status             # local install state + cert lifetime
```

Install dir defaults to `$SCRAPFLY_BYOP_DIR` or `$HOME/.scrapfly/byop`.

## Authentication

Set once:

```bash
export SCRAPFLY_API_KEY=scp-live-...
# or, persisted:
scrapfly config set-key scp-live-...
# interactive (opens a browser to mint a key):
scrapfly auth login        # / logout / whoami
```

Resolution order: `--api-key` flag > `SCRAPFLY_API_KEY` env >
`~/.scrapfly/config.json`. Dev / self-hosted stacks: `--host` and
`--browser-host`, or `SCRAPFLY_API_HOST` / `SCRAPFLY_BROWSER_HOST`.

`scrapfly status` prints CLI version + auth state + account usage in one
call; `scrapfly account` shows plan, quota and subscription usage.

LLM providers for `agent` mode use their own env vars:
`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, or `OLLAMA_HOST`.
The CLI auto-detects the first one set.

## Output contract (for LLMs)

All commands emit a stable JSON envelope on stdout:

```json
{"success": true,  "product": "scrape", "data": { ... }}
{"success": false, "product": "scrape", "error": {"code": "...", "message": "...", "http_status": 422}}
```

Exit code is `0` on success, `1` on any error. `--pretty` swaps JSON for a
one-line human summary. Binary products (screenshots, crawl artifacts,
session recordings) require `-o <file>` or `-O <dir>` for the path; without
either, JSON mode returns base64. `browser content --raw` prints the HTML
body directly to stdout with no envelope (handy for pipes).

## Browser session reference

### Lifecycle
| Command | Purpose |
|---|---|
| `browser --session <id> start [--url <target> --unblock]` | Start daemon; foreground, background it with `&` |
| `browser status` | Print pid, ws_url, alive |
| `browser stop` | Shut down daemon only (remote session lives until timeout) |
| `browser close` | Shut down daemon **and** release Scrapfly's remote browser immediately |

### Actions (sent to the daemon; `--session` optional once `start` has run)
| Command | Notes |
|---|---|
| `browser navigate <url>` | Page.navigate + wait for load |
| `browser snapshot` | AXTree with refs (`e1`, `e2`, …) + `ax_node_id` + `backend_node_id` |
| `browser click <locator>` | Click a CSS/XPath/AX node locator or snapshot ref |
| `browser fill <locator> <value> [--clear --wpm N]` | Fill a field, optionally clearing it and setting typing speed |
| `browser click-ai <description>` | Resolve the target from a natural-language description via the LLM, then click |
| `browser fill-ai <description> <value>` | Resolve the target via the LLM, then fill |
| `browser wait <selector> [--visible --timeout-ms N]` | Wait for an element, optionally requiring visibility |
| `browser slide <source> --target <sel> \| --distance <px>` | Drag a slider to a target or by a distance |
| `browser scroll <direction> [--amount px --ref e3]` | Scroll the page or an element |
| `browser screenshot [--fullpage]` | Capture a PNG; use `-o/-O` for the file |
| `browser content [--raw] [--skip-iframes]` | Rendered HTML with iframes inlined by default |
| `browser eval <js>` | Runtime.evaluate with awaitPromise |

### Locators

`click` / `fill` accept any of:
- **AXTree ref** - `e3`, taken from the last `snapshot`. Uses raw `Input.*` path.
- **CSS selector** (default) - `input[name=username]`
- **XPath** - `//button[text()="Submit"]` with `--selector-type xpath`
- **AX node id** - `42` from snapshot's `ax_node_id` field with `--selector-type axNodeId`
- **AI description** - `ai:<plain English>` (or the `click-ai` / `fill-ai`
  shortcuts) lets an LLM resolve the ref from the live snapshot.

CSS/XPath/AX node locators and snapshot refs are resolved against the live page.

### Recordings, extensions

| Command | Purpose |
|---|---|
| `browser list` | List active Browser sessions on the account |
| `browser video <run-id>` | Download the session recording (webm); needs `-o`/`-O` |
| `browser playback <run-id>` | Fetch playback metadata for a debug run |
| `browser extensions list \| get <id> \| upload <path> \| delete <id>` | Manage account browser extensions (`.zip`/`.crx`) |

### Human takeover (VNC / WebRTC) and credential vault

These flags work on `browser`, `browser start`, and `browser execute`:

| Flag | Purpose |
|---|---|
| `--vault NAME --vault-key B64KEY` | Attach a credential vault; items are decrypted and pushed over CDP before the session yields. The key is yours alone, Scrapfly never stores it, and omitting it fails the session with `ERR::BROWSER::VAULT_KEY_INVALID` |
| `--enable-vnc --vnc-password PW` | Expose the session over VNC so a human can take over |
| `--enable-rtc --rtc-password PW [--rtc-username U]` | Expose the session over WebRTC live video (username defaults to `scrapfly`) |
| `--hitl-allowed-network IP\|CIDR` | Trust a source range to attach with no credentials (repeatable). This REPLACES the password requirement, it does not add to it, and the server then generates its own secret |
| `browser salt` | Print the project salt, `sha256(api_key)[:8]` |

**Which VNC password a viewer types depends on the endpoint.** Scrapfly prefixes
your project salt at allocation, and the two endpoints check it differently:

| Endpoint | Password to type |
|---|---|
| Native client on the TCP mux, `vnc://<run_id>@<host>:5901` | `<project_salt>-<vnc_password>` |
| WebSocket, `wss://<host>/run/<run_id>/vnc` | the raw `<vnc_password>` |

The mux keys its DES challenge on the salted form (native clients do not strip
it), so two customers who pick the same password never collide; the WebSocket
handler strips the salt server-side before the same check. `browser start`
prints the TCP URL and the salted password that goes with it:

```bash
scrapfly browser --session demo --enable-vnc --vnc-password hunter2 start &
# [session demo] vnc vnc://01K5Z.../@browser.scrapfly.io:5901
# [session demo] vnc password <project_salt>-hunter2
```

The `vnc://` username is the run id, which only exists once the session is
allocated, so `browser start` prints the URL after it connects. In ws mode
(`scrapfly browser --enable-vnc --vnc-password PW`) the JSON envelope carries
`project_salt`, `vnc_password` (salted), and `vnc_url` with a `{run_id}`
placeholder.

## Crawler reference

### Submit + poll
| Command | Purpose |
|---|---|
| `crawl start <url>` | Submit a crawl; print the UUID immediately |
| `crawl run <url>` | Submit + poll until DONE / FAILED / CANCELLED |
| `crawl watch <uuid>` | Live stats panel (visited/extracted/failed/skipped/queue/credits) until terminal |
| `crawl status <uuid>` | Fetch status; envelope exposes a top-level `terminal` field (`running\|done\|failed\|cancelled`) for shell scripting |
| `crawl urls <uuid> --status visited\|pending\|failed\|skipped` | Stream discovered URLs (paginated) |
| `crawl contents <uuid> --format markdown [--url URL --plain]` | Bulk JSON or single-URL plain body |
| `crawl contents-batch <uuid> <url...> [--from-file F] --format markdown` | One round-trip for up to 100 URLs |
| `crawl cancel <uuid>` | Stop a running crawl |
| `crawl artifact <uuid> --type warc\|har -o file` | Download the recording |

### Flags on `crawl start` / `crawl run`
Every documented `CrawlerConfig` field is exposed:

| Flag | Maps to |
|---|---|
| `--max-pages N` | `page_limit` |
| `--max-depth N` | `max_depth` |
| `--max-duration N` | `max_duration` (seconds, 15-10800) |
| `--max-api-credit N` | `max_api_credit` |
| `--exclude PATH` / `--include PATH` | `exclude_paths` / `include_only_paths` (repeatable, mutually exclusive) |
| `--ignore-base-path` | `ignore_base_path_restriction` |
| `--follow-external` / `--allowed-external-domain D` | `follow_external_links` / `allowed_external_domains` |
| `--follow-internal-subdomains true\|false` (tri-state) / `--allowed-internal-subdomain D` | matching fields |
| `--header k=v` (repeatable) | `headers` |
| `--delay MS` / `--user-agent UA` / `--max-concurrency N` / `--rendering-delay MS` | request shape |
| `--use-sitemaps` / `--ignore-no-follow` / `--respect-robots-txt true\|false` | strategy |
| `--cache` / `--cache-ttl S` / `--cache-clear` | caching |
| `--content-format F` (repeatable) / `--extraction-rules-file PATH` | content + extraction |
| `--unblocker` / `--proxy-pool P` / `--country CC` | scraping options |
| `--webhook NAME` / `--webhook-event E` (repeatable) | webhooks |

### Read from a WARC / HAR artifact

```bash
scrapfly crawl artifact <uuid> --type warc -o crawl.warc
scrapfly crawl parse warc-list crawl.warc --pretty             # URLs + statuses
scrapfly crawl parse warc-get crawl.warc https://web-scraping.dev/products \
  --raw > page.html                                             # body straight to stdout

scrapfly crawl artifact <uuid> --type har -o crawl.har
scrapfly crawl parse har-list crawl.har --pretty               # request table
scrapfly crawl parse har-get crawl.har https://web-scraping.dev/product/1 \
  --raw | jq .                                                  # extract one response
```

`-` as the file path reads from stdin, so you can pipe straight from
`crawl artifact -o -`.

## Core API Endpoints

| Product | Endpoint | CLI |
|---|---|---|
| Web Scraping | `GET/POST /scrape` | `scrapfly scrape <url>` |
| Screenshot | `POST /screenshot` | `scrapfly screenshot <url>` |
| Extraction | `POST /extraction` | `scrapfly extract` |
| Crawler | `POST /crawl` + `/crawl/{uuid}/{status,urls,contents,artifact}` | `scrapfly crawl {start,run,status,urls,contents,cancel,artifact}` |
| Browser (CDP) | `wss://browser.scrapfly.io?api_key=...` | `scrapfly browser` |
| Browser Unblock | `POST /unblock` | `scrapfly browser <url> --unblock` |
| Browser Session Stop | `POST /session/{id}/stop` | `scrapfly browser close` |
| Session Recording | `GET /run/{id}/video` | `scrapfly browser video <run-id>` |
| Feedback | `POST /dashboard/api/feedback` (on `scrapfly.io`, not the API host) | `scrapfly feedback` |

## Quick examples

```bash
# Scrape a JS-heavy page with the unblocker and markdown output
scrapfly scrape https://web-scraping.dev/products --render-js --unblocker --format markdown

# Scrape many URLs in parallel (ndjson on stdout, one envelope per line)
scrapfly scrape https://web-scraping.dev/products \
               https://web-scraping.dev/product/1 \
               https://web-scraping.dev/product/2 \
               --format markdown --concurrency 5

# Or feed URLs from a file
scrapfly scrape --from-file urls.txt --format markdown | jq -c '.data.result.url'

# Pipe scrape body straight into AI extraction
scrapfly scrape https://web-scraping.dev/product/1 --render-js --proxified \
  | scrapfly extract --content-type text/html --prompt "name, price, sku"

# Screenshot into an auto-named file in a directory
scrapfly -O ./shots screenshot https://web-scraping.dev/products --resolution 1920x1080

# Crawl synchronously with markdown content streaming
scrapfly crawl run https://web-scraping.dev/products --max-pages 20 --content-format markdown

# Multi-call browser session: log in, dump rendered HTML, clean up
scrapfly browser --session demo --session-timeout 1800 start &
scrapfly browser navigate https://web-scraping.dev/login
scrapfly browser wait 'input[name=username]' --visible
scrapfly browser fill 'input[name=username]' user123 --wpm 280
scrapfly browser fill 'input[name=password]' password
scrapfly browser click 'button[type=submit]'
scrapfly browser content --raw > after-login.html
scrapfly browser close

# Hand a Scrapfly-unblocked CDP URL to browser-use
WS=$(scrapfly browser https://web-scraping.dev/products --unblock --country us | jq -r .data.ws_url)
python examples/browser-use-with-scrapfly.py   # mints its own CDP URL through scrapfly browser

# LLM agent with structured output
ANTHROPIC_API_KEY=... scrapfly agent "Name and price of the first product" \
  --url https://web-scraping.dev/products \
  --schema '{"type":"object","properties":{"name":{"type":"string"},"price":{"type":"string"}},"required":["name","price"]}'
```

## Reporting product feedback

`scrapfly feedback` files a report about a Scrapfly surface - the CLI, an API,
the docs, the dashboard, the MCP server, a skill - from the terminal. It lands
in the same store the dashboard feedback widget writes to, which is where the
team reads reports. It is not support: no ticket is opened and no reply comes
back, so an account or integration problem still goes to
https://scrapfly.io/dashboard/support.

```bash
scrapfly feedback --sentiment negative --feature crawler_api \
  --message "crawl status returned 200 with an empty urls array for ten minutes after the crawl finished" \
  --context "Mirroring a 300-page docs site, polling crawl status every 15s, CLI 0.4.1"
```

If you are an agent driving this CLI: when a Scrapfly surface was useful,
confusing, wrong, or incomplete during a session, draft **one** report at the
end of the session, once the user's task is done. Show them the draft and file
it only if they agree - it goes out on their API key and carries their account.

- `--sentiment negative` a defect or blocker stopped a command from working;
  `neutral` friction or a workaround, but the task completed; `positive` it
  worked, or a step behaved better than expected. File positive reports too.
- `--message` names the command and what it did, `--context` the goal and the
  CLI version. Both are 10-2000 characters, and `--message` needs five words:
  "it broke" is not a report anyone can act on.
- `--feature` scopes the report to a product area. An unknown value is rejected
  with the full list, so you can correct it from the error.
- `--actor` defaults to `agent` when a coding-agent environment variable is
  present, `human` otherwise. Pass `--actor human` when you are relaying the
  user's own words, and keep their wording.
- Report the **product**, not your own environment, not the model's behaviour,
  not a prompt you found unclear, and not a tool error you recovered from on
  the next call.
- A failed submission is never a reason to fail, stall or retry the task that
  produced it. On `429` the account's hourly budget is spent: drop the report.
  On any other error, show the drafted `--message` and `--context` so the user
  can file it by hand.

## Shell completion

Cobra ships completion scripts for bash, zsh, fish, and PowerShell. Install once:

```bash
# zsh
scrapfly completion zsh > "${fpath[1]}/_scrapfly"
# bash
scrapfly completion bash > /etc/bash_completion.d/scrapfly  # or ~/.bash_completion.d/
# fish
scrapfly completion fish > ~/.config/fish/completions/scrapfly.fish
```

## MCP server

`scrapfly mcp serve` exposes `scrape`, `screenshot`, `extract`, `crawl_run`,
and `selector` as MCP tools over stdio. Add the binary to a compatible
client (Claude Desktop, Cursor, Claude Code):

```jsonc
{
  "mcpServers": {
    "scrapfly": {
      "command": "scrapfly",
      "args": ["mcp", "serve"],
      "env": { "SCRAPFLY_API_KEY": "scp-live-..." }
    }
  }
}
```

Selector discovery also works from inside the agent loop via the
`find_selector` tool - useful when the model wants to produce a durable
CSS selector instead of relying on per-snapshot refs.

## Reference

- Docs: https://scrapfly.io/docs
- SDKs: https://scrapfly.io/docs/sdk
- CLI source + examples: https://github.com/scrapfly/scrapfly-cli
