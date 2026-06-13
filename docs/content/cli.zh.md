---
title: "CLI 使用"
weight: 25
description: "better-edit-tools 的 CLI 子命令说明，包括管道/stdin 写入支持。"
---

# CLI 使用

better-edit-tools 除了作为 MCP server 运行外，还提供 8 个核心编辑工具的 CLI 子命令，适合 shell 脚本、Pi Agent 或偏好命令行调用的场景。

## 基本用法

```bash
better-edit-tools <command> [options]
```

不带子命令时启动 MCP server（stdio）。

## 子命令

### 文件操作

- `read`：读取文件行范围。
- `replace`：替换指定行范围。
- `insert`：在指定行后插入内容。
- `delete`：删除指定行范围。
- `write`：写入或覆盖文件。

### 内容识别

- `balance`：检查括号、引号、标签成对情况。
- `func-range`：检测某行所在的函数或 `{}` 块范围。
- `tag-range`：检测某行所在的 XML/HTML/Vue 标签范围。

## 常用示例

```bash
# 读取 1-10 行
better-edit-tools read --file main.go --start 1 --end 10 --output json

# 替换指定范围
better-edit-tools replace --file main.go --start 5 --end 10 --content "..."

# 在第 4 行后插入
better-edit-tools insert --file main.go --after-line 4 --content "..."

# 删除指定范围
better-edit-tools delete --file main.go --start 5 --end 10

# 写入文件
better-edit-tools write --file main.go --content "package main\n"

# 结构检查 / 范围检测
better-edit-tools balance --file main.go
better-edit-tools func-range --file main.go --line 12
better-edit-tools tag-range --file index.html --line 8
```

## 管道与 stdin 支持

为了避免 shell 引号转义问题，部分命令支持 `--content-file` 和 `--old-file`，并把路径设为 `-` 表示从 stdin 读取。

### 替换内容来自 stdin

```bash
cat new_content.txt | better-edit-tools replace \
  --file main.go --start 5 --end 10 --content-file -
```

### 旧内容来自 stdin

```bash
cat old_snippet.txt | better-edit-tools replace \
  --file main.go --start 5 --end 10 --old-file - --content "..."
```

### 写入文件时内容来自 stdin

```bash
cat main.go | better-edit-tools write --file main.go --content-file -
```

### 通过 here-document

```bash
better-edit-tools write --file main.go --content-file - <<'EOF'
package main

func main() {}
EOF
```

## 通用选项

- `--output json`：输出与 Go API 对应的结构化 JSON。
- `--preview`：只返回 diff，不写入文件。
- `--brief`：返回精简结果。
- `--lang <zh|en>`：设置工具描述语言（仅影响 MCP server 模式）。

## 注意事项

- 基于 session 的特性（`viewed_code_id` 校验、快照/事务工具）仅在 MCP server 模式下可用，因为它们依赖进程内状态。
- CLI 模式不引入 `cobra` / `urfave/cli` 等外部依赖，继续手写 flag parser，因此参数风格与 MCP schema 尽量保持一致。

{{< issue title="反馈：CLI 使用文档" body="请描述你发现的问题、想补充的 CLI 场景或新证据。" labels="docs:talk" text="📝 发起 issue" >}}
