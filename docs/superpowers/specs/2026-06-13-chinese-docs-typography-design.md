# 中文文档排版优化设计

## 背景

`docs/content/**/*.zh.md` 中的中文文档存在以下排版问题：

1. 部分行内有序列表没有拆分为多行列表，影响可读性。
2. 链接后存在多余空格，例如 `[#52](...)） ` 后面多出一个空格。
3. 中英文、中文与数字、中文与代码之间缺少空格，或全角标点前后多了空格，不符合《中文文案排版指北》。

## 目标

对 `docs/content/` 下的所有中文 Markdown 文档进行排版规范化，使其符合《中文文案排版指北》的核心规则，同时保持 Markdown 语义和 shortcode 可用性。

## 范围

涉及文件：

- `docs/content/_index.zh.md`
- `docs/content/decisions.zh.md`
- `docs/content/llm-setup-guide.zh.md`
- `docs/content/go-api/_index.zh.md`
- `docs/content/go-api/README.zh.md`

## 方案

1. 使用 `autocorrect`（Rust 实现）对 5 个 `.zh.md` 文件批量格式化。
2. 手动复核 diff，修正以下可能的误判：
   - 代码块、行内代码、URL、issue shortcode 参数中的内容；
   - 专有名词大小写（如 GitHub、MCP、CLI 等）；
   - 全角标点与英文/数字之间的空格问题。
3. 将截图中行内有序列表改为标准多行有序列表。
4. 移除 `[#52](...)` 链接后的多余空格。
5. 运行 `hugo --gc --minify` 构建验证。
6. 提交并推送。

## 依赖

- `cargo install autocorrect`（系统已有 cargo）。

## 验收标准

- 5 个中文文档构建通过，无警告。
- issue 编号、commit hash 的 JS 自动链接仍然有效。
- 截图中的两个问题（行内列表、#52 后多余空格）已修复。
- 文档语义不变，shortcode 正常渲染。
