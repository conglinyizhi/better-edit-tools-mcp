<div align="right">
  <a href="README.md">English</a> | <a href="README.zh.md">中文</a>
</div>

# better-edit-tools

> 实验性项目：工具名称、参数和行为都可能随着设计继续调整。不要把具体工具名写死到 prompt 里，优先使用能力描述或动态解析的方式选择工具。

> 高性能 MCP（Model Context Protocol）文件编辑工具集 - Go 原生实现，提供原子写入、批量编辑智能排序、函数范围自动检测。
> 工具描述会在启动时通过 `--lang <zh|en>` 进行本地化；如果不传，则会尝试读取 `LANG` 环境变量。

## 工具说明

### `be-balance`

`be-balance` 是结构完整性检查工具，负责核对括号、花括号、方括号、HTML/XML 标签闭合以及引号是否成对。它在扫描时会尽量避开字符串与注释里的干扰符号，并提供 `aggregate`、`unbalanced`（默认）和 `tree` 三种模式，分别适合快速汇总、定位问题和查看层级结构。

── 即使代码里混着字符串、注释和标签，`be-balance` 也会尽量把结构问题揪出来。

### `be-show`

`be-show` 是只读查看工具，用来按行号展示文件内容。`end` 参数既可以是明确的行号，也可以传 `"auto"` 自动扩展到所在函数范围，适合在不手动计算区间的情况下快速查看上下文。

── 不用自己算范围，就能直接看到最需要的那一段上下文。

### `be-replace`

`be-replace` 用于精确替换指定行范围的内容。它适合已经知道目标区间的场景，能以最小范围完成局部修改，并保持和其他工具一致的 `target`、`preview` 交互方式。

── 已知目标区间时，直接做最小范围的精确替换。

### `be-insert`

`be-insert` 用于在指定行后插入内容，`line=0` 时表示插入到文件开头。它是增量编辑里最直接的原语，既能减少对原有内容的扰动，也能和共享参数体系保持一致。

── 把新内容插到准确位置，同时尽量不扰动原文件。

### `be-delete`

`be-delete` 负责删除单行、行范围或 JSON 数组指定的多行。它的设计偏向清理型修改，强调操作的可预测性，所以始终围绕“行”这个粒度来工作。

── 删除时始终保持行级粒度，结果更可控。

### `be-batch`

`be-batch` 是一次性执行多步编辑的入口，支持单文件和多文件的组合修改。它会自动从下往上应用操作，避免行号偏移带来的连锁错误，因此很适合要同时处理多个位置的场景。

── 一次提交多处修改，还能避免行号漂移。

### `be-write`

`be-write` 是原始写入工具，用来直接写入完整文件内容。它同时支持单文件 `{"file","content"}` 和多文件 `{"files":[...]}`，而且在标准 JSON 解析失败时，会自动切换到状态机式的降级提取逻辑，尽量保证 AI 生成内容也能顺利落盘。

── 就算大模型输出的 JSON 出错，`be-write` 也会尽量把这次长输出救回来。

### `be-func-range`

`be-func-range` 用来定位某一行所属的 `{}` 块或函数范围。它基于花括号计数，同时兼顾字符串和注释环境，因此比简单的分隔符查找更适合真实源码。

── 找到的不是随便一个花括号，而是真正的函数边界。

### `be-tag-range`

`be-tag-range` 用来查找某一行所在的 XML/HTML/Vue 标签配对范围。它相当于面向标记语言的范围定位工具，适合在需要“逻辑容器”而不是纯行号时使用。

── 直接定位标签对，把编辑边界收敛到真正的容器范围。

## 设计特点

- **原子写入**：文件修改通过临时文件 + 重命名完成，即使进程崩溃也不会损坏源文件。
- **批量编辑智能排序**：批量操作自动从下往上执行，无需手动调整行号顺序。
- **标准错误响应**：严格按照 MCP 规范，错误时返回 `isError: true`，客户端可正确识别。
- **Go 原生实现**：无运行时依赖，单二进制分发，启动快、易嵌入。
- **容错 JSON 解析**：AI 生成复杂文件内容时容易出现转义错误（反引号、`${}`、未转义引号），`write` 工具会自动降级为字符级提取，避免因 JSON 格式问题导致写入失败。

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

如果不传 `--lang`，服务会尝试从 `LANG` 环境变量推断语言，推断失败时默认英文。

### 在 MCP 客户端中注册

添加到 MCP 客户端配置中：

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

例如 Claude Desktop 的配置文件位置：

- macOS：`~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows：`%APPDATA%\Claude\claude_desktop_config.json`

### 安装

安装脚本会根据当前系统和架构自动下载最新 Release，校验 SHA-256 后安装到 `~/.local/share/better-edit-tools/bin/`。
也可以把 tag 作为参数传给脚本来安装指定版本。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/conglinyizhi/better-edit-tools-mcp/main/scripts/install.sh)
bash <(curl -fsSL https://raw.githubusercontent.com/conglinyizhi/better-edit-tools-mcp/main/scripts/install.sh) v0.1.0
```

Release 产物会提供 Linux、macOS、Windows 的 `amd64` / `arm64` 包，并附带对应的 `.sha256` 校验文件。Windows 产物使用 `.zip` 打包。
Release notes 会按 Conventional Commits 分组，建议使用 `feat(scope)!: ...`、`fix(scope): ...` 这类写法，方便自动生成更干净的 changelog。

## 致谢

工具集中的 `replace`、`insert`、`delete`、`batch` 等操作灵感来自 [includewudi/fast-edit](https://github.com/includewudi/fast-edit)。

## 许可证

MIT
