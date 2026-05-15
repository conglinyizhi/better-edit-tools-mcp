package betools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	maxChips = 30
	chipDir  = "/tmp/bet-chips"
)

// ChipRecord stores the original arguments from a failed tool call.
type ChipRecord struct {
	ID        string         `json:"id"`
	Tool      string         `json:"tool"`
	Args      map[string]any `json:"args"`
	ErrMsg    string         `json:"err_msg,omitempty"`
	CreatedAt int64          `json:"created_at"`
}

// chipStore is a global FIFO queue of chip records, protected by a mutex.
var (
	chipMu     sync.Mutex
	chipStore  []ChipRecord
	chipIDSet  map[string]struct{}
	chipDirInit sync.Once
)

func init() {
	chipDirInit.Do(ensureChipDir)
}

func ensureChipDir() {
	_ = os.MkdirAll(chipDir, 0755)
}

// chipIDExists checks whether an ID is already in use.
func chipIDExists(id string) bool {
	_, ok := chipIDSet[id]
	return ok
}

// SaveChip stores the tool arguments as a chip record.
// Called automatically when a tool errors and the args JSON length > 50.
// Returns the chip ID, or "" if args were too short to record.
func SaveChip(tool string, args map[string]any, errMsg string) string {
	b, _ := json.Marshal(args)
	if len(b) <= 50 {
		return ""
	}

	chipMu.Lock()
	defer chipMu.Unlock()

	if chipIDSet == nil {
		chipIDSet = make(map[string]struct{}, maxChips)
	}

	id := newShortID(chipIDExists)

	record := ChipRecord{
		ID:        id,
		Tool:      tool,
		Args:      args,
		ErrMsg:    errMsg,
		CreatedAt: time.Now().Unix(),
	}

	chipIDSet[id] = struct{}{}
	chipStore = append(chipStore, record)
	if len(chipStore) > maxChips {
		removed := chipStore[0]
		delete(chipIDSet, removed.ID)
		chipStore = chipStore[1:]
		removeChipFile(removed.ID)
	}

	persistChip(record)

	return id
}

func persistChip(record ChipRecord) {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(chipDir, fmt.Sprintf("chip-%s.json", record.ID))
	// Ignore write errors — chip storage is best-effort
	_ = os.WriteFile(path, data, 0644)
}

func removeChipFile(id string) {
	path := filepath.Join(chipDir, fmt.Sprintf("chip-%s.json", id))
	_ = os.Remove(path)
}

// GetChip retrieves a chip record by ID from the in-memory store.
// Falls back to reading from disk if not in memory (process restart).
func GetChip(id string) (*ChipRecord, error) {
	chipMu.Lock()
	for _, c := range chipStore {
		if c.ID == id {
			chipMu.Unlock()
			return &c, nil
		}
	}
	chipMu.Unlock()

	path := filepath.Join(chipDir, fmt.Sprintf("chip-%s.json", id))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("chip %s not found", id)
	}
	var rec ChipRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("chip %s corrupt: %v", id, err)
	}
	return &rec, nil
}

// ListChips returns all chip IDs currently in memory, in order (oldest first).
func ListChips() []string {
	chipMu.Lock()
	defer chipMu.Unlock()
	ids := make([]string, len(chipStore))
	for i, c := range chipStore {
		ids[i] = c.ID
	}
	return ids
}

func isBinary(data []byte) bool {
	const sampleSize = 8192
	n := len(data)
	if n > sampleSize {
		n = sampleSize
	}
	for _, b := range data[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

// SaveContentChip saves arbitrary content as a chip record and returns the ID.
// Unlike SaveChip (which stores tool args), this stores content directly.
// Returns the chip ID and a warning string if oldest chips were evicted.
func SaveContentChip(tool string, content string) (id string, overflowWarn string) {
	chipMu.Lock()
	defer chipMu.Unlock()

	if chipIDSet == nil {
		chipIDSet = make(map[string]struct{}, maxChips)
	}

	id = newShortID(chipIDExists)
	record := ChipRecord{
		ID:        id,
		Tool:      tool,
		Args:      map[string]any{"_content": content},
		CreatedAt: time.Now().Unix(),
	}

	chipIDSet[id] = struct{}{}
	chipStore = append(chipStore, record)

	if len(chipStore) > maxChips {
		removed := chipStore[0]
		delete(chipIDSet, removed.ID)
		chipStore = chipStore[1:]
		removeChipFile(removed.ID)
		overflowWarn = fmt.Sprintf("oldest chip %s was evicted (queue max %d)", removed.ID, maxChips)
	}

	persistChip(record)
	return id, overflowWarn
}

// ChipQueueInfo returns current chip queue metadata for status reporting.
type ChipQueueInfo struct {
	Count   int      `json:"count"`
	Max     int      `json:"max"`
	IDs     []string `json:"ids,omitempty"`
	Warning string   `json:"warning,omitempty"`
}

func ChipQueueInfoValue() ChipQueueInfo {
	chipMu.Lock()
	defer chipMu.Unlock()

	ids := make([]string, len(chipStore))
	for i, c := range chipStore {
		ids[i] = c.ID
	}
	return ChipQueueInfo{
		Count: len(chipStore),
		Max:   maxChips,
		IDs:   ids,
	}
}
