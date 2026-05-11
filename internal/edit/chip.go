package edit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	maxChips    = 10
	chipDir     = "/tmp/bet-chips"
)

// ChipRecord stores the original arguments from a failed tool call.
type ChipRecord struct {
	ID     int                    `json:"id"`
	Tool   string                 `json:"tool"`
	Args   map[string]any         `json:"args"`
}

// chipStore is a global FIFO queue of chip records, protected by a mutex.
var (
	chipMu    sync.Mutex
	chipStore []ChipRecord
	chipSeq   int // monotonic ID counter
)

func init() {
	os.MkdirAll(chipDir, 0755)
}

// SaveChip stores the tool arguments as a chip record.
// Called automatically when a tool errors and the args JSON length > 50.
// Returns the chip ID, or 0 if args were too short to record.
func SaveChip(tool string, args map[string]any) int {
	b, _ := json.Marshal(args)
	if len(b) <= 50 {
		return 0
	}

	chipMu.Lock()
	defer chipMu.Unlock()

	chipSeq++
	id := chipSeq

	record := ChipRecord{
		ID:   id,
		Tool: tool,
		Args: args,
	}

	// FIFO: append, pop oldest if over limit
	chipStore = append(chipStore, record)
	if len(chipStore) > maxChips {
		chipStore = chipStore[1:]
	}

	// Also persist to /tmp/bet-chips/
	persistChip(record)

	return id
}

func persistChip(record ChipRecord) {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(chipDir, fmt.Sprintf("chip-%d.json", record.ID))
	// Ignore write errors — chip storage is best-effort
	_ = os.WriteFile(path, data, 0644)
}

// GetChip retrieves a chip record by ID from the in-memory store.
// Falls back to reading from disk if not in memory (process restart).
func GetChip(id int) (*ChipRecord, error) {
	chipMu.Lock()
	for _, c := range chipStore {
		if c.ID == id {
			chipMu.Unlock()
			return &c, nil
		}
	}
	chipMu.Unlock()

	// Fallback: try reading from disk
	path := filepath.Join(chipDir, fmt.Sprintf("chip-%d.json", id))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("chip %d not found", id)
	}
	var rec ChipRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("chip %d corrupt: %v", id, err)
	}
	return &rec, nil
}

// ListChips returns all chip IDs currently in memory, in order (oldest first).
func ListChips() []int {
	chipMu.Lock()
	defer chipMu.Unlock()
	ids := make([]int, len(chipStore))
	for i, c := range chipStore {
		ids[i] = c.ID
	}
	return ids
}

// ReadFileContent reads a text file and returns its full content as a string.
// Returns an error if the file is binary (detected by null bytes in first 8KB).
func ReadFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %s: %v", path, err)
	}
	if isBinary(data) {
		return "", fmt.Errorf("file %s appears to be binary, refusing to read", path)
	}
	return string(data), nil
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
