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

所有工具通过 stdio 传输暴露，工具名前缀统一为 `better_edit_`。

## 架构

```
src/
├── main.rs              # MCP 服务器入口 + 工具注册（#[tool_router]）
├── fast_edit.rs         # 文件编辑核心操作（show/replace/insert/delete/batch/function-range）
└── structure_balance.rs # 符号平衡检查核心逻辑
```

关键入口点：
- `main.rs:OpenCodeTools` — 唯一 struct，所有工具通过 `#[tool_router(server_handler)]` 注册
- `main.rs:main()` — `OpenCodeTools.serve(stdio()).await?`
- `fast_edit.rs:op_show()`, `op_replace()`, 等 — 每个 `pub fn` 对应一个 MCP tool 的后端实现

## 工具说明

7 个 MCP tools，定义在 `main.rs`：
- `better_edit_balance` — 调用 `structure_balance::check_structure_balance()`
- `better_edit_show/replace/insert/delete/batch/function_range` — 调用 `fast_edit::op_*()`

所有工具返回 `Result<String, String>`，错误时 MCP 自动设 `isError: true`。

## 注意事项

- **无测试**：项目目前没有测试，修改时需手动验证
- **原子写入**：`fast_edit::write_file_atomic()` 先写临时文件再 rename，防崩溃
- **op_batch 忽略 format 参数**：`_format` 前缀表明暂未使用（为签名兼容保留）
- **中文描述**：所有 tool description 是中文的，rmcp `#[tool]` 宏直接透传
- **受限于文件系统**：所有工具操作本地文件，无网络/数据库能力
- **不在 OpenCode plugin 内**：这是标准 MCP 服务器，与 `@opencode-ai/plugin` 无关（原始 TypeScript 版才依赖那个包）
