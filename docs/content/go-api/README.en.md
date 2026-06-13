---
title: "Go API Reference"
weight: 50
description: "Go API reference for embedding better-edit-tools in Go agent frameworks."
---
# betools — Go API Reference

## Package

```
import "github.com/conglinyizhi/better-edit-tools-mcp/pkg/betools"
```

`betools` is the core editing library extracted from better-edit-tools, designed to be embedded directly in Go agent frameworks. It provides read, replace, insert, delete, write, function-range detection, and tag-range detection primitives — all write operations are protected by atomic file writes (temp file + rename).

**Minimum Go version**: Go 1.26+

---

## Public Functions

### File Operations

#### `Read`

```go
func Read(path string, start int, endLine int, brief bool, opts ...Option) (ShowResult, string, error)
```

Reads a line range from a file. Returns the content and a `viewed_code_id` (second return value) that can be passed to `Replace` for line-number validation.

- `start`: starting line number (>= 1)
- `endLine`: ending line number. Pass `0` or negative to auto-expand to the enclosing function scope (via `FuncRange`)
- Returns a `viewed_code_id`: UUID v4 for later `Replace` session validation
- `opts`: optional `WithFileSystem(...)` injection for sandboxed environments

`Show` remains available as a compatibility alias.

#### `Replace`

```go
func Replace(path string, start, end int, old *string, content string, format string, preview bool, sessionID string, brief bool, opts ...Option) (ReplaceResult, error)
```

Precise line-range substitution.

- `start`, `end`: line range (inclusive), both required
- `old`: optional — when non-nil, verifies current file content against old before writing; returns error on mismatch
- `content`: replacement content
- `format`: output format (`"trim"` or `""`)
- `preview`: if true, returns diff without writing to disk
- `sessionID`: optional — validates line count consistency against a prior `be-read` session
- `brief`: return minimal response (omit diff and balance)

#### `Insert`

```go
func Insert(path string, after int, content string, format string, preview bool, brief bool, opts ...Option) (InsertResult, error)
```

Inserts content after a specified line.

- `after`: insert after this line. Pass `0` to insert at the very beginning of the file
- `format`, `preview`, `brief`: same semantics as `Replace`

#### `Delete`

```go
func Delete(path string, start, end int, format string, preview bool, brief bool, opts ...Option) (DeleteResult, error)
```

Deletes lines in the specified range.

- `start`, `end`: line range (inclusive), both required
- `format`, `preview`, `brief`: same semantics as `Replace`

#### `Write`

```go
func Write(spec string, preview bool, brief bool, opts ...Option) (WriteResult, error)
```

Raw file write tool for full-content replacement. Supports single-file and multi-file payloads.

- `spec`: JSON string
  - Single file: `{"file":"...","content":"..."}`
  - Multi file: `{"files":[{"file":"...","content":"..."}]}`
- `preview`: if true, returns results without writing to disk
- `brief`: return minimal response (omit per-file details)
- Falls back to character-level extraction when JSON parsing fails

### Scope Detection

#### `FuncRange`

```go
func FuncRange(path string, line int, opts ...Option) (FunctionRangeResult, error)
```

Detects the enclosing `{}` block or function range for a line. Backtracks through func/type/method keywords.

#### `TagRange`

```go
func TagRange(path string, line int, opts ...Option) (TagRangeResult, error)
```

Finds the enclosing XML/HTML/Vue tag pair for a line.

#### `ResolveTargetSpan`

```go
func ResolveTargetSpan(path string, target ContentTarget, opts ...Option) (TargetSpan, error)
```

Resolves a `ContentTarget` to a line range. `ContentTarget.Kind` supports `"line"`, `"function"`, `"marker"`, `"tag"`.

### Structural Balance Detection

#### `CheckStructureBalance`

```go
func CheckStructureBalance(path string, verbose bool, opts ...Option) (string, error)
```

Checks brackets, braces, parentheses, HTML/XML tag closure, and quote parity.

- `verbose=false`: only outputs unmatched items
- `verbose=true`: outputs all matched pairs

### Session Management

#### `CreateSession`

```go
func CreateSession(file string, start, end int, opts ...Option) string
```

Creates a read session and returns its UUID. Core building block for `SessionFromCache`.

#### `GetSession`

```go
func GetSession(id string) *ReadSession
```

Looks up a session by UUID. Returns nil if expired (>24h) or not found.

#### `SessionFromCache`

```go
func SessionFromCache(id string, opts ...Option) (s *ReadSession, warning string)
```

Looks up and validates a session. If the file has changed (line count mismatch), returns a non-fatal warning with head/tail sample lines to help re-sync.

#### `CleanupSession`

```go
func CleanupSession(id string)
```

Manually removes a session. Expired sessions are auto-cleaned by a background goroutine every 30 minutes.

### File System Injection

```go
func WithFileSystem(fsys FileSystem) Option
```

Pass this option to constrain betools to a custom file system, such as a workspace wrapper or sandboxed view of the repository.

### Chip Storage

#### `SaveChip`

```go
func SaveChip(tool string, args map[string]any) int
```

Saves failed MCP tool call arguments as a chip record. Only records when the serialised JSON exceeds 50 bytes. Returns the chip ID, or 0 if args were too short. Maintains a FIFO queue of up to 10 records, persisted to `/tmp/bet-chips/`.

#### `GetChip`

```go
func GetChip(id int) (*ChipRecord, error)
```

Queries a chip record by ID. Checks memory first, falls back to `/tmp/bet-chips/chip-{id}.json` on disk.

#### `ListChips`

```go
func ListChips() []int
```

Returns all in-memory chip IDs in insertion order (oldest first).

---

## Public Types

### Result Types

| Type | Fields | Description |
|------|--------|-------------|
| `ShowResult` | Status, File, Start, End, Total, Content | File read result |
| `ReplaceResult` | Status, File, Removed, Added, Total, Diff, Balance, Affected, Preview, Warning | Replace result. Warning from session validation |
| `InsertResult` | Status, File, After, Added, Total, Diff, Balance, Affected, Preview | Insert result |
| `DeleteResult` | Status, File, Total, Diff, Balance, Affected, Preview | Delete result |
| `WriteResult` | Status, Files, Results([]WriteFileResult), Degraded, Warning, Preview | Write result. Degraded indicates fallback parser was used |
| `WriteFileResult` | File, Lines, Bytes | Per-file write result |
| `FunctionRangeResult` | Start, End | Function scope result |
| `TagRangeResult` | Start, End, Kind, Tag | Tag pair result. Tag is set for single-line tags |

### Balance Detection Types

| Type | Fields | Description |
|------|--------|-------------|
| `MatchedPair` | Symbol, OpenLine, CloseLine, Depth | Matched symbol pair |
| `UnbalancedItem` | Symbol, Line, Expected | Unmatched symbol |
| `QuoteWarning` | Symbol, Count, Lines | Quote parity warning |

### Session

| Type | Fields | Description |
|------|--------|-------------|
| `ReadSession` | File, StartLine, EndLine, LineCount, CreatedAt | Read session record |

### Chip

| Type | Fields | Description |
|------|--------|-------------|
| `ChipRecord` | ID, Tool, Args | Tool call snapshot |

### Target Resolution

| Type | Fields | Description |
|------|--------|-------------|
| `ContentTarget` | Kind, Value | Target descriptor |
| `TargetSpan` | Start, End | Resolved line range |

### Write Parameters

| Type | Fields | Description |
|------|--------|-------------|
| `WriteSpecItem` | File, Content | Single-file write parameters |

### Sentinel Errors

```go
var ErrInvalid = errors.New("invalid argument")
var ErrRead    = errors.New("read error")
var ErrWrite   = errors.New("write error")
```

All betools errors can be matched with `errors.Is` against these three sentinels.
