<div align="right">
  <a href="README.md">English</a> | <a href="README.zh.md">中文</a>
</div>

# better-edit-tools

> 高性能 MCP（Model Context Protocol）文件编辑工具集 — 原子写入、批量编辑智能排序、函数范围自动检测。

## 工具列表

| 工具 | 说明 |
|------|------|
| `be-balance` | 检查文件中括号/花括号/方括号的成对情况、HTML/XML 标签闭合以及引号奇偶警告。扫描时会尽量忽略字符串与注释中的符号。支持三种模式：`aggregate`（聚合）、`unbalanced`（失衡，默认）、`tree`（树状嵌套）。 |
| `be-show` | 带行号显示文件内容。`end` 可传行号或 `"auto"`，自动扩展到所在函数范围。 |
| `be-replace` | 替换文件中指定行范围的内容。 |
| `be-insert` | 在指定行后插入内容（`line=0` 表示插入到文件开头）。 |
| `be-delete` | 删除行 — 支持单行、范围删除和 JSON 数组批量删除。 |
| `be-batch` | 批量编辑文件（支持多文件），一次调用完成多处修改。操作自动从下往上排序，无需担心行号偏移。 |
| `be-write` | 写入原始内容到文件。支持单文件 `{"file","content"}` 和多文件 `{"files":[...]}` 格式。当标准 JSON 解析失败时（如反引号、`${}` 未转义），自动启用状态机降级提取。 |
| `be-function-range` | 传入文件路径和行号，返回该行所在函数的起止行号（基于花括号计数，支持字符串/注释感知）。 |

## 设计特点

- **原子写入**：文件修改通过临时文件 + 重命名完成，即使进程崩溃也不会损坏源文件。
- **批量编辑智能排序**：批量操作自动从下往上执行，无需手动调整行号顺序。
- **标准错误响应**：严格按照 MCP 规范，错误时返回 `isError: true`，客户端可正确识别。
- **纯 Rust 实现**：无运行时依赖，单二进制分发，启动快、内存低。
- **容错 JSON 解析**：AI 生成复杂文件内容时容易出现转义错误（反引号、`${}`、未转义引号），`write` 工具会自动降级为字符级提取，避免因 JSON 格式问题导致写入失败。


## 使用方法

### 构建

```bash
cargo build --release
```

编译产物在 `target/release/better-edit-tools`。

### 在 MCP 客户端中注册

添加到 MCP 客户端配置中：

```json
{
  "mcpServers": {
    "better-edit-tools": {
      "command": "/path/to/better-edit-tools/target/release/better-edit-tools"
    }
  }
}
```

例如 Claude Desktop 的配置文件位置：
- macOS：`~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows：`%APPDATA%\Claude\claude_desktop_config.json`

## 致谢

工具集中的 `replace`、`insert`、`delete`、`batch` 等操作灵感来自 [includewudi/fast-edit](https://github.com/includewudi/fast-edit)。

## 许可证

MIT
