# better-edit-tools

> MCP server porting [OpenCode](https://opencode.ai) built-in tools to the [Model Context Protocol](https://modelcontextprotocol.io), usable from any MCP-compatible client (Claude Desktop, Cursor, etc.).

## Tools

| Tool | Description |
|------|-------------|
| `better_edit_balance` | Check bracket/brace/parenthesis matching, HTML/XML tag closure, and quote parity in a file. Supports three modes: `aggregate`, `unbalanced` (default), `tree`. |
| `better_edit_show` | Display file content with line numbers. `end` can be a line number or `"auto"` to auto-expand to the enclosing function scope. |
| `better_edit_replace` | Replace a range of lines in a file. |
| `better_edit_insert` | Insert content after a given line (`line=0` inserts at the beginning). |
| `better_edit_delete` | Delete line(s) — single line, range, or batch by JSON array of line numbers. |
| `better_edit_batch` | Batch edit a file (or multiple files) with multiple operations in one call. Operations are applied bottom-up to avoid line-number drift. |
| `better_edit_function_range` | Find the start/end lines of the function or block enclosing a given line, using brace counting with string/comment awareness. |

> [!NOTE]
> The original OpenCode `fast_edit_paste` and `fast_edit_save_pasted` tools are **not** ported because they depend on OpenCode's internal message storage and clipboard infrastructure, which are not meaningful in a general MCP context.

## Usage

### Build

```bash
cargo build --release
```

The binary will be at `target/release/better-edit-tools`.

### Register in an MCP client

Add to your MCP client configuration:

```json
{
  "mcpServers": {
    "better-edit-tools": {
      "command": "/path/to/better-edit-tools/target/release/better-edit-tools"
    }
  }
}
```

For example, Claude Desktop's config is at `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows).

## Differences from the original OpenCode tools

- **Name prefix**: All tools use the `better_edit_` prefix instead of the original `structure_` / `fast_edit_` prefixes.
- **Language**: Rust instead of TypeScript/Bun. Faster startup and lower memory footprint.
- **Error signaling**: Errors are reported with `isError: true` per the MCP spec, instead of returning an `{ "error": ... }` JSON blob with `isError: false`.
- **Dropped tools**: `fast_edit_paste` (clipboard-dependent) and `fast_edit_save_pasted` (OpenCode-internal storage) are not included.

## License

Apache-2.0
