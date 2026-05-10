# better-edit-tools — AGENTS.md

## 构建

```bash
cargo build --release                          # 本地（默认 glibc）
cargo build --release --target x86_64-unknown-linux-musl  # 静态链接
```

Rust 版本要求：edition 2024（需 Rust 1.85+）。

## 发布

打 `v*` tag 推送即触发 GitHub Actions 自动构建并创建 Release：

```bash
git tag v0.1.0 && git push origin v0.1.0
```

Workflow 在 `.github/workflows/build.yml`，注意 job 级需声明 `permissions: contents: write` 才能创建 Release。

## 安装

用户可用 `scripts/install.sh` 从 GitHub latest release 下载静态二进制：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/conglinyizhi/better-edit-tools-mcp/main/scripts/install.sh)
```

二进制安装到 `~/.local/share/better-edit-tools/bin/`。

## MCP 注册

```json
{
  "mcp": {
    "better-edit-tools": {
      "type": "local",
      "command": ["/path/to/better-edit-tools"]
    }
  }
}
```

所有工具通过 stdio 传输暴露，工具名前缀统一为 `be-`。

## 架构

```
src/
├── main.rs                       # 启动入口：CLI 解析后进入 server::run()
├── cli.rs                        # 命令行解析（--help / --version / --lang）
├── lang.rs                       # 语言标签解析与默认语言选择
├── server.rs                     # MCP 服务器 + 工具注册 + 运行时本地化描述
├── fast_edit/
│   ├── mod.rs                    # 公共 API 导出
│   ├── core.rs                   # 核心工具函数（read_lines, write_file_atomic 等）
│   ├── edit.rs                   # 编辑操作（show/replace/insert/delete/batch）
│   ├── write.rs                  # 写入操作 + JSON 降级解析
│   └── func_range.rs             # 函数范围检测
├── error.rs                      # 共享错误类型（EditError / EditResult）
├── structure_balance.rs          # 符号平衡检查核心逻辑
```

关键入口点：
- `main.rs:main()` — 解析 CLI 参数后调用 `server::run(lang).await?`
- `server.rs:OpenCodeTools` — 统一持有工具路由，`list_tools` / `get_tool` 会根据语言覆盖 `Tool.description`
- `server.rs` 的 `#[tool_router]` + `#[tool_handler]` 组合负责工具调用与元数据返回
- `fast_edit::op_show()`, `op_replace()`, 等 — 每个 `pub fn` 对应一个 MCP tool 的后端实现

## 工具说明

8 个 MCP tools，定义在 `main.rs`：
- `better_edit_balance` — 调用 `structure_balance::check_structure_balance()`
- `better_edit_show/replace/insert/delete/batch/write` — 调用 `fast_edit::op_*()`
- `better_edit_function_range` — 调用 `fast_edit::op_function_range()`

MCP 工具对外仍返回 `Result<String, String>`，错误时 MCP 自动设 `isError: true`；内部编辑逻辑统一使用 `EditResult<T>` / `EditError`。

## 注意事项

- **实验性项目**：工具名称、参数和行为可能继续调整。不要在 prompt 或自动化脚本里写死具体工具名，优先使用能力描述或动态解析的方式选择工具。
- **多语言描述**：工具名和参数保持稳定，`--lang <zh|en>` 只影响 tool description 和文档文本，不改变执行语义。
- **无测试**：项目目前没有测试，修改时需手动验证
- **原子写入**：`fast_edit::write_file_atomic()` 先写临时文件再 rename，防崩溃
- **JSON 降级解析**：`op_write` 先尝试 `serde_json::from_str`，失败则调用 `parse_spec_raw` 状态机降级提取。实现在 `src/fast_edit/write.rs`。

- **语言协商**：启动时可通过 `--lang <zh|en>` 指定 tool description 语言；未指定时回退到 `LANG` 环境变量，最终默认英文
- **受限于文件系统**：所有工具操作本地文件，无网络/数据库能力
- **不在 OpenCode plugin 内**：这是标准 MCP 服务器，与 `@opencode-ai/plugin` 无关（原始 TypeScript 版才依赖那个包）
