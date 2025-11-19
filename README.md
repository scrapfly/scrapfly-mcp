## Getting Started

### Quick Install

Click one of the buttons below to install the MCP server in your preferred IDE:

[![Install in VS Code](https://img.shields.io/badge/Install_in-VS_Code-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://vscode.dev/redirect/mcp/install?name=scrapfly-cloud-mcp&config=%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D)
[![Install in VS Code Insiders](https://img.shields.io/badge/Install_in-VS_Code_Insiders-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=scrapfly-cloud-mcp&config=%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D&quality=insiders)
[![Install in Visual Studio](https://img.shields.io/badge/Install_in-Visual_Studio-C16FDE?style=flat-square&logo=visualstudio&logoColor=white)](https://vs-open.link/mcp-install?%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D)
[![Install in Cursor](https://img.shields.io/badge/Install_in-Cursor-000000?style=flat-square&logoColor=white)](https://cursor.com/en/install-mcp?name=scrapfly-cloud-mcp&config=eyJuYW1lIjoic2NyYXBmbHktY2xvdWQtbWNwIiwidHlwZSI6Imh0dHAiLCJ1cmwiOiJodHRwczovL21jcC5zY3JhcGZseS5pby9tY3AifQ==)

### Manual Installation

**Standard config** works in most tools:

```js
{
  "servers": {
    "scrapfly-cloud-mcp": {
      "type": "http",
      "url": "https://mcp.scrapfly.io/mcp"
    }
  }
}
```

<details>
<summary>VS Code</summary>

#### Click the button to install:

[![Install in VS Code](https://img.shields.io/badge/Install_in-VS_Code-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://vscode.dev/redirect/mcp/install?name=scrapfly-cloud-mcp&config=%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D)

#### Or install manually:

Follow the MCP install [guide](https://code.visualstudio.com/docs/copilot/chat/mcp-servers#_add-an-mcp-server), use the standard config above. You can also install the scrapfly-cloud-mcp MCP server using the VS Code CLI:

```bash
code --add-mcp '{\"name\":\"scrapfly-cloud-mcp\",\"type\":\"http\",\"url\":\"https://mcp.scrapfly.io/mcp\"}'
```

After installation, the scrapfly-cloud-mcp MCP server will be available for use with your GitHub Copilot agent in VS Code.
</details>

<details>
<summary>VS Code Insiders</summary>

#### Click the button to install:

[![Install in VS Code Insiders](https://img.shields.io/badge/Install_in-VS_Code_Insiders-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=scrapfly-cloud-mcp&config=%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D&quality=insiders)

#### Or install manually:

Follow the MCP install [guide](https://code.visualstudio.com/docs/copilot/chat/mcp-servers#_add-an-mcp-server), use the standard config above. You can also install the scrapfly-cloud-mcp MCP server using the VS Code Insiders CLI:

```bash
code-insiders --add-mcp '{\"name\":\"scrapfly-cloud-mcp\",\"type\":\"http\",\"url\":\"https://mcp.scrapfly.io/mcp\"}'
```

After installation, the scrapfly-cloud-mcp MCP server will be available for use with your GitHub Copilot agent in VS Code Insiders.
</details>

<details>
<summary>Visual Studio</summary>

#### Click the button to install:

[![Install in Visual Studio](https://img.shields.io/badge/Install_in-Visual_Studio-C16FDE?style=flat-square&logo=visualstudio&logoColor=white)](https://vs-open.link/mcp-install?%7B%22type%22%3A%22http%22%2C%22url%22%3A%22https%3A%2F%2Fmcp.scrapfly.io%2Fmcp%22%7D)

#### Or install manually:

1. Open Visual Studio
2. Navigate to the GitHub Copilot Chat window
3. Click the tools icon (🛠️) in the chat toolbar
4. Click the + "Add Server" button to open the "Configure MCP server" dialog
5. Fill in the configuration:
   - **Server ID**: `scrapfly-cloud-mcp`
   - **Type**: Select `http/sse` from the dropdown
   - **URL**: `https://mcp.scrapfly.io/mcp`
6. Click "Save" to add the server

For detailed instructions, see the [Visual Studio MCP documentation](https://learn.microsoft.com/visualstudio/ide/mcp-servers).
</details>

<details>
<summary>Cursor</summary>

#### Click the button to install:

[![Install in Cursor](https://img.shields.io/badge/Install_in-Cursor-000000?style=flat-square&logoColor=white)](https://cursor.com/en/install-mcp?name=scrapfly-cloud-mcp&config=eyJuYW1lIjoic2NyYXBmbHktY2xvdWQtbWNwIiwidHlwZSI6Imh0dHAiLCJ1cmwiOiJodHRwczovL21jcC5zY3JhcGZseS5pby9tY3AifQ==)

#### Or install manually:

Go to `Cursor Settings` -> `MCP` -> `Add new MCP Server`. Name to your liking, use `command` type with the command from the standard config above. You can also verify config or add command like arguments via clicking `Edit`.
</details>

<details>
<summary>Claude Code</summary>

Use the Claude Code CLI to add the scrapfly-cloud-mcp MCP server:

```bash
claude mcp add scrapfly-cloud-mcp --url https://mcp.scrapfly.io/mcp
```
</details>

<details>
<summary>Claude Desktop</summary>

Follow the MCP install [guide](https://modelcontextprotocol.io/quickstart/user), use the standard config above.
</details>

<details>
<summary>Codex</summary>

Create or edit the configuration file `~/.codex/config.toml` and add:

```toml
[mcp_servers.scrapfly-cloud-mcp]
url = "https://mcp.scrapfly.io/mcp"
```

For more information, see the [Codex MCP documentation](https://github.com/openai/codex/blob/main/codex-rs/config.md#mcp_servers).
</details>

<details>
<summary>Gemini CLI</summary>

Follow the MCP install [guide](https://github.com/google-gemini/gemini-cli/blob/main/docs/tools/mcp-server.md#configure-the-mcp-server-in-settingsjson), use the standard config above.
</details>

<details>
<summary>OpenCode</summary>

Follow the MCP Servers [documentation](https://opencode.ai/docs/mcp-servers/). For example in `~/.config/opencode/opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "scrapfly-cloud-mcp": {
      "type": "local",
      "command": [

      ],
      "enabled": true
    }
  }
}
```
</details>

<details>
<summary>Windsurf</summary>

Follow Windsurf MCP [documentation](https://docs.windsurf.com/windsurf/cascade/mcp). Use the standard config above.
</details>

### Configuration Details

- **Server Name:** `scrapfly-cloud-mcp`
- **Type:** Remote HTTP Server
- **URL:** `https://mcp.scrapfly.io/mcp`

### Need Help?

For more information about the Model Context Protocol, visit [modelcontextprotocol.io](https://modelcontextprotocol.io).
