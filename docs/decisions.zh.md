# 历史决策记录

> 本文档面向后续开发者、维护者和 AI Agent。如果你计划增加新功能、修改现有行为或提起新 issue，请先阅读本文，避免重复讨论已经关闭或已有结论的问题。
>
> 文档按「我想做 XXX」组织，每条决策都附上了对应的 issue / commit 证据。

---

## 我想增加 `be-batch` / 批量编辑工具

根据 [issue #13](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/13)、[#24](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/24)、[#35](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/35)、[#54](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/54) 以及 commit `8ae2b2f`，我们决定**不保留 `be-batch` 这个 MCP 工具**。

原因：

- `be-batch` 的 `spec` 是 JSON string 而非结构化参数，模型在构造 JSON string 时非常容易出错（tool confusion）。
- 维护者实测数据：3 次调用全部失败，全部卡在 spec 的 JSON string 构造上。
- 同样的场景可以用 `be-read` / `be-replace` / `be-insert` / `be-write` 串行完成，低阶工具在实际工作流中覆盖了 batch 原本要做的所有事。
- 当前一次 tool call 中模型可以发起多次 function call，不需要把所有操作塞进一个 JSON string。

因此，README 和 Go API 文档中残留的 `be-batch` 描述已在 v0.11.0 清理完毕（[#54](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/54)）。

如果你确实需要批量编辑，请使用单工具组合，或考虑在 CLI 模式下用脚本编排（v0.11.0 已提供 CLI 子命令）。

---

## 我想简化或扩展 `be-delete` / `be-insert` 的参数

根据 [issue #35](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/35) 以及 commit `8ae2b2f`，我们决定**保持参数精简，不轻易增加别名**。

当前约定：

- `be-delete`：只保留 `start` / `end` + `target`，已移除 `start_line` / `end_line` / `line` / `lines` 等别名。
- `be-insert`：只保留 `after_line`（CLI 中为 `--after-line`，兼容 `--after`），已移除隐含 -1 换算的 `line` 参数。

原因：

- 同一件事有太多表达方式会让模型产生 tool confusion（[issue #35](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/35) 的核心观点）。
- 参数别名越多，文档、测试、schema 维护成本越高。

如果你想新增参数别名，需要有非常强的证据表明当前参数确实让模型频繁出错，且新参数能显著降低错误率。

---

## 我想让工具名更短或改名

根据 [issue #2](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/2)，我们决定**工具名统一使用 `be-` 前缀**，并保持名称稳定。

当前工具名：

- `be-read`、`be-replace`、`be-insert`、`be-delete`、`be-write`
- `be-balance`、`be-func-range`、`be-tag-range`
- `be-trx-commit`、`be-trx-rollback`、`be-trx-status`
- `be-insert-chip`

原因：

- 前缀统一后更短，便于模型阅读和选择。
- 工具名是对外协议的一部分，改名是 breaking change，需要同步文档和所有调用方。
- 项目 README 已声明"实验性项目"，但后续应尽量避免无意义改名。

如果你想改名，请在 issue 中说明：当前名字具体造成了什么问题、新名字能降低多少混淆、是否值得 breaking change。

---

## 我想增加格式化 / 类型检查 / test hook 等后处理

根据 [issue #1](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/1) 的讨论，我们决定**不引入依赖外部环境的 post-edit validation hooks**。

原因：

- 格式化（gofmt、prettier 等）、类型检查、测试运行都依赖项目外部的工具链和语言运行时。
- 引入这些会显著增加项目复杂度和耦合度，违背"单二进制、零依赖"的设计目标。
- 这些工作更适合交给 Agent 工作流本身或 MCP client 侧的工具链去完成。

如果你希望增加某种 validation，请说明它为什么必须由 better-edit-tools 内部完成，而不是由调用方完成。

---

## 我想把工具描述 / InputSchema 外置化

根据 [issue #48](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/48)，我们决定**当前阶段不把工具元数据外置到配置文件**。

原因：

- 当前只有 8 个工具，InputSchema 和 description 变动不频繁。
- 外置化会增加加载校验、向后兼容和分发复杂度，收益不足以抵消成本。
- 项目仍处于实验性阶段，工具参数可能继续调整，过早做复杂配置机制是过度设计。

作为折中，v0.11.0 已经把**翻译文案**外置到 `internal/server/i18n/*.json` 并通过 `//go:embed` 嵌入（[#52](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/52)），这样改文案不需要改 Go 源码，但 schema 仍保留在代码中。

如果你认为应该外置 schema，请提供证据：工具数量是否已显著增加、是否有真实场景需要运行时覆盖 schema。

---

## 我想运行时覆盖或禁用某个工具

根据 [issue #49](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/49)，我们决定**不实现运行时工具覆盖 / 禁用机制**。

原因：

- 当前只有 8 个工具，规模不大，不需要复杂的运行时配置。
- 禁用核心工具可能导致 LLM 工作流断裂（例如禁用 `be-read` 后 `viewed_code_id` 机制失效）。
- 这属于过度设计，和项目当前阶段不匹配。

如果你确实需要禁用工具，可以在 MCP client 侧配置，这是更合理的分层。

---

## 我想把 `listTools` 改成工厂模式

根据 [issue #50](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/50)，我们决定**不把 `listTools` 重构为 Tool Factory**。

原因：

- 在 [#48](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/48) / [#49](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/49) 已关闭的前提下，工厂模式的主要收益（支撑外置配置）已经不存在。
- 当前 `server.go` 中工具定义虽然集中，但规模可控，改为工厂模式会增加不必要的抽象层。
- 保持简单直接更符合项目当前阶段。

如果未来工具数量显著增加或需要支持多组 schema，可以再重新评估。

---

## 我想让事务 / 快照和编辑工作流深度结合

根据 [issue #53](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/53)，我们决定**不在工具描述中过度强调事务 / 快照工作流**。

原因：

- `be-trx-rollback` 依赖内存中的 snapshot 队列，其可靠性受外部因素影响（例如文件被外部修改后 rollback 可能不一致）。
- snapshot 队列有 `MaxSnapshots = 30` 容量限制，且会 evict 最旧的快照，返回的 event_id 可能很快失效。
- 过度承诺 rollback 的可靠性会误导 LLM。
- 当前工具描述已包含基本功能说明，保持现状即可。

如果你希望改进事务机制，建议先解决 snapshot 的持久化 / 跨进程问题，而不是改描述。

---

## 我想增加 `be-apply` 或原子多文件编辑

根据 [issue #24](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/24)，我们决定**不实现 `be-apply` 工具**，并计划在需要时用"单调用两阶段提交"的完全不同设计重新审视。

原因：

- `be-apply` 的提案和 `be-batch` 有重叠，而当时 `be-batch` 正准备删除。
- 原子多文件编辑的复杂度高，需要全新的设计思路，不是简单扩展现有工具。

如果你对此有强烈需求，请先说明你的场景为什么无法通过现有工具组合 + snapshot 回滚来满足。

---

## 我想增加 CLI 子命令

根据 [issue #55](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/55)，我们决定**增加 8 个核心编辑工具的 CLI 子命令**（v0.11.0）。

已实现的命令：

- `read`、`replace`、`insert`、`delete`、`write`
- `balance`、`func-range`、`tag-range`

设计要点：

- 不带子命令时仍启动 MCP server，保持兼容。
- 不引入 `cobra` / `urfave/cli` 等外部依赖，继续手写 flag parser。
- CLI 参数尽量与 MCP schema 保持一致。
- `--output json` 输出与 Go API 对应的结构化 JSON。
- `viewed_code_id` 和事务 / 快照工具为 MCP-only，因为依赖进程内 session 状态。

注意：CLI 模式不是专门为 Pi agent 设计的。Pi 等 agent 直接生成精确 shell 命令容易出错（引号、空格、命令替换等，不过准确来说这多数是 LLM 本身的问题──无法正确的保证 JSON 生成有效。这个问题在指令上出现问题我们设计的二进制软件无法拦截，因为在 bash/shell 层会被识别为多条指令等问题），如果需要 Pi 集成，请优先写 Pi skill 文档或增加更适合 Pi 的高层包装命令，而不是要求废弃 CLI。

我个人推荐优先使用 MCP 包装的方式接入，尤其是对于 CLI 编辑代码的场景和 Pi Agent 场景。

---

## 我想让 `install.sh` 自动配置 MCP 客户端

根据 [issue #46](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/46) 的讨论，我们决定**install.sh 只负责下载、校验、放置二进制，并把绝对路径和 `--lang` 参数返回给用户 / agent**，不主动修改用户的 MCP 客户端配置文件。

原因：

- 自动修改用户配置文件不安全，且用户看不到 agent 的行为。
- 不同 MCP 客户端的配置路径和格式差异大，且变化快，维护成本高。
- 各 agent 通常有自己的 system prompt 或 skill 来指导如何配置 MCP，这部分应交给 agent 自己处理。

v0.11.0 新增了 `docs/llm-setup-guide.zh.md` / `docs/llm-setup-guide.md`，作为给 AI Agent 看的工作流文档。

---

## 我想给 Pi agent 做专门集成

目前**没有专门为 Pi 做集成**，但您可以用 skill 或高层 CLI 命令的形式改进。

已知问题：

- Pi 直接生成精确 shell 命令容易出错（引号、空格、`$()`、反引号等 shell 元字符），同时因为这会在 shell 层识别为其他指令，我们无法通过软件层面干预……？
- CLI 当前缺少 `--content-file` / `--old-file` 这类避开 shell 引号问题的参数。

如果你希望改进 Pi 集成，建议方向：

1. 先写 `docs/pi.md` skill 文档，教会 Pi 使用现有 CLI 的工作流。
2. 如果 Pi 仍然困难，再考虑增加 `--content-file` 或更自然的"old / new"高层命令。

不要因此废弃整个 CLI 功能：CLI 对 shell 脚本和普通终端用户仍然有价值。

---

## 我想修改 `--lang` 的默认值或自动检测逻辑

根据 [issue #45](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/45)，我们决定：

- 保持当前 `--lang` 默认回退到 `LANG` 环境变量、最终默认英文的行为。
- 在 README 顶部增加显式提示，提醒中文用户手动添加 `"args": ["--lang", "zh"]`。
- `install.sh` 会检测 `LANG`，中文系统下默认输出的配置示例包含 `--lang zh`。

原因：

- 默认英文是对全球用户最安全的假设。
- 中文用户不应依赖自动检测，因为 `LANG` 不一定反映 MCP client 的语言环境。
- 但如果您有更好的策略，欢迎发起 issue

---

## 我想修改版本号策略或 Release 流程

目前该项目处于高度迭代状态，因此我们暂时不打算迭代主版本号（即 major）

根据 `.github/workflows/build.yml` 和 commit 历史，当前策略：

- 版本号保存在 `internal/app/lang.go` 的 `Version` 常量中。
- 推送 `v*` tag 触发 GitHub Actions，自动构建 6 个平台包并创建 Release。
- Release notes 按 Conventional Commits 风格自动生成。

版本号规则：

- `0.MINOR.PATCH`
- 新增功能升级 minor（如 CLI 子命令、i18n 外置化 → v0.11.0）。
- bug 修复升级 patch。
- major 版本保留给未来稳定的 1.0 发布。

---

## 如果你要提新 issue

请先检查：

1. 你的建议是否和本文档中的某条历史决策冲突？
2. 你是否能提供新的证据（实测数据、具体场景、错误日志）说明历史决策需要重新审视？
3. 你的建议是否和项目"单二进制、零依赖、精确编辑"的核心定位一致？

如果只是想重新开启已被关闭的 issue，请在新的 issue 中明确引用历史 issue 编号，并说明为什么现在的情况和当时不同。
