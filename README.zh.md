<div align="right">
  <a href="README.md">English</a> | <a href="README.zh.md">中文</a>
</div>

# better-edit-tools

> 实验性项目：工具名称、参数和行为都可能随着设计继续调整。不要把具体工具名写死到 prompt 里，优先使用能力描述或动态解析的方式选择工具。

> 一个高性能的 MCP（Model Context Protocol）文件编辑工具集 —— Go 原生实现，提供原子写入、批量编辑智能排序、函数范围自动检测。旨在减少大模型修改文件产生的 token 数量，并尽可能挽回因参数错误导致的各种问题。
> 工具描述在启动时通过 `--lang <zh|en>` 本地化；不传则回退到 `LANG` 环境变量，最终默认英文。

如果你是 Go 开发者，想在 Agent 框架中直接嵌入编辑能力，请参见 [Go API 文档](docs/go-api/README.zh.md)。

## 工具说明

### `be-balance`

检查括号、花括号、方括号、HTML/XML 标签闭合以及引号是否成对。扫描时避开字符串与注释中的干扰符号。`verbose` 参数控制输出详细程度：

- `false`（默认）：只输出不匹配项
- `true`：输出全部匹配对

——即使混着代码、标记和字符串，也能把结构问题揪出来。

### `be-show`

只读查看工具，按行号展示文件内容。`end` 用正数指定结束行，传 `0` 或负数则自动扩展到所在函数范围。适合在不手动计算区间的情况下快速获取上下文。返回 `viewed_code_id`（v0.4+），可用于后续 `be-replace` 的行数校验。

——不用自己算范围，就能看到最需要的那一段上下文。

### `be-replace`

精确替换指定行范围的内容。支持通过 `viewed_code_id` 参数关联 `be-show` 的查询结果，自动校验行数是否一致。传入 `old` 时会校验当前文件内容是否与旧内容一致，不一致时直接返回错误。服务端也接受 `old_text` 作为别名。

——已知目标区间时，直接做最小范围精确替换。

### `be-insert`

在指定行后插入内容，`line=0` 表示插入到文件开头。服务端也接受 `after_line` 作为同义参数。增量编辑中最直接的原语，减少对原有内容的扰动。

——把新内容插到准确位置，尽量不扰动原文件。

### `be-insert-chip`

从文件（`file:///绝对路径`）或 chip 缓存（`chip://{id}`）读取内容并插入到指定行。不传 `from` 时列出所有可用的 chip ID。本工具桥接了失败操作与恢复之间的断层：当 `be-write` 因 JSON 格式异常失败并将参数保存为 chip 后，`be-insert-chip` 可将该内容回放到目标文件中。详细流程见 [Chip 恢复](#chip-恢复) 章节。

——从文件或失败缓存回放内容，精确插入到指定位置。

### `be-delete`

删除单行、行范围或 JSON 数组指定的多行。范围删除时同时接受 `start`/`end` 和 `start_line`/`end_line` 两种写法。始终围绕行粒度工作，强调操作的可预测性。

——行级精度的删除，结果可控。

### `be-batch`

一次性执行多步编辑的入口，支持单文件和多文件组合。自动从下往上应用操作，避免行号偏移带来的连锁错误。

——一次提交多处修改，避免行号漂移。

### `be-write`

原始写入工具，直接写入完整文件内容。

- 单文件：`{"file":"...","content":"..."}`
- 多文件：`{"files":[{"file":"...","content":"..."}]}`

标准 JSON 解析失败时自动切换到状态机式的降级提取逻辑，最大程度保证 AI 生成内容顺利落盘。`raw: true` 时 content 中的字面 `\n` 会转换为真正的换行符，解决 MCP 链路上 JSON 双重转义导致多行内容变一行的问题。

——就算大模型输出的 JSON 出错，也会尽量把这次长输出救回来。

### `be-func-range`

定位某一行所属的 `{}` 块或函数范围。基于花括号计数，兼顾字符串和注释环境，比简单的分隔符查找更适合真实源码。

——找到的不是随便一个花括号，而是真正的函数边界。

### `be-tag-range`

查找某一行所在的 XML/HTML/Vue 标签配对范围。相当于面向标记语言的范围定位工具。

——直接定位标签对，把编辑边界收敛到真正的容器范围。

## 设计特点

- 原子写入：文件修改通过临时文件 + 重命名完成，进程崩溃也不会损坏源文件。
- 批量编辑智能排序：批量操作自动从下往上执行，无需手动调整行号顺序。
- 标准错误响应：严格按 MCP 规范，错误时返回 `isError: true`。
- Go 原生实现：无运行时依赖，单二进制分发，启动快、易嵌入。
- 容错 JSON 解析：AI 生成内容出现转义错误时，`be-write` 自动降级为字符级提取，避免 JSON 格式问题导致写入失败。
- Session 状态桥接：`be-show` 返回 `viewed_code_id`，`be-replace` 可传入校验行数一致性。
- 多语言描述：`--lang <zh|en>` 切换工具描述语言；参数名和行为不变。

## 使用方法

### 构建

```bash
go build -o better-edit-tools ./cmd/better-edit-tools
```

编译产物在 `./better-edit-tools`。

### 运行

```bash
./better-edit-tools --lang zh
```

不传 `--lang` 时从 `LANG` 环境变量推断，失败则默认英文。

### 在 MCP 客户端中注册

添加到 MCP 客户端配置：

```json
{
  "mcpServers": {
    "better-edit-tools": {
      "command": "/path/to/better-edit-tools/better-edit-tools",
      "args": ["--lang", "zh"]
    }
  }
}
```

Claude Desktop 配置文件位置：

- macOS：`~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows：`%APPDATA%\Claude\claude_desktop_config.json`

### 安装

安装脚本根据当前系统和架构自动下载最新 Release，校验 SHA-256 后安装到 `~/.local/share/better-edit-tools/bin/`。也可传 tag 参数安装指定版本。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/conglinyizhi/better-edit-tools-mcp/main/scripts/install.sh)
bash <(curl -fsSL https://raw.githubusercontent.com/conglinyizhi/better-edit-tools-mcp/main/scripts/install.sh) v0.2
```

Release 产物提供 Linux、macOS、Windows 的 `amd64`/`arm64` 包和对应的 `.sha256` 校验文件。Windows 使用 `.zip` 打包。Release notes 按 Conventional Commits 分组生成。

## 致谢

工具集中的 `replace`、`insert`、`delete`、`batch` 等操作灵感来自 [includewudi/fast-edit](https://github.com/includewudi/fast-edit)。

## 许可证

MIT
