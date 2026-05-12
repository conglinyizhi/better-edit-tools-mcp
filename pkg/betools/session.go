package betools

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// ReadSession records what was shown in a be-show call.
type ReadSession struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	LineCount int    `json:"line_count"`
	CreatedAt int64  `json:"created_at"`
}

var (
	sessionMu   sync.RWMutex
	sessions    = make(map[string]*ReadSession) // uuid → session
	cleanupOnce sync.Once
)

const sessionTTL = 24 * time.Hour

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// UUID v4: 13th nibble = 4, 17th nibble in [8,9,a,b]
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// CreateSession stores a read session and returns its UUID.
func CreateSession(file string, start, end int) string {
	cleanupOnce.Do(startCleanupRoutine)

	s := &ReadSession{
		File:      file,
		StartLine: start,
		EndLine:   end,
		LineCount: end - start + 1,
		CreatedAt: time.Now().Unix(),
	}
	id := newUUID()

	sessionMu.Lock()
	sessions[id] = s
	sessionMu.Unlock()
	return id
}

// GetSession retrieves a read session by UUID. Returns nil if not found or expired.
func GetSession(id string) *ReadSession {
	sessionMu.RLock()
	s := sessions[id]
	sessionMu.RUnlock()
	return s
}

// SessionFromCache is a convenience wrapper: looks up the session, and if
// the file content has changed (line count mismatch), lines, sample from
// head/tail of the original range are collected to help the LLM re-sync.
// Returns the session, a warning string (empty if clean), and any error.
//
// The warning is non-fatal — the session is still returned so the caller can
// proceed with the edit if they choose.
func SessionFromCache(id string) (s *ReadSession, warning string) {
	s = GetSession(id)
	if s == nil {
		return nil, "session not found or expired (re-read the file)"
	}

	// Verify line count.
	lines, _, err := readLines(s.File)
	if err != nil {
		return s, fmt.Sprintf("session %q: can't read file — %v", id, err)
	}

	actualSlice := lines[s.StartLine-1 : s.EndLine]
	currentCount := len(actualSlice)

	if currentCount == s.LineCount {
		return s, "" // clean
	}

	// Line count mismatch — build a re-sync hint.
	head := actualSlice[0]
	tail := actualSlice[currentCount-1]
	warning = fmt.Sprintf(
		"⚠️  Content verification: read session %q expected %d lines (L%d–%d) but found %d lines. "+
			"The file may have been modified since reading.\n"+
			"First line of range: %s\nLast line of range: %s\n"+
			"You can proceed if the target is still correct, or re-read with be-show first.",
		id, s.LineCount, s.StartLine, s.EndLine, currentCount,
		trimForWarning(head), trimForWarning(tail),
	)
	return s, warning
}

func trimForWarning(s string) string {
	runes := []rune(s)
	if len(runes) > 80 {
		return string(runes[:80]) + "…"
	}
	return s
}

// CleanupSession removes a session from the cache.
func CleanupSession(id string) {
	sessionMu.Lock()
	delete(sessions, id)
	sessionMu.Unlock()
}

func startCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-sessionTTL).Unix()
			sessionMu.Lock()
			for id, s := range sessions {
				if s.CreatedAt < cutoff {
					delete(sessions, id)
				}
			}
			sessionMu.Unlock()
		}
	}()
}
