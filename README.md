<div align="right">
  <a href="README.md">English</a> | <a href="README.zh.md">中文</a>
</div>

# better-edit-tools

> A high-performance MCP (Model Context Protocol) file editing toolkit — atomic writes, smart batch sorting, and intelligent function-scope detection.

## Tools

| Tool | Description |
|------|-------------|
| `be-balance` | Check bracket/brace/parenthesis matching, HTML/XML tag closure, and quote parity in a file. Supports three modes: `aggregate`, `unbalanced` (default), `tree`. |
| `be-show` | Display file content with line numbers. `end` can be a line number or `"auto"` to auto-expand to the enclosing function scope. |
| `be-replace` | Replace a range of lines in a file. |
| `be-insert` | Insert content after a given line (`line=0` inserts at the beginning). |
| `be-delete` | Delete line(s) — single line, range, or batch by JSON array of line numbers. |
| `be-batch` | Batch edit a file (or multiple files) with multiple operations in one call. Operations are applied bottom-up to avoid line-number drift. |
| `be-write` | Write raw content to file(s). Supports both single-file `{"file","content"}` and multi-file `{"files":[...]}`. When standard JSON parsing fails (e.g. unescaped backticks or `${}`), falls back to a state-machine-based degraded extractor. |
| `be-func-range` | Find the start/end lines of the `{}` block or function enclosing a given line, using brace counting with string/comment awareness. |
| `be-tag-range` | Find the start/end lines of the XML/HTML/Vue tag pair enclosing a given line. |

## Design highlights

- **Atomic writes**: File modifications go through a temp-file-then-rename cycle, preventing data corruption if the process crashes mid-write.
- **Smart batch sorting**: Batch edits are automatically sorted from bottom to top, so you never have to worry about line-number offsets.
- **isError signaling**: Errors are properly reported with `isError: true` per the MCP spec.
- **Pure Rust**: No runtime dependencies — a single, statically linked binary.
- **Fault-tolerant JSON parsing**: AI-generated file content often contains backticks, `${}`, or unescaped quotes that break standard JSON encoding. The `write` tool automatically falls back to character-level extraction, so formatting errors don't block file writes.


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

## Acknowledgements

The `replace`, `insert`, `delete`, and `batch` operations are inspired by [includewudi/fast-edit](https://github.com/includewudi/fast-edit).

## License

MIT
