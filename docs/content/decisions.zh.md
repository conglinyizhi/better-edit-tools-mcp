---
title: "历史决策记录"
weight: 10
description: "本文档面向后续开发者、维护者和 AI Agent，记录项目的重要历史决策与讨论结论。"
---

# 历史决策记录

> 注意：开发团队优先维护中文文档。英文文档会尽力同步更新，但可能存在延迟。如果中英文内容有差异，请以中文版本为准。

> 本文档面向后续开发者、维护者和AI Agent。如果你计划增加新功能、修改现有行为或提起新issue，请先阅读本文，避免重复讨论已经关闭或已有结论的问题。

该文档由Agent和人类综合维护，问题作为标题，回答作为内容，每条决策都附上了对应的issue / commit证据。您可以尝试将文档交给长上下文大模型进行分析

## 如果你要提新 issue

~~请先检查：~~

~~1. 你的建议是否和本文档中的某条历史决策冲突？~~
~~2. 你是否能提供新的证据（实测数据、具体场景、错误日志）说明历史决策需要重新审视？~~
~~3. 你的建议是否和项目"单二进制、零依赖、精确编辑"的核心定位一致？~~

上面的内容是大模型生成的，为了获取更多的信息，只要友好的提出问题，都可以加入讨论

对于任何（对，任何）下面提到的相关问题，均可以提出issue来加入讨论，GitHub也可以针对某一行发起issue，我们期待您提供更多我们不知道的信息来让这个工具变得更高效可用

如果只是想重新开启已被关闭的issue，请在新的issue中明确引用历史issue编号，并说明为什么现在的情况和当时不同。

{{< issue title="反馈：历史决策记录" body="请描述你发现的问题、想讨论的历史决策或新证据。" text="📝 针对本文档创建 Issue" >}}

---

## 我想增加 `be-batch` / 批量编辑工具

根据[issue #13](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/13)、
[#24](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/24)、
[#35](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/35)、
[#54](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/54)以及commit `8ae2b2f`，
我们决定**不保留`be-batch`这个MCP工具**。

原因：

- `be-batch`的`spec`是JSON string而非结构化参数，模型在构造JSON string时非常容易出错（tool confusion）。
- 我们将该工具落实到Agent（Hermes）实测数据：3次调用全部失败，全部卡在spec的JSON string构造上。
- 同样的场景可以用`be-read` / `be-replace` / `be-insert` / `be-write`串行完成，低阶工具在实际工作流中覆盖了batch
  原本要做的所有事。
- 当前一次tool call中模型可以发起多次function call，不需要把所有操作塞进一个JSON string。

因此，README和Go API文档中残留的`be-batch`描述已在v0.11.0清理完毕（
[#54](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/54)）。

如果你确实需要批量编辑，请使用单工具组合，或考虑在CLI模式下用脚本编排（v0.11.0已提供CLI子命令）。


{{< issue title="反馈：我想增加 `be-batch` / 批量编辑工具" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 我想简化或扩展 `be-delete` / `be-insert` 的参数

根据[issue #35](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/35)以及commit
`8ae2b2f`，我们决定**保持参数精简，不轻易增加别名**。

当前约定：

- `be-delete`：只保留`start` / `end` + `target`，已移除`start_line` / `end_line` / `line` / `lines`
  等别名。
- `be-insert`：只保留`after_line`（CLI中为`--after-line`，兼容`--after`），已移除隐含-1换算的`line`参数。

原因：

- 同一件事有太多表达方式会让模型产生tool confusion（
  [issue #35](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/35)的核心观点）。
- 参数别名越多，文档、测试、schema维护成本越高。

如果你想新增参数别名，需要有非常强的证据表明当前参数确实让模型频繁出错，且新参数能显著降低错误率。


{{< issue title="反馈：我想简化或扩展 `be-delete` / `be-insert` 的参数" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 我想让工具名更短或改名

根据[issue #2](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/2)，我们决定**工具名统一使用
`be-`前缀**，并保持名称稳定。

当前工具名：

- `be-read`、`be-replace`、`be-insert`、`be-delete`、`be-write`
- `be-balance`、`be-func-range`、`be-tag-range`
- `be-trx-commit`、`be-trx-rollback`、`be-trx-status`
- `be-insert-chip`

原因：

- 前缀统一后更短，便于模型阅读和选择。
- 工具名是对外协议的一部分，改名是breaking change，需要同步文档和所有调用方。
- 项目README已声明"实验性项目"，但后续应尽量避免无意义改名。

因为在大模型侧实现不太标准，如果直接将这个工具使用MCP的方式接入，会看到很多可能性，比如……

- be-read（直接写入工具名到工具列表）
- mcp_better_edit_tools_be_read(Hermes行为)
- better-edit-tools_be-read（OpenCode行为）

其他方式获取没问题但名称仍然很长，我们担心`be-read`如果某个agent不进行刻意区分，可能会和agent自带的read工具冲突，因此有意的添加了这个前缀

如果你想改名，请在issue中说明：当前名字具体造成了什么问题、新名字能降低多少混淆、是否值得breaking change（虽然没有到主版本>0的阶段，
但我们仍然略带谨慎的看重兼容性避免更新影响扩展性）……至少给用户一个方便的迁移指南


{{< issue title="反馈：我想让工具名更短或改名" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 我想增加格式化 / 类型检查 / test hook 等后处理

根据[issue #1](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/1)的讨论，我们决定**
不引入依赖外部环境的post-edit validation hooks**。

原因：

- 格式化（gofmt、prettier等）、类型检查、测试运行都依赖项目外部的工具链和语言运行时。
- 引入这些会显著增加项目复杂度和耦合度，违背"单二进制、零依赖"的设计目标。
- 这些工作更适合交给Agent工作流本身或MCP client侧的工具链去完成。

如果你希望增加某种validation，请说明它为什么必须由better-edit-tools内部完成，而不是由调用方完成。


{{< issue title="反馈：我想增加格式化 / 类型检查 / test hook 等后处理" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 我想把工具描述 / InputSchema 外置化

根据[issue #48](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/48)，我们决定**
当前阶段不把工具元数据外置到配置文件**。根据
[issue #49](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/49)，我们决定**
不实现运行时工具覆盖/禁用机制**。

原因：

- 当前只有8个工具，InputSchema和description变动不频繁，并且规模不大，不需要复杂的运行时配置。
- 外置化会增加加载校验、向后兼容和分发复杂度，收益不足以抵消成本。
- 项目仍处于实验性阶段，工具参数可能继续调整，过早做复杂配置机制是过度设计。
- 禁用核心工具可能导致LLM工作流断裂（例如禁用`be-read`后`viewed_code_id`机制失效），此时部分参数大模型可能无法理解。
- 这属于过度设计，和项目当前阶段不匹配。

作为折中，v0.11.0已经把**翻译文案**外置到`internal/server/i18n/*.json`并通过`//go:embed`嵌入（
[#52](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/52)），这样改文案不需要改Go源码，但
schema仍保留在代码中。

如果你想在外置/内置之间做选择，请在issue中说明：你遇到的具体维护痛点是什么、你愿意承担多少额外复杂度。


{{< issue title="反馈：我想把工具描述 / InputSchema 外置化" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 我想把 `listTools` 改成工厂模式

根据[issue #50](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/50)，我们决定**不把
`listTools`重构为Tool Factory**。

原因：

- 在[#48](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/48) /
  [#49](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/49)已关闭的前提下，工厂模式的主要收益（
  支撑外置配置）已经不存在。
- 当前`server.go`中工具定义虽然集中，但规模可控，改为工厂模式会增加不必要的抽象层。
- 保持简单直接更符合项目当前阶段。

如果未来工具数量显著增加或需要支持多组schema，可以再重新评估。


{{< issue title="反馈：我想把 `listTools` 改成工厂模式" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 我想让事务 / 快照和编辑工作流深度结合

根据[issue #53](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/53)，我们决定**
不在工具描述中过度强调事务/快照工作流**。

原因：

- `be-trx-rollback`依赖内存中的snapshot队列，其可靠性受外部因素影响（例如文件被外部修改后rollback可能不一致）。
- snapshot队列有`MaxSnapshots = 30`容量限制，且会删除最旧的快照，返回的event_id可能很快失效。
- 过度承诺rollback的可靠性会误导LLM。
- 当前工具描述已包含基本功能说明，保持现状即可。

如果你希望改进事务机制，建议先解决snapshot的持久化/跨进程问题，而不是改描述。


{{< issue title="反馈：我想让事务 / 快照和编辑工作流深度结合" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 我想增加 `be-apply` 或原子多文件编辑

根据[issue #24](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/24)，我们决定**不实现
`be-apply`工具**，并计划在需要时用"单调用两阶段提交"的完全不同设计重新审视。

原因：

- `be-apply`的提案和`be-batch`有重叠，而当时`be-batch`正准备删除。
- 原子多文件编辑的复杂度高，需要全新的设计思路，不是简单扩展现有工具。

如果你对此有强烈需求，请先说明你的场景为什么无法通过现有工具组合+ snapshot回滚来满足。


{{< issue title="反馈：我想增加 `be-apply` 或原子多文件编辑" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 我想增加 CLI 子命令 / 我想给 Pi agent 做专门集成

根据[issue #55](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/55)，我们决定**增加8
个核心编辑工具的CLI子命令**（v0.11.0）。这也是因为我们开始使用Pi Agent发现官方不支持MCP做出的新功能决策；但即使如此，我们仍然**无法专门为Pi做集成**（
主要因为跨语言Golang和TypeScript的差异），但您可以用skill或高层CLI命令的形式改进。如果有更好的提议欢迎提出issue

已实现的命令：

- 用于编辑文本的：`read`、`replace`、`insert`、`delete`、`write`
- 用于识别文本内容的：`balance`、`func-range`、`tag-range`

设计要点：

- 不带子命令时仍启动MCP server，保持兼容。
- 不引入`cobra` / `urfave/cli`等外部依赖，继续手写flag parser。
- CLI参数尽量与MCP schema保持一致。
- `--output json`输出与Go API对应的结构化JSON。
- `viewed_code_id`和事务/快照工具为MCP-only，因为依赖进程内session状态。

注意：CLI模式不是专门为Pi agent设计的。Pi等agent直接生成精确shell命令容易出错（引号、空格、命令替换等，不过准确来说这多数是LLM本身的问题──无法正确的保证
JSON生成有效。这个问题在指令上出现问题我们设计的二进制软件无法拦截，因为在bash/shell层会被识别为多条指令等问题），如果需要Pi集成，请优先写Pi skill
文档或增加更适合Pi的高层包装命令，而不是要求废弃CLI。

我个人推荐优先使用MCP包装的方式接入，尤其是对于CLI编辑代码的场景和Pi Agent场景。

已知问题：

- Pi直接生成精确shell命令容易出错（引号、空格、`$()`、反引号等shell元字符），同时因为这会在shell层识别为其他指令，我们无法通过软件层面干预……？
- CLI当前缺少`--content-file` / `--old-file`这类避开shell引号问题的参数。

如果你希望改进Pi集成，建议方向：

1. 先写`docs/pi.md` skill文档，教会Pi使用现有CLI的工作流。
2. 如果Pi仍然困难，再考虑增加`--content-file`或更自然的"old / new"高层命令。

kimi 2.7对我们说过的：

> 不要因此废弃整个CLI功能：CLI对shell脚本和普通终端用户仍然有价值。

然后就发生了下面这件事：

在编辑这段落内容的时候我想到一个事情，可以尝试通过管道符的方式避免agent错误的生成符号导致无法解析，但kimi agent提示这仍然需要保证开头和末尾的EOL标志成对，
这我实在无能为力了，于是发起一次功能提交（详见提交`2e4d97a`），尽可能的让CLI也能尽可能的使用这种功能


{{< issue title="反馈：我想增加 CLI 子命令 / 我想给 Pi agent 做专门集成" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 我想让 `install.sh` 自动配置 MCP 客户端

根据[issue #46](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/46)的讨论，我们决定
**install.sh只负责下载、校验、放置二进制，并把绝对路径和`--lang`参数返回给用户/ agent**，不主动修改用户的MCP客户端配置文件。

原因：

- 自动修改用户配置文件不安全，且用户看不到agent的行为。
- 不同MCP客户端的配置路径和格式差异大，且变化快，维护成本高。
- 各agent通常有自己的system prompt或skill来指导如何配置MCP，这部分应交给agent自己处理。

考虑到实际上`install.sh`只对Linux和macOS系统用户友好，后期可能需要提供其他方式来对Windows用户提供相应的安装方式，但也因为上面的原因，
我们不能也无法提供一键安装的方式，不过，v0.11.0新增了`docs/llm-setup-guide.zh.md` / `docs/llm-setup-guide.md`，作为给
AI Agent看的工作流文档。


{{< issue title="反馈：我想让 `install.sh` 自动配置 MCP 客户端" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 我想修改 `--lang` 的默认值或自动检测逻辑

根据[issue #45](https://github.com/conglinyizhi/better-edit-tools-mcp/issues/45)，我们决定：

- 保持当前`--lang`默认回退到`LANG`环境变量、最终默认英文的行为。
- 在README顶部增加显式提示，提醒中文用户手动添加`"args": ["--lang", "zh"]`。
- `install.sh`会检测`LANG`，中文系统下默认输出的配置示例包含`--lang zh`。

原因：

- 默认英文是对全球用户最安全的假设。
- 中文用户不应依赖自动检测，因为`LANG`不一定反映MCP client的语言环境。
- 但如果您有更好的策略，欢迎发起issue

附：这是一台我们开发环境中的中文语言的ArchLinux执行`export | rg LANG`的结果：

```shell
CSF_LANGUAGE=us
LANG=zh_CN.UTF-8
LANGUAGE=''
```

LC的过滤结果没有放出来，是因为完全没有可以放出来的内容(exit 1)


{{< issue title="反馈：我想修改 `--lang` 的默认值或自动检测逻辑" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 我想修改版本号策略或 Release 流程

目前该项目处于高度迭代状态，因此我们暂时不打算迭代主版本号（即major）

但是如果您有对于这个方案的提议，也可以发起issue一起讨论

根据`.github/workflows/build.yml`和commit历史，当前策略：

- 版本号保存在`internal/app/lang.go`的`Version`常量中。
- 推送`v*` tag触发GitHub Actions，自动构建6个平台包并创建Release。
- Release notes按Conventional Commits风格自动生成。

版本号规则：

- `0.MINOR.PATCH`
- 新增功能升级minor（如CLI子命令、i18n外置化→v0.11.0）。
- bug修复升级patch。
- major版本保留给未来稳定的1.0发布。


{{< issue title="反馈：我想修改版本号策略或 Release 流程" body="请描述你的建议、问题或新证据。" labels="docs" text="📝 发起 issue" >}}

---

## 部分版本我没有看到对应的 Release 包

因为CI或者说workflows设计的问题，直到推上去我们才直到是否有问题，也就是github action开始工作，这个时候如果成功就有包，失败就没有包，
这导致我们没有太好的办法在每次提交之前都在本地跑CI模拟流程，我们也在想办法解决