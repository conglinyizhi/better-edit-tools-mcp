package betools

import (
	"fmt"
	"os"
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
type SnapshotRange struct {
	Start int      `json:"start"`
	End   int      `json:"end"`
	Lines []string `json:"lines"`
}

// MaxSnapshots is the maximum number of pending snapshots in the queue.
const MaxSnapshots = 30

var (
	snapshotMu  sync.Mutex
	snapshots   []SnapshotRecord
	snapshotIDs map[string]struct{}
)

// PushSnapshot pushes a record onto the queue. If full, evicts oldest.
// Returns the record ID and a warning string (empty if no issue, non-empty if eviction happened).
// The file modification itself is NOT affected by queue capacity — it already completed.
func PushSnapshot(rec SnapshotRecord) (id string, queueWarning string) {
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
		queueWarning = fmt.Sprintf("snapshot queue reached maximum capacity (%d); oldest snapshot evicted. The file was written successfully.", MaxSnapshots)
	}

	snapshots = append(snapshots, rec)
	return id, queueWarning
}

// snapshotIDExists checks if a short ID is already in use.
func snapshotIDExists(id string) bool {
	_, ok := snapshotIDs[id]
	return ok
}

// CommitSnapshots clears ALL pending snapshots from the queue.
// Returns the number of snapshots committed.
func CommitSnapshots() int {
	snapshotMu.Lock()
	defer snapshotMu.Unlock()

	n := len(snapshots)
	snapshots = nil
	snapshotIDs = nil
	return n
}

// RollbackSnapshots rolls back the last N snapshots in LIFO order.
// For each snapshot, it writes the Before content back to the file.
// Returns the number successfully rolled back, and any errors encountered.
// If an error occurs mid-rollback, it continues with remaining items.
func RollbackSnapshots(step int) (count int, errors []error) {
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

		data, err := os.ReadFile(rec.File)
		if err != nil {
			errors = append(errors, fmt.Errorf("read %s: %w", rec.File, err))
			continue
		}

		content := string(data)
		le := detectLineEnding(content)
		lines := splitKeepLineEnding(content, le)

		beforeStart := rec.Before.Start
		beforeEnd := rec.Before.End

		if beforeStart < 1 || beforeStart > len(lines) {
			errors = append(errors, fmt.Errorf("%s: before start %d out of current range (1-%d)", rec.File, beforeStart, len(lines)))
			continue
		}
		if beforeEnd < beforeStart {
			beforeEnd = beforeStart
		}
		if beforeEnd > len(lines) {
			beforeEnd = len(lines)
		}

		newLines := make([]string, 0, len(lines)-len(lines[beforeStart-1:beforeEnd])+len(rec.Before.Lines))
		newLines = append(newLines, lines[:beforeStart-1]...)
		newLines = append(newLines, rec.Before.Lines...)
		newLines = append(newLines, lines[beforeEnd:]...)

		newContent := strings.Join(newLines, "")
		if err := os.WriteFile(rec.File, []byte(newContent), 0o644); err != nil {
			errors = append(errors, fmt.Errorf("write %s: %w", rec.File, err))
			continue
		}

		count++
	}

	return count, errors
}

// ListSnapshots returns a copy of all pending snapshots (newest first).
func ListSnapshots() []SnapshotRecord {
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
	Used     int   `json:"used"`
	Max      int   `json:"max"`
	MemBytes int64 `json:"mem_bytes"`
}

// SnapshotQueueStats returns usage info for the status tool.
func SnapshotQueueStats() QueueStats {
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

	return QueueStats{
		Used:     len(snapshots),
		Max:      MaxSnapshots,
		MemBytes: mem,
	}
}
