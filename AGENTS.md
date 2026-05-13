# better-edit-tools — AGENTS.md

## 构建

```bash
go build -o better-edit-tools ./cmd/better-edit-tools
```

Go 版本要求：Go 1.26+。

## 发布

打 `v*` tag 推送即触发 GitHub Actions 自动构建并创建 Release：

```bash
git tag v0.2 && git push origin v0.2
```

Workflow 在 `.github/workflows/build.yml`，注意 job 级需声明 `permissions: contents: write` 才能创建 Release。
Release 产物会同时提供 Linux/macOS 的 amd64/arm64 包和对应 SHA-256 校验文件。
Release notes 按 Conventional Commits 风格生成 changelog，提交时请尽量写清 `type(scope)!: subject`，这样自动分组更准确。

### 提交模板

建议统一使用下面的格式：

```text
feat(parser): add release note grouping
fix(server)!: drop legacy stdout protocol
docs: update build instructions
refactor(edit): simplify batch execution
test(release): cover changelog parser
chore: bump workflow dependencies
```

- `type` 保持小写，优先使用 `feat`、`fix`、`docs`、`refactor`、`test`、`perf`、`build`、`ci`、`chore`、`revert`
- `scope` 可选，但涉及子模块时建议补上
- `!` 表示 breaking change
- `subject` 用祈使句，尽量短，避免把正文塞进标题

## 安装

用户可用 `scripts/install.sh` 从 GitHub latest release 下载静态二进制：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/conglinyizhi/better-edit-tools-mcp/main/scripts/install.sh)
```

二进制安装到 `~/.local/share/better-edit-tools/bin/`。
脚本会自动按当前系统和架构选择资产，并校验 SHA-256 后再解压。

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
cmd/
└── better-edit-tools/main.go     # 启动入口：CLI 解析后进入 server.Run()
internal/
├── app/                          # 命令行和语言协商
├── edit/                         # 核心编辑库（show/replace/insert/delete/batch/write）
└── server/                       # MCP/stdio 适配层
```

关键入口点：
- `cmd/better-edit-tools/main.go:main()` — 解析 CLI 参数后调用 `server.Run(cfg)`
- `internal/server/server.go:Server` — 统一处理 stdio JSON-RPC、工具注册和工具调用
- `internal/edit/*.go` — 每个编辑原语对应一个可直接嵌入 Go agent 框架调用的库函数

## 工具说明

9 个 MCP tools：
- `be-balance`
- `be-read`
- `be-replace`
- `be-insert`
- `be-delete`
- `be-batch`
- `be-write`
- `be-func-range`
- `be-tag-range`

MCP 工具对外返回 JSON 字符串内容；错误时响应体会包含 `isError: true`。内部编辑逻辑统一使用 Go 错误返回。

## 注意事项

- **实验性项目**：工具名称、参数和行为可能继续调整。不要在 prompt 或自动化脚本里写死具体工具名，优先使用能力描述或动态解析的方式选择工具。
- **多语言描述**：工具名和参数保持稳定，`--lang <zh|en>` 只影响 tool description 和文档文本，不改变执行语义。
- **测试**：修改编辑逻辑和协议层时，优先补 Go 测试，不要只靠手工验证。
- **原子写入**：`internal/edit.WriteFileAtomic()` 先写临时文件再 rename，防崩溃
- **JSON 降级解析**：`internal/edit.Write()` 先尝试标准 JSON，失败则调用降级提取逻辑。

- **语言协商**：启动时可通过 `--lang <zh|en>` 指定 tool description 语言；未指定时回退到 `LANG` 环境变量，最终默认英文
- **受限于文件系统**：所有工具操作本地文件，无网络/数据库能力
- **不在 OpenCode plugin 内**：这是标准 MCP 服务器，与 `@opencode-ai/plugin` 无关（原始 TypeScript 版才依赖那个包）
- **提交规范**：以后提交信息使用约定式提交（Conventional Commits），例如 `feat: ...`、`fix: ...`、`docs: ...`、`chore: ...`、`refactor: ...`、`test: ...`、`perf: ...`。这样 GitHub Actions 可以自动按类型生成 changelog。
