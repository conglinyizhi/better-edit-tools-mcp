package betools

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// SnapshotRecord captures the before and after state of a file editing operation.
type SnapshotRecord struct {
	ID        string         `json:"id"`
	Tool      string         `json:"tool"`
	File      string         `json:"file"`
	Before    SnapshotRange  `json:"before"`
	After     SnapshotRange  `json:"after"`
	Args      map[string]any `json:"args,omitempty"`
	Summary   string         `json:"summary"`
	CreatedAt int64          `json:"created_at"`
}

// SnapshotRange describes a range of lines in a file.
// When used in a SnapshotRecord, Start/End/Lines now always represent the
// entire file (Start=1, End=total lines, Lines=full file) so that rollback
// can restore the complete file regardless of later edits.
type SnapshotRange struct {
	Start int      `json:"start"`
	End   int      `json:"end"`
	Lines []string `json:"lines"`
}

// MaxSnapshots is the maximum number of pending snapshots in the queue.
const MaxSnapshots = 30

// MaxSnapshotTotalBytes is the maximum total disk space used by snapshots.
const MaxSnapshotTotalBytes = 100 * 1024 * 1024 // 100MB

var (
	snapshotMu     sync.Mutex
	snapshots      []SnapshotRecord
	snapshotIDs    map[string]struct{}
	snapshotLoaded bool

	// maxSnapshotTotalBytes is mutable so tests can exercise capacity limits
	// without writing hundreds of megabytes to disk.
	maxSnapshotTotalBytes = MaxSnapshotTotalBytes
)

// snapshotPersistEnabled returns true if disk persistence should be used.
// BETTER_EDIT_SNAPSHOT_PERSIST defaults to true; only the literal value
// "false" (case-insensitive) disables persistence. An empty value also
// counts as true.
func snapshotPersistEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("BETTER_EDIT_SNAPSHOT_PERSIST")))
	if v == "" {
		return true
	}
	return v != "false"
}

// SnapshotDir returns the platform-appropriate snapshot cache directory.
//   - BETTER_EDIT_SNAPSHOT_DIR env var overrides everything (useful for tests).
//   - Otherwise the base directory is chosen per platform and appended with
//     a workspace-specific subdirectory so different projects do not share
//     snapshot state.
//   - Windows: %LOCALAPPDATA%/better-edit-tools-mcp/snapshots/<workspace-id>
//   - Linux/macOS: $XDG_CACHE_HOME/better-edit-tools-mcp/snapshots/<workspace-id>
//     or ~/.cache/better-edit-tools-mcp/snapshots/<workspace-id>
//   - Fallback: /tmp/better-edit-tools-mcp-snapshots/<workspace-id>
func SnapshotDir() string {
	if dir := os.Getenv("BETTER_EDIT_SNAPSHOT_DIR"); dir != "" {
		return dir
	}

	var base string
	switch runtime.GOOS {
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			base = filepath.Join(localAppData, "better-edit-tools-mcp", "snapshots")
		}
	default:
		if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
			base = filepath.Join(xdgCache, "better-edit-tools-mcp", "snapshots")
		}
		if base == "" {
			if home, err := os.UserHomeDir(); err == nil {
				base = filepath.Join(home, ".cache", "better-edit-tools-mcp", "snapshots")
			}
		}
	}
	if base == "" {
		base = "/tmp/better-edit-tools-mcp-snapshots"
	}
	return filepath.Join(base, WorkspaceID())
}

// WorkspaceID returns a stable, 16-character hexadecimal identifier for the
// current project/workspace. It is used to isolate snapshot directories.
// Resolution order:
//   1. BETTER_EDIT_WORKSPACE environment variable.
//   2. Root of the current git repository (git rev-parse --show-toplevel).
//   3. Current working directory.
func WorkspaceID() string {
	if id := os.Getenv("BETTER_EDIT_WORKSPACE"); id != "" {
		return hashWorkspaceID(id)
	}

	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		root := strings.TrimSpace(string(out))
		if root != "" {
			return hashWorkspaceID(root)
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	return hashWorkspaceID(wd)
}

// hashWorkspaceID returns a stable 16-character hex hash of s.
func hashWorkspaceID(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

// ensureSnapshotStore lazily initializes the snapshot directory and loads
// previously persisted snapshots from disk. This makes snapshots survive
// process restarts while still respecting environment overrides set after
// package init (e.g. in tests).
func ensureSnapshotStore() {
	snapshotMu.Lock()
	if snapshotLoaded {
		snapshotMu.Unlock()
		return
	}
	snapshotLoaded = true
	snapshotMu.Unlock()

	dir := SnapshotDir()
	_ = os.MkdirAll(dir, 0755)

	snapshotMu.Lock()
	defer snapshotMu.Unlock()
	loadSnapshotsFromDiskLocked()
}

// loadSnapshotsFromDiskLocked reads all snapshot JSON files from the cache
// directory into the in-memory store. The caller must hold snapshotMu.
func loadSnapshotsFromDiskLocked() {
	if !snapshotPersistEnabled() {
		return
	}

	dir := SnapshotDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	snapshots = nil
	if snapshotIDs == nil {
		snapshotIDs = make(map[string]struct{}, MaxSnapshots)
	}

	var loaded []SnapshotRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "snapshot-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			continue
		}
		var rec SnapshotRecord
		if jsonErr := json.Unmarshal(data, &rec); jsonErr != nil {
			continue
		}
		loaded = append(loaded, rec)
		snapshotIDs[rec.ID] = struct{}{}
	}

	// Sort by CreatedAt to preserve chronological order
	sort.Slice(loaded, func(i, j int) bool {
		return loaded[i].CreatedAt < loaded[j].CreatedAt
	})

	// Trim to MaxSnapshots if too many on disk
	if len(loaded) > MaxSnapshots {
		for _, r := range loaded[:len(loaded)-MaxSnapshots] {
			delete(snapshotIDs, r.ID)
			removeSnapshotFile(r.ID)
		}
		loaded = loaded[len(loaded)-MaxSnapshots:]
	}

	snapshots = loaded

	// Enforce total disk capacity after loading from disk.
	evictSnapshotsByCapacityLocked()
}

// snapshotDiskBytesLocked returns the total on-disk size of all pending
// snapshot files. The caller must hold snapshotMu.
func snapshotDiskBytesLocked() int64 {
	var total int64
	dir := SnapshotDir()
	for _, s := range snapshots {
		info, err := os.Stat(filepath.Join(dir, fmt.Sprintf("snapshot-%s.json", s.ID)))
		if err == nil {
			total += info.Size()
		}
	}
	return total
}

// evictSnapshotsByCapacityLocked removes the oldest snapshots until the total
// on-disk size is within maxSnapshotTotalBytes. It deletes both the in-memory
// record and the persisted file. The caller must hold snapshotMu.
func evictSnapshotsByCapacityLocked() {
	for snapshotDiskBytesLocked() > int64(maxSnapshotTotalBytes) && len(snapshots) > 0 {
		removed := snapshots[0]
		delete(snapshotIDs, removed.ID)
		snapshots = snapshots[1:]
		removeSnapshotFile(removed.ID)
	}
}

// PushSnapshot pushes a record onto the queue. If full, evicts oldest.
// Returns the record ID and a warning string (empty if no issue, non-empty if eviction happened).
// The file modification itself is NOT affected by queue capacity — it already completed.
func PushSnapshot(rec SnapshotRecord) (id string, queueWarning string) {
	ensureSnapshotStore()

	snapshotMu.Lock()
	defer snapshotMu.Unlock()

	if snapshotIDs == nil {
		snapshotIDs = make(map[string]struct{}, MaxSnapshots)
	}

	id = newShortID(snapshotIDExists)
	rec.ID = id
	rec.CreatedAt = time.Now().Unix()
	snapshotIDs[id] = struct{}{}

	if len(snapshots) >= MaxSnapshots {
		removed := snapshots[0]
		delete(snapshotIDs, removed.ID)
		snapshots = snapshots[1:]
		removeSnapshotFile(removed.ID)
		queueWarning = fmt.Sprintf("snapshot queue reached maximum capacity (%d); oldest snapshot evicted. The file was written successfully.", MaxSnapshots)
	}

	snapshots = append(snapshots, rec)
	if snapshotPersistEnabled() {
		persistSnapshot(rec)
	}

	beforeCap := len(snapshots)
	evictSnapshotsByCapacityLocked()
	if len(snapshots) < beforeCap {
		if queueWarning != "" {
			queueWarning += " "
		}
		queueWarning += fmt.Sprintf("snapshot queue exceeded maximum disk capacity (%d bytes); oldest snapshot(s) evicted. The file was written successfully.", maxSnapshotTotalBytes)
	}

	return id, queueWarning
}

// snapshotIDExists checks if a short ID is already in use.
func snapshotIDExists(id string) bool {
	_, ok := snapshotIDs[id]
	return ok
}

// persistSnapshot writes a snapshot record to disk. Failures are best-effort
// and do not block the editing operation.
func persistSnapshot(record SnapshotRecord) {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(SnapshotDir(), fmt.Sprintf("snapshot-%s.json", record.ID))
	_ = os.WriteFile(path, data, 0o644)
}

// removeSnapshotFile deletes a snapshot file from disk. Failures are ignored.
func removeSnapshotFile(id string) {
	path := filepath.Join(SnapshotDir(), fmt.Sprintf("snapshot-%s.json", id))
	_ = os.Remove(path)
}

// CommitSnapshots clears ALL pending snapshots from the queue and deletes
// their persisted files on disk.
// Returns the number of snapshots committed.
func CommitSnapshots() int {
	snapshotMu.Lock()
	n := len(snapshots)
	snapshots = nil
	snapshotIDs = nil
	snapshotMu.Unlock()

	dir := SnapshotDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return n
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "snapshot-") && strings.HasSuffix(name, ".json") {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
	return n
}

// RollbackSnapshots rolls back the last N snapshots in LIFO order.
// For each snapshot, it restores the complete Before file content.
// Returns the number successfully rolled back, and any errors encountered.
// If an error occurs mid-rollback, it continues with remaining items.
func RollbackSnapshots(step int) (count int, errors []error) {
	ensureSnapshotStore()

	snapshotMu.Lock()

	if step <= 0 || len(snapshots) == 0 {
		snapshotMu.Unlock()
		return 0, nil
	}

	if step > len(snapshots) {
		step = len(snapshots)
	}

	// Get the requested snapshots (newest last in the slice)
	toRollback := make([]SnapshotRecord, step)
	copy(toRollback, snapshots[len(snapshots)-step:])

	// Remove them from the queue
	snapshots = snapshots[:len(snapshots)-step]
	snapshotIDs = make(map[string]struct{}, len(snapshots))
	for _, s := range snapshots {
		snapshotIDs[s.ID] = struct{}{}
	}

	snapshotMu.Unlock()

	// Roll back in LIFO order (newest first)
	for i := len(toRollback) - 1; i >= 0; i-- {
		rec := toRollback[i]

		content := strings.Join(rec.Before.Lines, "")

		mode := os.FileMode(0o644)
		if info, err := os.Stat(rec.File); err == nil {
			mode = info.Mode().Perm()
		}

		if err := os.WriteFile(rec.File, []byte(content), mode); err != nil {
			errors = append(errors, writePath(rec.File, err))
			continue
		}

		removeSnapshotFile(rec.ID)
		count++
	}

	return count, errors
}

// ListSnapshots returns a copy of all pending snapshots (newest first).
func ListSnapshots() []SnapshotRecord {
	ensureSnapshotStore()

	snapshotMu.Lock()
	defer snapshotMu.Unlock()

	result := make([]SnapshotRecord, len(snapshots))
	for i, s := range snapshots {
		result[len(snapshots)-1-i] = s
	}
	return result
}

// QueueStats returns usage info for the status tool.
type QueueStats struct {
	Used      int   `json:"used"`
	Max       int   `json:"max"`
	MemBytes  int64 `json:"mem_bytes"`
	DiskBytes int64 `json:"disk_bytes"`
}

// SnapshotQueueStats returns usage info for the status tool.
func SnapshotQueueStats() QueueStats {
	ensureSnapshotStore()

	snapshotMu.Lock()
	defer snapshotMu.Unlock()

	var mem int64
	for _, s := range snapshots {
		for _, l := range s.Before.Lines {
			mem += int64(len(l))
		}
		for _, l := range s.After.Lines {
			mem += int64(len(l))
		}
	}

	var disk int64
	dir := SnapshotDir()
	for _, s := range snapshots {
		info, err := os.Stat(filepath.Join(dir, fmt.Sprintf("snapshot-%s.json", s.ID)))
		if err == nil {
			disk += info.Size()
		}
	}

	return QueueStats{
		Used:      len(snapshots),
		Max:       MaxSnapshots,
		MemBytes:  mem,
		DiskBytes: disk,
	}
}
