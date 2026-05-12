// Package betools provides file editing primitives for MCP-powered editors and
// AI agent frameworks.
//
// It offers atomic write operations, batch editing with automatic bottom-to-top
// sorting, structural balance checking, function- and tag-scope detection, and
// a session bridge that lets an agent validate file consistency between a read
// and a subsequent write.
//
// All write operations use a temp-file-then-rename cycle, so a crash during a
// write never corrupts the original file.
//
// # Session Bridge
//
// Show returns a session_id (UUID v4). Later Replace calls can pass that ID to
// detect whether the file has been modified since the read. A non-fatal warning
// is returned with sample lines to help the agent re-synchronise.
//
// # Degraded JSON Parsing
//
// Write automatically degrades to a character-level extraction when standard
// JSON parsing fails. This is designed for AI-generated content that may
// contain unescaped backticks, ${} substitutions, or stray quotes.
//
// # Sentinel Errors
//
//	ErrInvalid — invalid arguments
//	ErrRead    — file read failures
//	ErrWrite   — file write failures
//
// Use errors.Is to match against these sentinels.
//
// Minimum Go version: Go 1.23+
package betools
