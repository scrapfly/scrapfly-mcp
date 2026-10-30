# Scrapfly MCP Server

<p align="center">
  <a href="https://scrapfly.io">
    <img src="https://avatars.githubusercontent.com/u/54183743?s=400&u=5279c1aaea18805aa5cc4fec1053ac2a2cfaac5d&v=4" alt="Scrapfly" width="200"/>
  </a>
</p>

<p align="center">
  <strong>Give your AI real-time access to any website</strong>
</p>

<p align="center">
  <a href="https://scrapfly.io/mcp-cloud">🌐 Landing Page</a> •
  <a href="https://scrapfly.io/docs/mcp/getting-started">📖 Documentation</a> •
  <a href="https://scrapfly.io/mcp-cloud/n8n">🎮 Live Demo</a> •
  <a href="https://scrapfly.io/register">🔑 Get API Key</a>
</p>

---

## At a glance

| | |
|---|---|
| **Remote server** | `https://mcp.scrapfly.io/mcp` (streamable HTTP, nothing to install) |
| **Self-hosted** | Go binary (stdio or `-http`) or the Docker image of this repo, see Self-Hosted below |
| **stdio-only clients** | `npx mcp-remote https://mcp.scrapfly.io/mcp` |
| **Authentication** | OAuth2 sign-in on the remote server, or a Scrapfly API key (`SCRAPFLY_API_KEY`, `Authorization: Bearer <key>`) |
| **Tools** | **62**: web scraping (5), account (2), Cloud Browser sessions (9), page interaction (17), WebMCP (2), credential vaults (15), alerts (12). Full list under Available Tools below. |

Add it to any MCP client:

```json
{
  "mcpServers": {
    "scrapfly": { "type": "http", "url": "https://mcp.scrapfly.io/mcp" }
  }
}
```

Tools that fetch pages need no setup beyond the key: `web_get_page` (quick fetch), `web_scrape` (full control: JS rendering, proxy pool, country, extraction) and `screenshot`.

---

## What is Scrapfly MCP?

The **Scrapfly MCP Server** connects your AI assistants to live web data through the [Model Context Protocol](https://modelcontextprotocol.io). Transform your AI from being limited by training data to having real-time access to **any website**.

### ✨ What Your AI Can Do

| Capability | Description |
|------------|-------------|
| 🌐 **Scrape Live Data** | Pull current prices, listings, news, or any webpage content in real-time |
| 🛡️ **Bypass Anti-Bot Systems** | Automatically handle CAPTCHAs, proxies, JavaScript rendering, and rate limits |
| ⚡ **Extract Structured Data** | Parse complex websites into clean JSON using [AI-powered extraction](https://scrapfly.io/docs/scrape-api/extraction) |
| 📸 **Capture Screenshots** | Take visual snapshots of pages or specific elements for analysis |

### 🏆 Why Scrapfly?

Built on **battle-tested infrastructure** used by thousands of developers:

- **99.9% Uptime** — Enterprise-grade reliability
- **100+ Countries** — [Global proxy network](https://scrapfly.io/docs/scrape-api/proxy) with datacenter & residential IPs, in three proxy pools (Basic, Unblocker, Unblocker Ultra) or Auto, where the API picks the pool
- **Anti-Bot Bypass** — [Advanced Unblocker technology](https://scrapfly.io/docs/scrape-api/anti-scraping-protection) defeats modern protections
- **OAuth2 Security** — [Enterprise authentication](https://scrapfly.io/docs/mcp/authentication) for production deployments

> 📖 **Learn more**: [Why Scrapfly MCP?](https://scrapfly.io/docs/mcp/getting-started#why-scrapfly-mcp)

---

## 🚀 Quick Install

Click one of the buttons below to install the MCP server in your preferred IDE:

[![Install in VS Code](https://img.shields.io/badge/Install_in-VS_Code-0098FF?style=for-the-badge&logo=visualstudiocode&logoColor=white)](https://vscode.dev/redirect/mcp/install?name=scrapfly-cloud-mcp&config=%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D)
[![Install in VS Code Insiders](https://img.shields.io/badge/Install_in-VS_Code_Insiders-24bfa5?style=for-the-badge&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=scrapfly-cloud-mcp&config=%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D&quality=insiders)
[![Install in Visual Studio](https://img.shields.io/badge/Install_in-Visual_Studio-C16FDE?style=for-the-badge&logo=visualstudio&logoColor=white)](https://vs-open.link/mcp-install?%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D)
[![Install in Cursor](https://img.shields.io/badge/Install_in-Cursor-000000?style=for-the-badge&logoColor=white)](https://cursor.com/en/install-mcp?name=scrapfly-cloud-mcp&config=eyJuYW1lIjoic2NyYXBmbHktY2xvdWQtbWNwIiwidHlwZSI6Imh0dHAiLCJ1cmwiOiJodHRwczovL21jcC5zY3JhcGZseS5pby9tY3AifQ==)

---

## 📦 Manual Installation

### Standard Configuration

Works with most MCP-compatible tools:

```json
{
  "servers": {
    "scrapfly-cloud-mcp": {
      "type": "http",
      "url": "https://mcp.scrapfly.io/mcp"
    }
  }
}
```

### Cloud Configuration (NPX)

For tools that require a local process:

```json
{
  "mcpServers": {
    "scrapfly": {
      "command": "npx",
      "args": [
        "mcp-remote",
        "https://mcp.scrapfly.io/mcp"
      ]
    }
  }
}
```

---

## 🔧 IDE-Specific Setup

<details>
<summary><strong>VS Code</strong></summary>

### One-Click Install

[![Install in VS Code](https://img.shields.io/badge/Install_in-VS_Code-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://vscode.dev/redirect/mcp/install?name=scrapfly-cloud-mcp&config=%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D)

### Manual Install

Follow the [VS Code MCP guide](https://code.visualstudio.com/docs/copilot/chat/mcp-servers#_add-an-mcp-server) or use the CLI:

```bash
code --add-mcp '{"name":"scrapfly-cloud-mcp","type":"http","url":"https://mcp.scrapfly.io/mcp"}'
```

After installation, Scrapfly tools will be available in GitHub Copilot Chat.

> 📖 **Full guide**: [VS Code Integration](https://scrapfly.io/docs/mcp/integrations/vscode)
</details>

<details>
<summary><strong>VS Code Insiders</strong></summary>

### One-Click Install

[![Install in VS Code Insiders](https://img.shields.io/badge/Install_in-VS_Code_Insiders-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=scrapfly-cloud-mcp&config=%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D&quality=insiders)

### Manual Install

```bash
code-insiders --add-mcp '{"name":"scrapfly-cloud-mcp","type":"http","url":"https://mcp.scrapfly.io/mcp"}'
```

> 📖 **Full guide**: [VS Code Integration](https://scrapfly.io/docs/mcp/integrations/vscode)
</details>

<details>
<summary><strong>Visual Studio</strong></summary>

### One-Click Install

[![Install in Visual Studio](https://img.shields.io/badge/Install_in-Visual_Studio-C16FDE?style=flat-square&logo=visualstudio&logoColor=white)](https://vs-open.link/mcp-install?%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D)

### Manual Install

1. Open Visual Studio
2. Navigate to **GitHub Copilot Chat** window
3. Click the tools icon (🛠️) in the chat toolbar
4. Click **+ Add Server** to open the configuration dialog
5. Configure:
   - **Server ID**: `scrapfly-cloud-mcp`
   - **Type**: `http/sse`
   - **URL**: `https://mcp.scrapfly.io/mcp`
6. Click **Save**

> 📖 **Full guide**: [Visual Studio MCP documentation](https://learn.microsoft.com/visualstudio/ide/mcp-servers)
</details>

<details>
<summary><strong>Cursor</strong></summary>

### One-Click Install

[![Install in Cursor](https://img.shields.io/badge/Install_in-Cursor-000000?style=flat-square&logoColor=white)](https://cursor.com/en/install-mcp?name=scrapfly-cloud-mcp&config=eyJuYW1lIjoic2NyYXBmbHktY2xvdWQtbWNwIiwidHlwZSI6Imh0dHAiLCJ1cmwiOiJodHRwczovL21jcC5zY3JhcGZseS5pby9tY3AifQ==)

### Manual Install

1. Go to `Cursor Settings` → `MCP` → `Add new MCP Server`
2. Use the standard configuration above
3. Click **Edit** to verify or add arguments

> 📖 **Full guide**: [Cursor Integration](https://scrapfly.io/docs/mcp/integrations/cursor)
</details>

<details>
<summary><strong>Claude Code</strong></summary>

Use the Claude Code CLI:

```bash
claude mcp add scrapfly-cloud-mcp --url https://mcp.scrapfly.io/mcp
```

> 📖 **Full guide**: [Claude Code Integration](https://scrapfly.io/docs/mcp/integrations/claude-code)
</details>

<details>
<summary><strong>Claude Desktop</strong></summary>

Add to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "scrapfly": {
      "command": "npx",
      "args": ["mcp-remote", "https://mcp.scrapfly.io/mcp"]
    }
  }
}
```

> 📖 **Full guide**: [Claude Desktop Integration](https://scrapfly.io/docs/mcp/integrations/claude-desktop)
</details>

<details>
<summary><strong>Cline</strong></summary>

Add to your Cline MCP settings:

```json
{
  "scrapfly-cloud-mcp": {
    "type": "http",
    "url": "https://mcp.scrapfly.io/mcp"
  }
}
```

> 📖 **Full guide**: [Cline Integration](https://scrapfly.io/docs/mcp/integrations/cline)
</details>

<details>
<summary><strong>Windsurf</strong></summary>

Follow the [Windsurf MCP documentation](https://docs.windsurf.com/windsurf/cascade/mcp) using the standard configuration.

> 📖 **Full guide**: [Windsurf Integration](https://scrapfly.io/docs/mcp/integrations/windsurf)
</details>

<details>
<summary><strong>Zed</strong></summary>

Add to your Zed settings:

```json
{
  "context_servers": {
    "scrapfly-cloud-mcp": {
      "type": "http",
      "url": "https://mcp.scrapfly.io/mcp"
    }
  }
}
```

> 📖 **Full guide**: [Zed Integration](https://scrapfly.io/docs/mcp/integrations/zed)
</details>

<details>
<summary><strong>OpenAI Codex</strong></summary>

Create or edit `~/.codex/config.toml`:

```toml
[mcp_servers.scrapfly-cloud-mcp]
url = "https://mcp.scrapfly.io/mcp"
```

> 📖 **More info**: [Codex MCP documentation](https://github.com/openai/codex/blob/main/codex-rs/config.md#mcp_servers)
</details>

<details>
<summary><strong>Gemini CLI</strong></summary>

Follow the [Gemini CLI MCP guide](https://github.com/google-gemini/gemini-cli/blob/main/docs/tools/mcp-server.md) using the standard configuration.
</details>

<details>
<summary><strong>OpenCode</strong></summary>

Add to `~/.config/opencode/opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "scrapfly-cloud-mcp": {
      "type": "http",
      "url": "https://mcp.scrapfly.io/mcp",
      "enabled": true
    }
  }
}
```

> 📖 **More info**: [OpenCode MCP documentation](https://opencode.ai/docs/mcp-servers/)
</details>

---

## 🛠️ Available Tools

The Scrapfly MCP Server exposes **62 tools** in seven groups. Most tasks need only the first group.

### Web scraping (5)

Stateless, one call per page.

| Tool | Description | Use Case |
|------|-------------|----------|
| `scraping_instruction_enhanced` | Cheat-sheet of the scraping options | Optional, when the right options are unclear |
| `web_get_page` | Quick page fetch with smart defaults | Simple scraping tasks |
| `web_scrape` | Full control: headers, JS rendering, country, proxy pool, extraction | Complex scraping, login flows |
| `screenshot` | One-shot PNG of a URL | Visual analysis, monitoring |
| `check_if_blocked` | Decide whether a scrape result is a block page | Retry or escalate the proxy pool |

### Account (2)

| Tool | Description |
|------|-------------|
| `info_account` | Subscription plan, API credit usage, concurrency |
| `info_api_key` | The API key this server is authenticated with |

### Cloud Browser sessions (9)

A stateful real browser, kept between calls.

| Tool | Description |
|------|-------------|
| `cloud_browser_open` | Start a session at a URL |
| `browser_unblock` | Same, through the anti-bot bypass first |
| `cloud_browser_navigate` | Load a new URL in the open session |
| `cloud_browser_sessions` | List active sessions |
| `cloud_browser_close` | Close the session |
| `cloud_browser_screenshot` | PNG of the session, optionally of a CSS selector |
| `cloud_browser_eval` | Run JavaScript and return its value |
| `cloud_browser_downloads` | Files downloaded during the session |
| `cloud_browser_performance` | PageSpeed-style lab run with mobile throttling |

### Page interaction (17)

Act on the open Cloud Browser session. Elements are addressed by the `uid` that `take_snapshot` returns.

`take_snapshot`, `take_screenshot`, `find_elements`, `get_page_url`, `evaluate_script`, `click`, `hover`, `drag`, `fill`, `type_text`, `press_key`, `select_option`, `dropdown_options`, `scroll`, `scroll_to_text`, `go_back`, `wait`

### WebMCP (2)

For pages that register their own tools with `navigator.modelContext`.

`list_webmcp_tools`, `call_webmcp_tool`

### Credential vaults (15)

Store logins for Cloud Browser sessions, optionally mirrored from 1Password.

`cloud_browser_vault_create`, `cloud_browser_vault_list`, `cloud_browser_vault_get`, `cloud_browser_vault_update`, `cloud_browser_vault_delete`, `cloud_browser_vault_rotate`, `cloud_browser_vault_item_create`, `cloud_browser_vault_item_list`, `cloud_browser_vault_item_update`, `cloud_browser_vault_item_delete`, `cloud_browser_vault_service_link`, `cloud_browser_vault_service_update`, `cloud_browser_vault_service_sync`, `cloud_browser_vault_service_test`, `cloud_browser_vault_service_unlink`

### Alerts (12)

Create, test and snooze alert rules on your account metrics. Writes take a confirm step.

`alert_metric_families`, `alert_preview`, `alert_create`, `alert_update`, `alert_delete`, `alert_get`, `alert_list`, `alert_count_active`, `alert_series`, `alert_test`, `alert_snooze`, `alert_unsnooze`

> 📖 **Full reference**: [Tools & API Specification](https://scrapfly.io/docs/mcp/tools)

### Proxy pools

`web_scrape` and `web_get_page` take a `proxy_pool`, which sets both the proxies and the Unblocker
(anti-bot bypass):

| `proxy_pool` | What it runs |
|---|---|
| `basic` | datacenter proxies, Unblocker off |
| `unblocker` | residential proxies, Unblocker on |
| `unblocker_ultra` | residential proxies, Unblocker on, the only pool accepted by targets and anti-bot protections that have a price of their own |
| `auto` | the API picks the pool before the request, from the target's profile: the Unblocker on datacenter proxies, or the pool the target needs; priced as the pool it runs on |

Pick the pool a site needs: `basic` where datacenter proxies get through, `unblocker` or
`unblocker_ultra` where it needs residential proxies and the Unblocker, `auto` to let the API pick.
A target or an anti-bot protection with a price of its own refuses `basic` and `unblocker` with
`ERR::PROXY::UNBLOCKER_ULTRA_REQUIRED` (HTTP 422, not billed): resend with
`proxy_pool=unblocker_ultra`, or with `proxy_pool=auto`, which is accepted on every target, the ones
that require `unblocker_ultra` included, where their price is added. `public_tor_pool` runs on the
Tor network (.onion).

#### Deprecated: `unblocker` and `asp`

The `unblocker` input of `web_scrape` and its former name `asp` are deprecated: use `proxy_pool`
(`unblocker_ultra` for the Unblocker). The
[migration guide](https://scrapfly.io/new-pricing#legacy-spellings) covers requests written before
the proxy pools, the pool names `public_datacenter_pool` and `public_residential_pool` included.
Without a `proxy_pool`, the server sends the API no pool and, by default, `asp=true`, a request the
guide covers as well.

### Example: Scrape a Page

```
User: "What are the top posts on Hacker News right now?"

AI: Uses web_get_page to fetch https://news.ycombinator.com and returns current top stories
```

### Example: Extract Structured Data

```
User: "Get all product prices from this Amazon page"

AI: Uses web_scrape with extraction_model="product_listing" to return structured JSON
```

> 📖 **More examples**: [Real-World Examples](https://scrapfly.io/docs/mcp/examples)

---

## 🔐 Authentication

Scrapfly MCP supports multiple authentication methods:

| Method | Best For | Documentation |
|--------|----------|---------------|
| **OAuth2** | Production, multi-user apps | [OAuth2 Setup](https://scrapfly.io/docs/mcp/authentication#oauth2) |
| **API Key** | Personal use, development | [API Key Setup](https://scrapfly.io/docs/mcp/authentication#api-key) |
| **Header Auth** | Custom integrations | [Header Auth](https://scrapfly.io/docs/mcp/authentication#header) |

> 🔑 **Get your API key**: [Scrapfly Dashboard](https://scrapfly.io/dashboard)

---

## 📊 Configuration Reference

| Setting | Value |
|---------|-------|
| **Server Name** | `scrapfly-cloud-mcp` |
| **Type** | Remote HTTP Server |
| **URL** | `https://mcp.scrapfly.io/mcp` |
| **Protocol** | MCP over HTTP/SSE |

---

## 🖥️ Self-Hosted / Local Deployment

You can run the Scrapfly MCP server locally or self-host it.

### CLI Arguments

| Flag | Description |
|------|-------------|
| `-http <address>` | Start HTTP server at the specified address (e.g., `:8080`). Takes precedence over `PORT` env var. In server-key mode this binds loopback (`127.0.0.1`) unless you also pass `-allow-remote`. |
| `-apikey <key>` | Use this API key instead of the `SCRAPFLY_API_KEY` environment variable. |
| `-allow-remote` | Server-key HTTP mode only: permit binding a non-loopback interface. That mode performs no per-caller authentication — every request uses the baked-in key — so only enable this behind your own auth proxy. |
| `-verify-ssl` | Verify TLS certificates on outbound calls (default `true`). Set `false` only when targeting a host that serves a self-signed certificate. |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `PORT` | HTTP port to listen on. Used if `-http` flag is not set. |
| `SCRAPFLY_API_KEY` | Scrapfly API key baked into the server. When set, the server uses it for every request (server-key mode) and does **not** read a per-request key. |
| `SCRAPFLY_ALLOW_REMOTE` | `1`/`true` is the env equivalent of `-allow-remote`. |
| `SCRAPFLY_VERIFY_SSL` | `0`/`false` is the env equivalent of `-verify-ssl=false`. |

Per-request key: start the server **without** a baked-in key (`-http` only) and
each caller supplies their own key per request — as `Authorization: Bearer <key>`,
or `?key=<key>` / `?apiKey=<key>`. This path is authenticated; the server-key
path above is not, which is why it stays on loopback by default.

### Examples

```bash
# Start HTTP server on port 8080
./scrapfly-mcp -http :8080

# Start HTTP server using PORT env var
PORT=8080 ./scrapfly-mcp

# Start with an API key. Prefer the environment variable: a key passed as a
# flag lands in your shell history and in the process list.
# Server-key mode has no per-caller auth, so it binds 127.0.0.1 here.
SCRAPFLY_API_KEY=... ./scrapfly-mcp -http :8080

# Same, but expose it on all interfaces — only behind your own auth proxy.
SCRAPFLY_API_KEY=... ./scrapfly-mcp -http 0.0.0.0:8080 -allow-remote

# Start in stdio mode (for local MCP clients)
./scrapfly-mcp
```

### Docker

```bash
# Build
docker build -t scrapfly-mcp .

# Run (Smithery compatible - uses PORT env var)
docker run -p 8080:8080 scrapfly-mcp

# Run with custom port
docker run -e PORT=9000 -p 9000:9000 scrapfly-mcp
```

---

## 🤝 Framework Integrations

Scrapfly MCP also works with AI frameworks and automation tools:

| Framework | Documentation |
|-----------|---------------|
| **LangChain** | [LangChain Integration](https://scrapfly.io/docs/mcp/integrations/langchain) |
| **LlamaIndex** | [LlamaIndex Integration](https://scrapfly.io/docs/mcp/integrations/llamaindex) |
| **CrewAI** | [CrewAI Integration](https://scrapfly.io/docs/mcp/integrations/crewai) |
| **OpenAI** | [OpenAI Integration](https://scrapfly.io/docs/mcp/integrations/openai) |
| **n8n** | [n8n Integration](https://scrapfly.io/docs/mcp/integrations/n8n) |
| **Make** | [Make Integration](https://scrapfly.io/docs/mcp/integrations/make) |
| **Zapier** | [Zapier Integration](https://scrapfly.io/docs/mcp/integrations/zapier) |

> 📖 **All integrations**: [Integration Index](https://scrapfly.io/docs/mcp/integrations)

---

## 📚 Resources

- 🌐 [MCP Cloud Landing Page](https://scrapfly.io/mcp-cloud) — Product overview & features
- 🎮 [Live n8n Demo](https://scrapfly.io/mcp-cloud/n8n) — Try it in your browser
- 📖 [Full Documentation](https://scrapfly.io/docs/mcp/getting-started)
- 🛠️ [Tools Reference](https://scrapfly.io/docs/mcp/tools)
- 💡 [Examples & Use Cases](https://scrapfly.io/docs/mcp/examples)
- ❓ [FAQ](https://scrapfly.io/docs/mcp/faq)
- 🔐 [Authentication Guide](https://scrapfly.io/docs/mcp/authentication)

---

## 💬 Need Help?

- 📖 [Scrapfly Documentation](https://scrapfly.io/docs)
- 📧 [Contact Support](https://scrapfly.io/docs/support)
- 🌐 [Model Context Protocol](https://modelcontextprotocol.io)

---

<p align="center">
  <a href="https://scrapfly.io">
    <img src="https://avatars.githubusercontent.com/u/54183743?s=400&u=5279c1aaea18805aa5cc4fec1053ac2a2cfaac5d&v=4" alt="Scrapfly" width="50"/>
  </a>
  <br/>
  <strong>Made with ❤️ by <a href="https://scrapfly.io">Scrapfly</a></strong>
  <br/>
  <sub>The Web Scraping API for Developers</sub>
</p>
