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
- **100+ Countries** — [Global proxy network](https://scrapfly.io/docs/scrape-api/proxy) with datacenter & residential IPs
- **Anti-Bot Bypass** — [Advanced ASP technology](https://scrapfly.io/docs/scrape-api/anti-scraping-protection) defeats modern protections
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

The Scrapfly MCP Server provides **5 powerful tools** covering 99% of web scraping use cases:

| Tool | Description | Use Case |
|------|-------------|----------|
| `scraping_instruction_enhanced` | Get best practices & POW token | **Always call first!** |
| `web_get_page` | Quick page fetch with smart defaults | Simple scraping tasks |
| `web_scrape` | Full control with browser automation | Complex scraping, login flows |
| `screenshot` | Capture page screenshots | Visual analysis, monitoring |
| `info_account` | Check usage & quota | Account management |

> 📖 **Full reference**: [Tools & API Specification](https://scrapfly.io/docs/mcp/tools)

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
| `-http <address>` | Start HTTP server at the specified address (e.g., `:8080`). Takes precedence over `PORT` env var. |
| `-apikey <key>` | Use this API key instead of the `SCRAPFLY_API_KEY` environment variable. |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `PORT` | HTTP port to listen on. Used if `-http` flag is not set. |
| `SCRAPFLY_API_KEY` | Default Scrapfly API key. Can also be passed via query parameter `?apiKey=xxx` at runtime. |

### Examples

```bash
# Start HTTP server on port 8080
./scrapfly-mcp -http :8080

# Start HTTP server using PORT env var
PORT=8080 ./scrapfly-mcp

# Start with API key
./scrapfly-mcp -http :8080 -apikey scp-live-xxxx

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
