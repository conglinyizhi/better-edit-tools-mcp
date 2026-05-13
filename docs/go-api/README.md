# betools — Go API Reference

## Package

```
import "github.com/conglinyizhi/better-edit-tools-mcp/pkg/betools"
```

`betools` is the core editing library extracted from better-edit-tools, designed to be embedded directly in Go agent frameworks. It provides read, replace, insert, delete, batch, write, function-range detection, and tag-range detection primitives — all write operations are protected by atomic file writes (temp file + rename).

**Minimum Go version**: Go 1.26+

---

## Public Functions

### File Operations

#### `Read`

```go
func Read(path string, start int, endLine int, opts ...Option) (ShowResult, string, error)
```

Reads a line range from a file. Returns the content and a `viewed_code_id` (second return value) that can be passed to `Replace` for line-number validation.

- `start`: starting line number (>= 1)
- `endLine`: ending line number. Pass `0` or negative to auto-expand to the enclosing function scope (via `FuncRange`)
- Returns a `viewed_code_id`: UUID v4 for later `Replace` session validation
- `opts`: optional `WithFileSystem(...)` injection for sandboxed environments

`Show` remains available as a compatibility alias.

#### `Replace`

```go
func Replace(path string, start, end int, old *string, content string, raw bool, format string, preview bool, sessionID string, opts ...Option) (ReplaceResult, error)
```

Precise line-range substitution.

- `start`, `end`: line range (inclusive), both required
- `old`: optional — when non-nil, verifies current file content against old before writing; returns error on mismatch
- `content`: replacement content
- `raw`: write verbatim without formatting adjustments
- `format`: output format (`"trim"` or `""`)
- `preview`: if true, returns diff without writing to disk
- `viewed_code_id`: optional — validates line count consistency against a prior `be-read` session

#### `Insert`

```go
func Insert(path string, after int, content string, raw bool, format string, preview bool, opts ...Option) (InsertResult, error)
```

Inserts content after a specified line.

- `after`: insert after this line. Pass `0` to insert at the very beginning of the file
- `raw`, `format`, `preview`: same semantics as `Replace`

#### `Delete`

```go
func Delete(path string, start, end, line int, linesJSON *string, format string, preview bool, opts ...Option) (DeleteResult, error)
```

Deletes lines. Three modes:

- Line range: set `start` + `end` (inclusive)
- Single line: set `line`
- Batch: set `linesJSON` as a JSON array like `"[3,5,8]"`

#### `Batch`

```go
func Batch(spec string, preview bool, opts ...Option) (BatchResult, error)
```

Multi-operation editing entry point. Accepts a JSON string:

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

Edits are automatically sorted bottom-to-top to prevent line-number drift.

#### `Write`

```go
func Write(spec string, preview bool, raw bool, opts ...Option) (WriteResult, error)
```

Raw file write tool for full-content replacement. Supports single-file and multi-file payloads.

- `spec`: JSON string
  - Single file: `{"file":"...","content":"..."}`
  - Multi file: `{"files":[{"file":"...","content":"..."}]}`
- `raw`: when true, literal `\n` in content is converted to real newlines
- Falls back to character-level extraction when JSON parsing fails; `preview=true` returns results without writing

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
| `BatchResult` | Status, Files, Results([]BatchFileResult), Preview | Batch operation result |
| `WriteResult` | Status, Files, Results([]WriteFileResult), Degraded, Warning, Preview | Write result. Degraded indicates fallback parser was used |
| `BatchFileResult` | File, Edits, Total | Per-file batch result |
| `WriteFileResult` | File, Lines, Bytes | Per-file write result |
| `FunctionRangeResult` | Start, End | Function scope result |
| `TagRangeResult` | Start, End, Kind, Tag | Tag pair result. Tag is set for single-line tags |

### Balance Detection Types

| Type | Fields | Description |
|------|--------|-------------|
| `MatchedPair` | Symbol, OpenLine, CloseLine, Depth | Matched symbol pair |
| `UnbalancedItem` | Symbol, Line, Expected | Unmatched symbol |
| `QuoteWarning` | Symbol, Count, Lines | Quote parity warning |

### Batch Operation Inputs

| Type | Fields | Description |
|------|--------|-------------|
| `BatchEditSpec` | Action, Start, End, Line, Content | Single edit description |
| `BatchFileSpec` | File, Edits([]BatchEditSpec) | Per-file edit list |

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
