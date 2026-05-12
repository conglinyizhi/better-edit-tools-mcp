# betools — Go API 文档

## 包说明

```
import "github.com/conglinyizhi/better-edit-tools-mcp/pkg/betools"
```

`betools` 是 better-edit-tools 的核心编辑库，可直接嵌入 Go Agent 框架使用。它提供文件读取、内容替换、插入、删除、批量操作、全局写入、函数范围检测等原语，所有写操作都经过原子写入（临时文件 + rename）保护。

**版本要求**：Go 1.23+

---

## 公开函数

### 文件操作

#### `Show`

```go
func Show(path string, start int, endLine int) (ShowResult, string, error)
```

读取指定文件的行范围。返回内容包括完整内容和 `session_id`（第二个返回值）。`session_id` 可传给 `Replace` 进行行数校验。

- `start`：起始行号（>= 1）
- `endLine`：结束行号。传 `0` 或负数时自动扩展到所在函数范围（基于 `FuncRange`）
- 返回 `session_id`：UUID v4，用于后续 `Replace` 的 session 校验

#### `Replace`

```go
func Replace(path string, start, end int, old *string, content string, raw bool, format string, preview bool, sessionID string) (ReplaceResult, error)
```

精确替换指定行范围的内容。

- `start`、`end`：行范围（闭区间），二者均不可省略
- `old`：可选，非 nil 时会校验当前文件内容与 old 是否一致，不一致则返回错误
- `content`：替换后的内容
- `raw`：是否原样写入，不与格式化逻辑冲突
- `format`：写入后的格式（"trim" 或 ""）
- `preview`：如果为 true，只返回 diff 不写入文件
- `sessionID`：可选，传 `be-show` 返回的 `session_id` 时校验行数一致性

#### `Insert`

```go
func Insert(path string, after int, content string, raw bool, format string, preview bool) (InsertResult, error)
```

在指定行后插入内容。

- `after`：在此行后插入。传 `0` 表示在文件最开头插入
- `content`：插入的内容
- `raw`、`format`、`preview`：与 `Replace` 语义一致

#### `Delete`

```go
func Delete(path string, start, end, line int, linesJSON *string, format string, preview bool) (DeleteResult, error)
```

删除行。支持三种模式：

- 行范围删除：传 `start` + `end`（闭区间）
- 单行删除：传 `line`
- 批量删除：传 `linesJSON`，JSON 数组格式如 `"[3,5,8]"`

#### `Batch`

```go
func Batch(spec string, preview bool) (BatchResult, error)
```

多步编辑入口。传入 JSON 字符串，格式如下：

```json
{
  "files": [
    {
      "file": "/path/to/file.go",
      "edits": [
        {"action": "replace", "start": 3, "end": 5, "content": "new content"},
        {"action": "insert", "line": 10, "content": "inserted line"},
        {"action": "delete", "line": 20}
      ]
    }
  ]
}
```

自动从下往上排序以避免行号偏移。

#### `Write`

```go
func Write(spec string, preview bool, raw bool) (WriteResult, error)
```

原始写入工具，直接覆盖文件内容。支持单文件和批量文件写入。

- `spec`：JSON 字符串
  - 单文件：`{"file":"...","content":"..."}`
  - 多文件：`{"files":[{"file":"...","content":"..."}]}`
- `raw`：为 true 时 content 中的字面 `\n` 转为真实换行符
- JSON 解析失败时自动降级为字符级提取；`preview=true` 时只返回结果不写入

### 范围检测

#### `FuncRange`

```go
func FuncRange(path string, line int) (FunctionRangeResult, error)
```

检测某一行所在的 `{}` 块或函数范围。支持 func、type、method 关键字回溯。

#### `TagRange`

```go
func TagRange(path string, line int) (TagRangeResult, error)
```

检测某一行所在的 XML/HTML/Vue 标签配对范围。

#### `ResolveTargetSpan`

```go
func ResolveTargetSpan(path string, target ContentTarget) (TargetSpan, error)
```

根据 `ContentTarget` 解析目标范围。`ContentTarget.Kind` 支持 `"line"`、`"function"`、`"marker"`、`"tag"`。

### 结构平衡检测

#### `CheckStructureBalance`

```go
func CheckStructureBalance(path string, verbose bool) (string, error)
```

检查文件中括号、花括号、方括号、HTML/XML 标签闭合和引号成对情况。

- `verbose=false`：只输出不匹配项
- `verbose=true`：输出全部匹配对

### Session 管理

#### `CreateSession`

```go
func CreateSession(file string, start, end int) string
```

创建一个读会话，返回 UUID。`SessionFromCache` 的核心调用方。

#### `GetSession`

```go
func GetSession(id string) *ReadSession
```

按 UUID 查询读会话。已过期（>24h）或不存在时返回 nil。

#### `SessionFromCache`

```go
func SessionFromCache(id string) (s *ReadSession, warning string)
```

查询会话并校验文件是否变更（行数对比）。文件变更时返回非空 `warning` 但不会阻塞操作，包含头尾样本行帮助 LLM 重新定位。

#### `CleanupSession`

```go
func CleanupSession(id string)
```

手动删除一个读会话记录。过期会话由后台 goroutine 每 30 分钟自动清理。

### Chip 存储

#### `SaveChip`

```go
func SaveChip(tool string, args map[string]any) int
```

保存失败的 MCP 工具调用参数。当参数 JSON 长度 > 50 时写入 chip 存储。返回 chip ID，参数过短时返回 0。

内部维护一个最大 10 条记录的 FIFO 队列，同时持久化到 `/tmp/bet-chips/`。

#### `GetChip`

```go
func GetChip(id int) (*ChipRecord, error)
```

按 ID 查询 chip 记录。先从内存查找，内存中不存在时从 `/tmp/bet-chips/chip-{id}.json` 回退读取。

#### `ListChips`

```go
func ListChips() []int
```

返回内存中所有 chip ID，按存入顺序排列（旧 → 新）。

---

## 公开类型

### 操作结果

| 类型 | 字段 | 说明 |
|------|------|------|
| `ShowResult` | Status, File, Start, End, Total, Content | 文件读取结果 |
| `ReplaceResult` | Status, File, Removed, Added, Total, Diff, Balance, Affected, Preview, Warning | 替换结果。Warning 由 session 校验产生 |
| `InsertResult` | Status, File, After, Added, Total, Diff, Balance, Affected, Preview | 插入结果 |
| `DeleteResult` | Status, File, Total, Diff, Balance, Affected, Preview | 删除结果 |
| `BatchResult` | Status, Files, Results([]BatchFileResult), Preview | 批量操作结果 |
| `WriteResult` | Status, Files, Results([]WriteFileResult), Degraded, Warning, Preview | 写入结果。Degraded 表示是否启用了降级解析 |
| `BatchFileResult` | File, Edits, Total | 批量操作的单文件结果 |
| `WriteFileResult` | File, Lines, Bytes | 写入的单文件结果 |
| `FunctionRangeResult` | Start, End | 函数范围结果 |
| `TagRangeResult` | Start, End, Kind, Tag | 标签范围结果。Tag 在单行标签时返回 |

### 平衡检测类型

| 类型 | 字段 | 说明 |
|------|------|------|
| `MatchedPair` | Symbol, OpenLine, CloseLine, Depth | 成对符号 |
| `UnbalancedItem` | Symbol, Line, Expected | 不匹配符号 |
| `QuoteWarning` | Symbol, Count, Lines | 引号警告 |

### 批量操作输入

| 类型 | 字段 | 说明 |
|------|------|------|
| `BatchEditSpec` | Action, Start, End, Line, Content | 单条编辑描述 |
| `BatchFileSpec` | File, Edits([]BatchEditSpec) | 单文件编辑列表 |

### Session 相关

| 类型 | 字段 | 说明 |
|------|------|------|
| `ReadSession` | File, StartLine, EndLine, LineCount, CreatedAt | 读会话记录 |

### Chip 相关

| 类型 | 字段 | 说明 |
|------|------|------|
| `ChipRecord` | ID, Tool, Args | 工具调用参数快照 |

### 目标解析

| 类型 | 字段 | 说明 |
|------|------|------|
| `ContentTarget` | Kind, Value | 目标描述 |
| `TargetSpan` | Start, End | 解析为行范围 |

### 写入参数

| 类型 | 字段 | 说明 |
|------|------|------|
| `WriteSpecItem` | File, Content | 单文件写入参数 |

### 错误哨兵

```go
var ErrInvalid = errors.New("invalid argument")
var ErrRead    = errors.New("read error")
var ErrWrite   = errors.New("write error")
```

所有 betools 返回的错误均可通过 `errors.Is` 与这三个哨兵值匹配。
