<div align="right">
  <a href="README.md">English</a> | <a href="README.zh.md">中文</a>
</div>

# better-edit-tools

> A high-performance MCP (Model Context Protocol) file editing toolkit — atomic writes, smart batch sorting, and intelligent function-scope detection.
> Experimental project: tool names, parameters, and behaviors may change as the design evolves. Do not hardcode specific tool names into prompts; prefer capability-based or dynamically resolved tool selection.
> Tool descriptions are localized at startup via `--lang <zh|en>` and fall back to the `LANG` environment variable when omitted.

## Tools

### `be-balance`

`be-balance` is the guardrail tool for structural sanity checks. It verifies bracket, brace, and parenthesis matching, plus HTML/XML tag closure and quote parity. The scanner is designed to be practical for source files: it supports `aggregate`, `unbalanced` (default), and `tree` modes so you can choose between a quick summary, a focused failure list, or a nested structure view.

── Catch structural mistakes early, even when the file mixes code, markup, and strings.

### `be-show`

`be-show` is a read-only inspection tool for source navigation. It prints file content with line numbers, and its `end` parameter can either be an explicit line number or `"auto"` to expand to the enclosing function scope. That makes it useful when you want context without manually calculating line ranges.

── Read the exact slice you need without guessing the enclosing function range.

### `be-replace`

`be-replace` performs a precise line-range substitution. It is meant for surgical edits where you already know the exact span to change, and it keeps the `target` and `preview` flow consistent with the other editing tools. Compared with batch editing, it is the simplest option when a single contiguous block must be replaced.

── Replace a known block with minimal movement and minimal surprise.

### `be-insert`

`be-insert` adds content after a specified line, with `line=0` reserving the very beginning of the file. It is the preferred primitive for incremental edits because it avoids rewriting unrelated lines while still working with the shared `target` and `preview` parameters.

── Insert new content exactly where it belongs without touching the rest of the file.

### `be-delete`

`be-delete` removes one line, a line range, or a batch of line numbers supplied as JSON. The tool is intentionally flexible for cleanup tasks, but still stays line-oriented so the resulting change is predictable and easy to reason about.

── Remove cleanup targets with line-level precision and predictable results.

### `be-batch`

`be-batch` is the multi-operation editing entry point. It can apply several edits in one call, including edits across multiple files, and it processes them from bottom to top to prevent line-number drift. This makes it the best choice when a change set would otherwise require several separate tool calls.

── Apply a whole edit plan in one pass while keeping line numbers stable.

### `be-write`

`be-write` is the raw file write tool for full-content replacement. It accepts both single-file payloads and multi-file payloads, and it has a degraded parsing path for AI-generated content that does not survive strict JSON encoding, such as backticks, `${}`, or unescaped quotes. That fallback exists so content generation failures do not block the actual write.

── Even when the JSON wrapper breaks, the write still tries to rescue the payload.

### `be-func-range`

`be-func-range` detects the enclosing `{}` block or function range for a given line. It uses brace counting with string- and comment-aware scanning, which makes it more reliable than a naïve delimiter search on real-world source code.

── Find the logical function boundary instead of guessing from raw braces alone.

### `be-tag-range`

`be-tag-range` finds the enclosing XML/HTML/Vue tag pair for a line. It is the counterpart to `be-func-range` for markup-oriented files and is useful when you need the logical container rather than a raw line span.

── Locate the surrounding tag pair that defines the real editing boundary.

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

### Run

```bash
./target/release/better-edit-tools --lang en
```

If `--lang` is not provided, the server tries to infer the language from the `LANG` environment variable and defaults to English.

### Register in an MCP client

Add to your MCP client configuration:

```json
{
  "mcpServers": {
    "better-edit-tools": {
      "command": "/path/to/better-edit-tools/target/release/better-edit-tools",
      "args": ["--lang", "en"]
    }
  }
}
```

For example, Claude Desktop's config is at `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows).

## Acknowledgements

The `replace`, `insert`, `delete`, and `batch` operations are inspired by [includewudi/fast-edit](https://github.com/includewudi/fast-edit).

## License

MIT
