---
title: "pkg/ 包结构重构设计"
weight: 15
description: "将 pkg/betools 单一超大包拆分为面向其他 agent 开发者可嵌入的子包方案。"
---

# `pkg/` 包结构重构设计

## 背景与问题

当前 `pkg/betools` 是一个单一大型包，囊括了文件系统抽象、文本工具、结构分析、核心编辑、会话缓存、快照队列、chip 队列等全部能力。对于只想把「编辑原语」嵌入到自己 agent 框架中的开发者来说，这带来几个问题：

1. **API 表面过大**：导入 `pkg/betools` 会一次性暴露 20+ 公开标识符，阅读 godoc 时噪音高。
2. **职责混杂**：做 balance check 的代码和写临时文件的代码、维护全局队列的代码都在同一个包内，难以按需裁剪。
3. **测试与 mock 成本高**：虽然提供了 `MemFS`，但它和编辑核心混在一起，沙盒化时仍需理解整个包。
4. **未来扩展容易循环依赖**：随着功能增加，不同子能力互相引用会加剧，最终形成难以拆分的泥球。

本方案参考 `net/http`、`database/sql`、`encoding/json` 等标准库的分层思路，把 `pkg/betools` 拆成一组职责单一、依赖有向的子包，并在顶层保留 `pkg/betools` 作为兼容外观（facade）。

## 设计目标

1. **按需导入**：agent 开发者可以只导入自己需要的子包，例如仅 `pkg/edit` + `pkg/fs`。
2. **清晰的依赖图**：子包之间必须形成 DAG，禁止循环依赖。
3. **向后兼容**：现有通过 `pkg/betools` 使用的代码在至少一个主版本周期内继续工作。
4. **可测试性**：`MemFS`、diff、balance 等都可以独立测试和 mock。
5. **渐进迁移**：不要求一次性完成全部拆分，允许按阶段落地。

## 新的目录结构（建议到文件级别）

```
pkg/
├── betools/              # 兼容外观包，重新导出下层公共 API
│   ├── doc.go            # 包级文档，说明本包为 facade
│   └── facade.go         # 类型别名与转发函数（见下文）
├── fs/                   # 文件系统抽象与原子写
│   ├── fs.go             # FileSystem 接口、OSFileSystem、ErrNotExist
│   └── memfs.go          # MemFS（测试/沙箱用）
├── textutil/             # 文本与行处理工具
│   ├── lines.go          # detectLineEnding、splitKeepLineEnding、prepareContentLines
│   ├── normalize.go      # normalizeLineBreaks、normalizeLineBlock
│   ├── warnings.go       # scanContentWarnings、isTabDominant
│   └── diff.go           # buildDiff
├── structure/            # 结构分析
│   ├── balance.go        # CheckStructureBalance、quickBalanceCheck
│   ├── func_range.go     # FuncRange、functionRangeRaw
│   └── tag_range.go      # TagRange
├── filerange/            # 文件路径 + 行范围解析
│   └── parse.go          # ParseFileRange、HasFileRange
├── session/              # be-read 会话缓存
│   ├── session.go        # CreateSession、GetSession、SessionFromCache
│   └── cleanup.go        # TTL 清理协程
├── queue/                # 快照与 chip 队列（可进一步拆分为 snapshot/chip）
│   ├── snapshot.go       # SnapshotRecord、PushSnapshot、RollbackSnapshots ...
│   ├── chip.go           # ChipRecord、SaveChip、GetChip、SaveContentChip ...
│   └── id.go             # newShortID
└── edit/                 # 核心编辑操作
    ├── edit.go           # Show、Read、Replace、Insert、Delete
    ├── write.go          # Write、WriteSpecItem
    └── target.go         # ResolveTargetSpan、ContentTarget
```

> 说明：`internal/` 目录下已经存在 `internal/edit`，如果未来希望把内部实现与公开 API 完全对齐，可以考虑把 `pkg/edit` 作为公开包装器，具体实现继续放在 `internal/edit`，或者把 `internal/edit` 重命名为 `internal/editimpl` 以避免 import 路径歧义。本方案暂时保留 `internal/edit` 不动，仅在 `pkg/` 下新增公开子包。

## 每个包的公开 API 表面

### `pkg/fs`

```go
package fs

// FileSystem 定义 betools 使用的文件操作。
// 沙箱环境或测试可以注入自定义实现。
type FileSystem interface {
    ReadFile(name string) ([]byte, error)
    WriteFile(name string, data []byte, perm fs.FileMode) error
    Stat(name string) (fs.FileInfo, error)
    Rename(oldpath, newpath string) error
    Remove(name string) error
    Open(name string) (io.ReadCloser, error)
    Create(name string) (io.WriteCloser, error)
}

// OSFileSystem 是默认的宿主机实现。
type OSFileSystem struct{}

// MemFS 是线程安全的内存文件系统，适用于测试和沙箱。
type MemFS struct{ ... }
func NewMemFS(initial map[string]string) *MemFS

// ErrNotExist 便于测试代码统一判断文件不存在。
var ErrNotExist = errors.New("file does not exist")
```

### `pkg/textutil`

```go
package textutil

func DetectLineEnding(content string) string
func SplitKeepLineEnding(content, le string) []string
func PrepareContentLines(content, lineEnding string) []string
func NormalizeLineBreaks(s string) string
func NormalizeLineBlock(content string) string
func ScanContentWarnings(content string) []string
func RustLineCount(content string) int
```

### `pkg/structure`

```go
package structure

import "github.com/conglinyizhi/better-edit-tools-mcp/pkg/fs"

type Option func(*Config)
func WithFileSystem(fsys fs.FileSystem) Option

func CheckStructureBalance(path string, verbose bool, opts ...Option) (string, error)
func QuickBalanceCheck(content string) string
func FuncRange(path string, line int, opts ...Option) (FunctionRangeResult, error)
func TagRange(path string, line int, opts ...Option) (TagRangeResult, error)
```

> 每个子包都可以有自己的 `Option` 与 `Config`，但都统一通过 `WithFileSystem` 注入文件系统。这样可以避免子包之间互相依赖对方的 option 类型。

### `pkg/filerange`

```go
package filerange

func ParseFileRange(input string) (file string, start int, end int, err error)
func HasFileRange(file string) bool
```

### `pkg/session`

```go
package session

import "github.com/conglinyizhi/better-edit-tools-mcp/pkg/fs"

type ReadSession struct { ... }

func CreateSession(file string, start, end int, opts ...Option) string
func GetSession(id string) *ReadSession
func SessionFromCache(id string, opts ...Option) (s *ReadSession, warning string)
func CleanupSession(id string)
```

### `pkg/queue`

```go
package queue

import "github.com/conglinyizhi/better-edit-tools-mcp/pkg/fs"

type SnapshotRecord struct { ... }
type SnapshotRange struct { ... }
type QueueStats struct { ... }
type ChipRecord struct { ... }
type ChipQueueInfo struct { ... }

// snapshot
func PushSnapshot(rec SnapshotRecord) (id string, queueWarning string)
func CommitSnapshots() int
func RollbackSnapshots(step int) (count int, errors []error)
func ListSnapshots() []SnapshotRecord
func SnapshotQueueStats() QueueStats

// chip
func SaveChip(tool string, args map[string]any, errMsg string) string
func SaveContentChip(tool string, content string) (id string, overflowWarn string)
func GetChip(id string) (*ChipRecord, error)
func ListChips() []string
func ChipQueueInfoValue() ChipQueueInfo
```

> 如果 snapshot 与 chip 的演进方向不同（例如 snapshot 需要持久化、chip 需要网络同步），未来可以再拆分为 `pkg/snapshot` 和 `pkg/chip`，中间由 `pkg/edit` 同时依赖。

### `pkg/edit`

```go
package edit

import (
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/fs"
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/structure"
)

type Option func(*Config)
func WithFileSystem(fsys fs.FileSystem) Option

func Show(path string, start int, endLine int, brief bool, opts ...Option) (ShowResult, string, error)
func Read(path string, start int, endLine int, brief bool, opts ...Option) (ShowResult, string, error)
func Replace(path string, start, end int, old *string, content string, format string, preview bool, sessionID string, brief bool, opts ...Option) (ReplaceResult, error)
func Insert(path string, after int, content string, format string, preview bool, brief bool, opts ...Option) (InsertResult, error)
func Delete(path string, start, end int, format string, preview bool, brief bool, opts ...Option) (DeleteResult, error)
func Write(spec string, preview bool, brief bool, opts ...Option) (WriteResult, error)

func ResolveTargetSpan(path string, target ContentTarget, opts ...Option) (TargetSpan, error)
```

> 结果类型（`ShowResult`、`ReplaceResult` 等）保留在 `pkg/edit` 中，因为它们是编辑操作的输出契约。其它包如果需要（例如 facade 需要统一导出），可通过类型别名引用。

### `pkg/betools`（兼容外观）

```go
package betools

import (
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/edit"
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/fs"
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/filerange"
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/queue"
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/session"
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/structure"
)

// 类型别名
type (
    FileSystem          = fs.FileSystem
    OSFileSystem        = fs.OSFileSystem
    MemFS               = fs.MemFS
    Option              = edit.Option
    ContentTarget       = edit.ContentTarget
    TargetSpan          = edit.TargetSpan
    ShowResult          = edit.ShowResult
    ReplaceResult       = edit.ReplaceResult
    InsertResult        = edit.InsertResult
    DeleteResult        = edit.DeleteResult
    WriteSpecItem       = edit.WriteSpecItem
    WriteFileResult     = edit.WriteFileResult
    WriteResult         = edit.WriteResult
    FunctionRangeResult = structure.FunctionRangeResult
    TagRangeResult      = structure.TagRangeResult
    MatchedPair         = structure.MatchedPair
    UnbalancedItem      = structure.UnbalancedItem
    QuoteWarning        = structure.QuoteWarning
    ReadSession         = session.ReadSession
    SnapshotRecord      = queue.SnapshotRecord
    SnapshotRange       = queue.SnapshotRange
    QueueStats          = queue.QueueStats
    ChipRecord          = queue.ChipRecord
    ChipQueueInfo       = queue.ChipQueueInfo
)

// 转发函数
var (
    WithFileSystem        = edit.WithFileSystem
    NewMemFS              = fs.NewMemFS
    Show                  = edit.Show
    Read                  = edit.Read
    Replace               = edit.Replace
    Insert                = edit.Insert
    Delete                = edit.Delete
    Write                 = edit.Write
    ResolveTargetSpan     = edit.ResolveTargetSpan
    FuncRange             = structure.FuncRange
    TagRange              = structure.TagRange
    CheckStructureBalance = structure.CheckStructureBalance
    ParseFileRange        = filerange.ParseFileRange
    HasFileRange          = filerange.HasFileRange
    CreateSession         = session.CreateSession
    GetSession            = session.GetSession
    SessionFromCache      = session.SessionFromCache
    CleanupSession        = session.CleanupSession
    PushSnapshot          = queue.PushSnapshot
    CommitSnapshots       = queue.CommitSnapshots
    RollbackSnapshots     = queue.RollbackSnapshots
    ListSnapshots         = queue.ListSnapshots
    SnapshotQueueStats    = queue.SnapshotQueueStats
    SaveChip              = queue.SaveChip
    SaveContentChip       = queue.SaveContentChip
    GetChip               = queue.GetChip
    ListChips             = queue.ListChips
    ChipQueueInfoValue    = queue.ChipQueueInfoValue
)
```

通过这种方式，现有代码 `import ".../pkg/betools"` 和 `betools.Replace(...)` 完全不用改。

## 如何避免循环依赖

循环依赖是拆包时最容易踩的坑。本方案通过以下规则约束：

1. **分层原则**：把功能按抽象级别分层，低层包不依赖高层包。
   - 第 0 层：`pkg/fs`（纯接口与实现）
   - 第 1 层：`pkg/textutil`、`pkg/filerange`（纯文本逻辑，可依赖 fs 读文件）
   - 第 2 层：`pkg/structure`、`pkg/session`（依赖 fs + textutil）
   - 第 3 层：`pkg/queue`（依赖 fs + textutil，用于回滚写文件和持久化 chip）
   - 第 4 层：`pkg/edit`（依赖以上所有包）
   - 第 5 层：`pkg/betools`（仅做重新导出）

2. **Option 本地化**：每个子包维护自己的 `Option` / `Config`，只注入 `fs.FileSystem`。这样 `pkg/structure` 不会为了使用 `pkg/edit` 的 option 而反向依赖。

3. **结果类型归操作包**：`ShowResult` 等类型放在 `pkg/edit`，`FunctionRangeResult` 放在 `pkg/structure`。若 facade 需要统一导出，使用类型别名，而不是把类型上移到一个公共包导致大家都依赖它。

4. **全局状态收敛到 queue/session**：
   - 会话缓存全局状态留在 `pkg/session`。
   - 快照与 chip 全局状态留在 `pkg/queue`。
   - `pkg/edit` 在操作成功后调用 `queue.PushSnapshot` 和 `queue.SaveContentChip`，这是自上而下的依赖，不会循环。

5. **避免公共 types 包**：专门建一个 `pkg/types` 听起来诱人，但会导致所有包都依赖它，反而容易循环。需要共享的小类型（如 `MatchedPair`）直接放在产生它的 `pkg/structure` 中，由 facade 统一别名导出。

## 迁移步骤和兼容性策略

### 阶段一：建立子包与兼容外观（推荐首先落地）

1. 新建 `pkg/fs`，把 `FileSystem`、`OSFileSystem`、`MemFS`、`ErrNotExist` 移过去。
2. 更新 `pkg/betools/fs.go`：导入 `pkg/fs`，保留 `Option` 与 `WithFileSystem` 作为转发。
3. 运行 `go build ./... && go test ./...`，修复引用。
4. 新建 `pkg/filerange`，移动 `ParseFileRange` / `HasFileRange`。
5. 在 `pkg/betools` 中通过别名保留二者。
6. 重复上述过程，逐步拆分 `textutil`、`structure`、`session`、`queue`、`edit`。

### 阶段二：清理内部实现

1. `internal/server` 与 `internal/app` 可以直接改为导入更细粒度的子包（例如 `pkg/edit`、`pkg/filerange`、`pkg/structure`、`pkg/queue`），减少 facade 的依赖。
2. 如果 `internal/edit` 与新的 `pkg/edit` 职责重叠，应合并或重命名，避免维护两份编辑逻辑。

### 阶段三：废弃与移除 facade

1. 在 `pkg/betools/doc.go` 中标注：
   ```go
   // Deprecated: pkg/betools is a compatibility facade.
   // New code should use the focused subpackages under pkg/.
   ```
2. 保留至少一个主版本周期（例如 v0.x → v1.0），再在 major 版本移除 facade。

### 兼容性保障

- 所有公开函数签名在 facade 中保持不变。
- JSON 输出结构（`ShowResult` 等）由 `pkg/edit` 定义，facade 通过类型别名确保字段完全一致。
- 新增子包不破坏现有 import 路径；只有新增路径，没有删除路径。

## 其他 agent 开发者如何引用

### 场景 A：只需要读取和替换文件

```go
package myagent

import (
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/edit"
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/fs"
)

func patchFile(path string, line int, old, new string) error {
    mem := fs.NewMemFS(map[string]string{path: old + "\n"})
    res, err := edit.Replace(path, line, line, &old, new, "plain", false, "", false,
        edit.WithFileSystem(mem))
    if err != nil {
        return err
    }
    _ = res.Diff
    return nil
}
```

### 场景 B：只需要结构检查

```go
package myagent

import (
    "fmt"
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/structure"
)

func check(path string) error {
    report, err := structure.CheckStructureBalance(path, false)
    if err != nil {
        return err
    }
    fmt.Println(report)
    return nil
}
```

### 场景 C：沙箱化所有文件操作

```go
package myagent

import (
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/edit"
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/fs"
)

func runInSandbox(files map[string]string) (*fs.MemFS, error) {
    mem := fs.NewMemFS(files)
    for path := range files {
        if _, _, err := edit.Read(path, 1, -1, true, edit.WithFileSystem(mem)); err != nil {
            return nil, err
        }
    }
    return mem, nil
}
```

### 场景 D：继续使用老接口

```go
package myagent

import (
    "github.com/conglinyizhi/better-edit-tools-mcp/pkg/betools"
)

func legacy(path string) error {
    _, _, err := betools.Read(path, 1, -1, false)
    return err
}
```

## 验收标准

1. `go build ./...` 与 `go test ./...` 全部通过。
2. `pkg/betools` 中所有公开标识符均为类型别名或转发变量/函数，无独立实现逻辑。
3. 子包之间 `go list -deps` 不出现循环依赖。
4. 现有 `internal/server` 与 `internal/app` 在不修改调用代码的情况下继续编译。
5. 新增示例代码能独立编译运行。

## 相关文件

- 当前实现：`pkg/betools/*.go`
- 兼容性外观规划：`pkg/betools/facade.go`
- 本设计文档：`docs/content/decisions/pkg-restructure.md`
