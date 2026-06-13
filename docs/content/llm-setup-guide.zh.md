---
title: "大模型自配置指南"
weight: 20
description: "帮助 AI Agent 把 better-edit-tools 二进制安装到本地并完成 MCP 配置的流程指南。"
---
# 大模型自配置指南

本文档面向需要帮用户把 `better-edit-tools` 二进制安装到本地的 AI Agent（Kimi Code、Claude Code、Cline 等）。

## 目标

下载对应系统的 Release 二进制、校验 SHA-256、放置到稳定位置，并把绝对路径返回给用户或交给下一步 MCP 配置逻辑。

**除非用户明确要求，否则不要修改用户的 MCP 客户端配置文件。** 你的任务只是把二进制安装好并验证可用。

## 工作流

1. **检测平台**
   - 系统：`linux`、`darwin`（macOS）或 `windows`
   - 架构：`amd64` 或 `arm64`
   - Windows 推荐路径：`%LOCALAPPDATA%\better-edit-tools\bin\better-edit-tools.exe`
   - Linux/macOS 推荐路径：`${HOME}/.local/share/better-edit-tools/bin/better-edit-tools`

2. **选择 Release URL**
   - 基础地址：`https://github.com/conglinyizhi/better-edit-tools-mcp/releases/latest/download/`
   - 包名：`better-edit-tools-${OS}-${ARCH}.tar.gz`
   - 校验文件：`checksums.txt`
   - 如需指定版本，把 `latest` 替换为 tag，例如 `v0.12.0`。

3. **下载并校验**
   - 同时下载包和 `checksums.txt`。
   - 计算包的 SHA-256。
   - 与 `checksums.txt` 中的预期值对比。
   - 校验不通过时中止，并告知用户。

4. **解压并放置**
   - 如安装目录不存在则创建。
   - 将包解压到安装目录。
   - Linux/macOS 下确保二进制有执行权限（`chmod +x`）。

5. **报告结果**
   - 返回二进制的绝对路径。
   - 提醒 `--lang` 参数：如果用户 MCP 客户端需要中文工具描述，在 `args` 中加入 `"--lang", "zh"`。

## 平台说明

| 系统    | 推荐安装目录                                               | 二进制名                 |
|---------|------------------------------------------------------------|--------------------------|
| Linux   | `${HOME}/.local/share/better-edit-tools/bin`               | `better-edit-tools`      |
| macOS   | `${HOME}/.local/share/better-edit-tools/bin`               | `better-edit-tools`      |
| Windows | `%LOCALAPPDATA%\better-edit-tools\bin`                     | `better-edit-tools.exe`  |

## 自检清单

- [ ] 二进制存在于报告的绝对路径。
- [ ] `better-edit-tools --version` 能返回预期版本。
- [ ] 如果用户使用中文模型，提醒其在 MCP 参数中加入 `--lang zh`。
