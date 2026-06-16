# Chip 机制设计决策

## 1. 设计动机

在 MCP（Model Context Protocol）调用链路中，LLM 生成的工具参数常常因为 JSON 转义、字段缺失、格式错乱等原因导致调用失败。一旦失败，原始的参数内容可能只出现在一次性的错误响应里，后续轮次中模型很难精确复现完全相同的内容，尤其是大段代码写入（`be-write`）或多行替换时。

Chip 机制的核心目标是把“失败现场”或“被删除的内容”临时保存下来，作为模型后续恢复操作的可靠数据来源。它相当于在进程内维护了一个轻量级的 FIFO 缓存队列，让 LLM 可以通过专用工具 `be-insert-chip` 把之前丢失或删除的内容重新插入到目标文件中，从而降低因一次调用失败而导致整轮任务中断的概率。

## 2. 何时会创建 chip

当前有两种场景会触发 chip 记录：

### 2.1 工具调用失败时自动保存参数

当某个工具调用返回错误，且入参序列化后的 JSON 长度大于 50 字节时，服务端会调用 `SaveChip(tool, args, errMsg)` 把原始参数保存为 chip。

```go
func SaveChip(tool string, args map[string]any, errMsg string) string
```

- 参数 JSON 长度 ≤ 50 字节时不保存，避免记录过多无意义的短参数。
- 保存成功后返回 chip ID（如 `a3f7b2`）；不需要恢复时返回空字符串。

### 2.2 删除操作时保存被删除内容

`be-delete` 在真正落盘前，会把被删除的非空内容通过 `SaveContentChip` 保存为 chip，并在返回的 `warnings` 中提示模型：

```go
deletedContent := strings.Join(fileLines[start-1:end], "")
if deletedContent != "" {
    chipID, chipWarn := SaveContentChip("be-delete", deletedContent)
    warnings = append(warnings, fmt.Sprintf("deleted content saved as chip://%s", chipID))
}
```

这让误删后可以通过 `be-insert-chip` 把内容重新插回文件。

## 3. chip 里保存了什么

chip 的数据结构定义在 `pkg/betools/chip.go`：

```go
type ChipRecord struct {
    ID        string         `json:"id"`        // chip 唯一标识
    Tool      string         `json:"tool"`      // 来源工具名，如 be-write / be-delete
    Args      map[string]any `json:"args"`      // 原始参数或被删除内容
    ErrMsg    string         `json:"err_msg,omitempty"` // 失败时的错误信息
    CreatedAt int64          `json:"created_at"`        // 创建时间戳（Unix 秒）
}
```

- **来自失败调用的 chip**：`Args` 保存原始工具参数，`ErrMsg` 保存错误文本。
- **来自删除操作的 chip**：`Args` 为 `{"_content": "<被删除的文本>"}`，`ErrMsg` 为空。

chip ID 由 `newShortID` 生成，默认是 6 位十六进制随机串（3 字节熵），冲突时最多重试 5 次，仍冲突则回退到 12 位十六进制串。

## 4. 如何列出、读取、使用 chip

### 4.1 列出 chip

调用 `be-insert-chip` 且不传 `from` 和 `to` 时，服务端返回当前内存中的所有 chip ID：

```json
{
  "status": "ok",
  "chips": ["a3f7b2", "c8e101", "d245aa"]
}
```

内部通过 `ListChips()` 实现，返回顺序为**先入先出**（最老的在前）。

### 4.2 读取单个 chip

通过 `GetChip(id)` 可按 ID 读取 chip。优先从内存队列查找；若不在内存中（例如进程重启后），会回退到磁盘读取 `chip-{id}.json`。

### 4.3 使用 chip 回放内容

`be-insert-chip` 支持两种来源：

- `file:///absolute/path`：从指定文件读取内容。
- `chip://{id}`：从 chip 缓存读取内容。

目标位置格式为：

- `to`: `file:///absolute/path:line_number`

当来源是 `chip://` 时，服务端会把 chip 的 `Args` 重新序列化为 JSON，并附加注释头后通过 `betools.Insert` 插入目标文件：

```go
content = fmt.Sprintf("// Chip %s from tool %q\n// Original arguments:\n%s", rec.ID, rec.Tool, string(argsJSON))
```

这样模型可以在 diff 中清楚地看到回放的是哪一次失败调用的内容。

## 5. 队列容量、淘汰策略与持久化

### 5.1 容量与淘汰

```go
const maxChips = 30
```

- chip 队列是全局唯一的，受 `sync.Mutex` 保护。
- 当队列长度超过 30 时，淘汰**最旧的 chip**（FIFO）。
- 被淘汰的 chip 会从内存 `chipStore` 和 `chipIDSet` 中移除，并同步删除对应的磁盘文件。
- `SaveContentChip` 在发生淘汰时会返回警告文本，例如：
  ```
  oldest chip a3f7b2 was evicted (queue max 30)
  ```

### 5.2 磁盘持久化

每个 chip 独立写入一个 JSON 文件：

```go
path := filepath.Join(ChipDir(), fmt.Sprintf("chip-%s.json", record.ID))
```

缓存目录按平台选择：

- Windows：`%LOCALAPPDATA%/better-edit-tools-mcp/chips`
- Linux/macOS：`$XDG_CACHE_HOME/better-edit-tools-mcp/chips` 或 `~/.cache/better-edit-tools-mcp/chips`
- 兜底：`/tmp/better-edit-tools-mcp-chips`

进程启动时通过 `loadChipsFromDisk()` 从该目录恢复 chip：

1. 读取所有 `.json` 文件；
2. 按 `CreatedAt` 排序；
3. 若超过 `maxChips`，则淘汰旧文件；
4. 载入内存队列。

写入和删除都是“尽力而为”（best-effort），即使磁盘 IO 失败也不会中断主流程。

## 6. 当前限制与未来可扩展方向

### 6.1 当前限制

1. **容量固定**：`maxChips` 是编译期常量 30，无法按会话或磁盘空间动态调整。
2. **无 TTL**：只有 FIFO 淘汰，没有按时间过期机制。
3. **仅文本**：chip 内容以字符串形式保存，不适合直接保存二进制文件内容。
4. **小参数不保存**：参数 JSON ≤ 50 字节时不会生成 chip，某些短但关键的参数可能因此丢失。
5. **磁盘持久化不可靠**：写入失败被静默忽略；进程崩溃时可能残留或丢失部分 chip。
6. **跨进程一致性有限**：虽然会读盘恢复，但并发运行多个 server 实例时可能相互覆盖文件。
7. **回放格式固定**：`chip://` 来源会强制加上注释头，某些场景下可能不希望出现额外注释。

### 6.2 未来可扩展方向

- **可配置容量**：通过启动参数或环境变量调整 `maxChips`。
- **TTL / 过期策略**：为 chip 增加过期时间，自动清理陈旧记录。
- **分类与检索**：按工具类型、文件路径、错误关键词分组，方便模型快速定位需要恢复的 chip。
- **快照联动**：将 chip 与事务快照（snapshot）结合，支持“回滚到删除前状态”并自动附带对应 chip。
- **直接重放参数**：除了把 `Args` 作为文本插入，也可以提供“按原始参数重新调用一次工具”的恢复模式。
- **加密或签名**：对持久化到共享缓存目录的敏感文件内容进行加密或校验，防止信息泄露或篡改。
- **更友好的列表视图**：在 `ListChips` 中返回工具名、创建时间、内容摘要等元数据，而不是仅返回 ID。

---

**相关源码**：`pkg/betools/chip.go`、`pkg/betools/id.go`、`pkg/betools/ops.go`、`internal/server/server.go`
