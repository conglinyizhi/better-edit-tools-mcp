---
title: "LLM Self-Setup Guide"
weight: 30
description: "Guide for AI Agents to download, verify, and install the better-edit-tools binary."
---
# LLM Self-Setup Guide

This guide is written for AI agents (Kimi Code, Claude Code, Cline, etc.) that need to make the `better-edit-tools` binary available on the user's machine.

## Goal

Download the correct release binary, verify it, place it in a stable location, and report the absolute path back to the user or to the calling MCP client configuration step.

**Do not modify the user's MCP client configuration file unless the user explicitly asks you to.** Your job is to get the binary installed and verified.

## Workflow

1. **Detect platform**
   - OS: `linux`, `darwin` (macOS), or `windows`
   - Arch: `amd64` or `arm64`
   - On Windows, prefer `%LOCALAPPDATA%\better-edit-tools\bin\better-edit-tools.exe`
   - On Linux/macOS, prefer `${HOME}/.local/share/better-edit-tools/bin/better-edit-tools`

2. **Pick release URL**
   - Base: `https://github.com/conglinyizhi/better-edit-tools-mcp/releases/latest/download/`
   - Archive: `better-edit-tools-${OS}-${ARCH}.tar.gz`
   - Checksums: `checksums.txt`
   - For a specific version, replace `latest` with the tag (e.g. `v0.12.0`).

3. **Download and verify**
   - Download both the archive and `checksums.txt`.
   - Compute SHA-256 of the archive.
   - Compare with the expected hash in `checksums.txt`.
   - Abort and tell the user if the checksum does not match.

4. **Extract and place**
   - Create the install directory if it does not exist.
   - Extract the archive into the install directory.
   - On Linux/macOS, ensure the binary is executable (`chmod +x`).

5. **Report**
   - Return the absolute path to the binary.
   - Mention the `--lang` flag: if the user's MCP client should show Chinese tool descriptions, pass `"--lang", "zh"` in the MCP server `args`.

## Platform Notes

| OS      | Preferred install dir                                   | Binary name             |
|---------|----------------------------------------------------------|-------------------------|
| Linux   | `${HOME}/.local/share/better-edit-tools/bin`             | `better-edit-tools`     |
| macOS   | `${HOME}/.local/share/better-edit-tools/bin`             | `better-edit-tools`     |
| Windows | `%LOCALAPPDATA%\better-edit-tools\bin`                   | `better-edit-tools.exe` |

## Verification Checklist

- [ ] Binary exists at the reported absolute path.
- [ ] `better-edit-tools --version` returns the expected version.
- [ ] If the user uses a Chinese model, remind them to add `--lang zh` in MCP args.
