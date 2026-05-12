<div align="right">
  <a href="README.zh.md">中文</a> | <a href="README.md">English</a>
</div>

# better-edit-tools

> An experimental high-performance MCP (Model Context Protocol) file editing toolkit in Go — atomic writes, smart batch sorting, and intelligent function-scope detection.
> Experimental project: tool names, parameters, and behaviors may change as the design evolves. Do not hardcode specific tool names into prompts; prefer capability-based or dynamically resolved tool selection.
> Tool descriptions are localized at startup via `--lang <zh|en>` and fall back to the `LANG` environment variable when omitted.

If you are a Go developer who wants to embed editing capabilities directly in your agent framework, see the [Go API documentation](docs/go-api/README.md).

## Tools

### `be-balance`

Structural sanity check for brackets, braces, parentheses, HTML/XML tag closure, and quote parity. The scanner avoids interference from strings and comments. The `verbose` parameter controls output detail:

- `false` (default): only outputs unmatched items.
- `true`: outputs all matched pairs.

——Catch structural mistakes early, even when the file mixes code, markup, and strings.

### `be-show`

Read-only source inspection tool. Prints file content with line numbers. Pass a positive `end` value for an explicit range, or `0` or negative to auto-expand to the enclosing function scope. Returns a `session_id` that can be passed to `be-replace` for line-number validation.

——Read the exact slice you need without guessing the enclosing function range.

### `be-replace`

Precise line-range substitution. Accepts a `session_id` parameter from a prior `be-show` to validate that line numbers still match. When `old` is provided, the tool verifies current content before writing and returns an error on mismatch. The server also accepts `old_text` as an alias.

——Replace a known block with minimal movement and minimal surprise.

### `be-insert`

Adds content after a specified line. `line=0` inserts at the very beginning of the file. Also accepts `after_line` as an alias. The preferred primitive for incremental edits — avoids rewriting unrelated lines.

——Insert new content exactly where it belongs without touching the rest of the file.

### `be-delete`

Removes one line, a line range, or a batch of line numbers supplied as JSON. Accepts both `start`/`end` and `start_line`/`end_line` aliases. Line-oriented for predictable, easy-to-reason-about results.

——Remove targets with line-level precision and predictable results.

### `be-batch`

Multi-operation editing entry point. Applies several edits in one call, including edits across multiple files, and processes them bottom-to-top to prevent line-number drift.

——Apply a whole edit plan in one pass while keeping line numbers stable.

### `be-write`

Raw file write tool for full-content replacement. Accepts both single-file and multi-file payloads via direct arguments:

- Single file: `{"file":"...","content":"..."}`
- Multi file: `{"files":[{"file":"...","content":"..."}]}`

A degraded parsing path is automatically invoked when standard JSON parsing fails, rescuing AI-generated content with broken escaping. When `raw: true`, literal `\n` in content is converted to real newlines, solving the double-encoding problem in MCP call chains.

——Even when the JSON wrapper breaks, the write still tries to rescue the payload.

### `be-func-range`

Detects the enclosing `{}` block or function range for a given line. Uses brace counting with string- and comment-aware scanning for reliability on real-world source code.

——Find the logical function boundary instead of guessing from raw braces alone.

### `be-tag-range`

Finds the enclosing XML/HTML/Vue tag pair for a line. The markup-oriented counterpart to `be-func-range`.

——Locate the surrounding tag pair that defines the real editing boundary.

## Design highlights

- **Atomic writes**: File modifications go through a temp-file-then-rename cycle, preventing data corruption if the process crashes mid-write.
- **Smart batch sorting**: Batch edits are automatically sorted bottom-to-top, so you never have to worry about line-number offsets.
- **isError signaling**: Errors properly report `isError: true` per the MCP spec.
- **Go-native**: No runtime dependencies — a single binary with a small embedded editing library.
- **Fault-tolerant JSON parsing**: AI-generated content often contains backticks, `${}`, or unescaped quotes; `be-write` automatically falls back to character-level extraction.
- **Session state bridging**: `be-show` returns a `session_id` that `be-replace` can accept to validate consistent line numbering.
- **Localized descriptions**: `--lang <zh|en>` switches tool description language; parameter names and behavior remain unchanged.

## Usage

### Build

```bash
go build -o better-edit-tools ./cmd/better-edit-tools
```

The binary will be at `./better-edit-tools`.

### Run

```bash
./better-edit-tools --lang en
```

If `--lang` is not provided, the server tries to infer the language from the `LANG` environment variable and defaults to English.

### Register in an MCP client

Add to your MCP client configuration:

```json
{
  "mcpServers": {
    "better-edit-tools": {
      "command": "/path/to/better-edit-tools/better-edit-tools",
      "args": ["--lang", "en"]
    }
  }
}
```

For example, Claude Desktop's config is at `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows).

### Install

The helper script downloads the latest release for your OS and architecture, verifies the SHA-256 checksum, and installs the binary into `~/.local/share/better-edit-tools/bin/`. Pass a tag name to install a specific version.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/conglinyizhi/better-edit-tools-mcp/main/scripts/install.sh)
bash <(curl -fsSL https://raw.githubusercontent.com/conglinyizhi/better-edit-tools-mcp/main/scripts/install.sh) v0.2
```

Release assets are published for Linux, macOS, and Windows on both `amd64` and `arm64`, with matching `.sha256` checksum files. Windows releases are packaged as `.zip` files. Release notes are grouped from Conventional Commits.

## Acknowledgements

The `replace`, `insert`, `delete`, and `batch` operations are inspired by [includewudi/fast-edit](https://github.com/includewudi/fast-edit).

## License

MIT
