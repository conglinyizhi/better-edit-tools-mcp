---
title: "CLI Usage"
weight: 26
description: "CLI subcommands for better-edit-tools, including pipe/stdin write support."
---

# CLI Usage

In addition to running as an MCP server, better-edit-tools exposes 8 core editing tools as CLI subcommands. This is useful for shell scripts, Pi Agent, or any workflow that prefers command-line invocation.

## Basic usage

```bash
better-edit-tools <command> [options]
```

Running without a subcommand starts the MCP server over stdio.

## Subcommands

### File operations

- `read`: Read a line range from a file.
- `replace`: Replace a line range.
- `insert`: Insert content after a line.
- `delete`: Delete a line range.
- `write`: Write or overwrite a file.

### Content recognition

- `balance`: Check pairing of brackets, quotes, and tags.
- `func-range`: Detect the function or `{}` block range for a line.
- `tag-range`: Detect the XML/HTML/Vue tag range for a line.

## Common examples

```bash
# Read lines 1-10
better-edit-tools read --file main.go --start 1 --end 10 --output json

# Replace a range
better-edit-tools replace --file main.go --start 5 --end 10 --content "..."

# Insert after line 4
better-edit-tools insert --file main.go --after-line 4 --content "..."

# Delete a range
better-edit-tools delete --file main.go --start 5 --end 10

# Write a file
better-edit-tools write --file main.go --content "package main\n"

# Balance / range detection
better-edit-tools balance --file main.go
better-edit-tools func-range --file main.go --line 12
better-edit-tools tag-range --file index.html --line 8
```

## Pipe and stdin support

To avoid shell quoting issues, some commands support `--content-file` and `--old-file`. Set the path to `-` to read from stdin.

### Replacement content from stdin

```bash
cat new_content.txt | better-edit-tools replace \
  --file main.go --start 5 --end 10 --content-file -
```

### Old content from stdin

```bash
cat old_snippet.txt | better-edit-tools replace \
  --file main.go --start 5 --end 10 --old-file - --content "..."
```

### Write file content from stdin

```bash
cat main.go | better-edit-tools write --file main.go --content-file -
```

### Using a here-document

```bash
better-edit-tools write --file main.go --content-file - <<'EOF'
package main

func main() {}
EOF
```

## Common options

- `--output json`: Output structured JSON matching the Go API result types.
- `--preview`: Return the diff without writing to disk.
- `--brief`: Return a brief result.
- `--lang <zh|en>`: Set tool description language (MCP server mode only).

## Notes

- Session-based features (`viewed_code_id` validation and snapshot/transaction tools) are only available in MCP server mode because they rely on in-process state.
- CLI mode does not depend on `cobra` / `urfave/cli`; it uses a hand-written flag parser to keep the binary self-contained, and its parameters mirror the MCP schema.

{{< issue title="Feedback: CLI usage docs" body="Please describe the issue, missing CLI scenario, or new evidence." labels="docs:talk" text="📝 Open issue" >}}
