package betools

import (
	"crypto/rand"
	"encoding/hex"
)

// newShortID generates a 6-character hex string (3 bytes → 6 hex chars).
// The exists callback is called to check for collisions; it retries up to
// 5 times before falling back to a 12-character ID (extremely unlikely).
func newShortID(exists func(string) bool) string {
	for attempts := 0; attempts < 5; attempts++ {
		id := randomHex(3)
		if !exists(id) {
			return id
		}
	}
	// Fallback: 6 bytes → 12 hex chars, astronomically unlikely to collide.
	return randomHex(6)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
