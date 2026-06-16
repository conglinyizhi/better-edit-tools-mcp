package betools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	maxChips = 30
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
	chipMu      sync.Mutex
	chipStore   []ChipRecord
	chipIDSet   map[string]struct{}
	chipDirInit sync.Once
)

// ChipDir returns the platform-appropriate chip cache directory.
//   - Windows: %LOCALAPPDATA%/better-edit-tools-mcp/chips
//   - Linux/macOS: $XDG_CACHE_HOME/better-edit-tools-mcp/chips
//     or ~/.cache/better-edit-tools-mcp/chips
//   - Fallback: /tmp/better-edit-tools-mcp-chips
func ChipDir() string {
	switch runtime.GOOS {
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "better-edit-tools-mcp", "chips")
		}
	default:
		if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
			return filepath.Join(xdgCache, "better-edit-tools-mcp", "chips")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".cache", "better-edit-tools-mcp", "chips")
		}
	}
	return "/tmp/better-edit-tools-mcp-chips"
}

func init() {
	chipDirInit.Do(func() {
		dir := ChipDir()
		_ = os.MkdirAll(dir, 0755)
		loadChipsFromDisk()
	})
}

// loadChipsFromDisk reads all chip JSON files from the cache directory
// into the in-memory store, restoring state from a previous process run.
func loadChipsFromDisk() {
	dir := ChipDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if chipIDSet == nil {
		chipIDSet = make(map[string]struct{}, maxChips)
	}
	var loaded []ChipRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			continue
		}
		var rec ChipRecord
		if jsonErr := json.Unmarshal(data, &rec); jsonErr != nil {
			continue
		}
		loaded = append(loaded, rec)
		chipIDSet[rec.ID] = struct{}{}
	}
	// Sort by CreatedAt to preserve chronological order
	for i := 0; i < len(loaded); i++ {
		for j := i + 1; j < len(loaded); j++ {
			if loaded[j].CreatedAt < loaded[i].CreatedAt {
				loaded[i], loaded[j] = loaded[j], loaded[i]
			}
		}
	}
	// Trim to maxChips if too many on disk
	if len(loaded) > maxChips {
		for _, r := range loaded[:len(loaded)-maxChips] {
			delete(chipIDSet, r.ID)
			removeChipFile(r.ID)
		}
		loaded = loaded[len(loaded)-maxChips:]
	}
	chipStore = loaded
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
	path := filepath.Join(ChipDir(), fmt.Sprintf("chip-%s.json", record.ID))
	// Ignore write errors — chip storage is best-effort
	_ = os.WriteFile(path, data, 0644)
}

func removeChipFile(id string) {
	path := filepath.Join(ChipDir(), fmt.Sprintf("chip-%s.json", id))
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

	path := filepath.Join(ChipDir(), fmt.Sprintf("chip-%s.json", id))
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
