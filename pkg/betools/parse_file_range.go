package betools

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseFileRange parses a file path with optional line range specification.
// Format: "path/to/file.go" or "path/to/file.go:23" or "path/to/file.go:1-3"
// Returns: file path, start line (0 if not specified), end line (0 if not specified), error
func ParseFileRange(input string) (file string, start int, end int, err error) {
	// Find the last colon that's not part of a Windows drive letter (e.g., C:\)
	colonIdx := strings.LastIndex(input, ":")
	
	// Check if this is a Windows path like C:\path\file.go
	// In that case, the first colon is part of the drive letter
	if colonIdx > 0 && input[colonIdx-1] == ':' {
		// Could be part of a protocol like file://, check for that too
		colonIdx = strings.LastIndex(input[:colonIdx-1], ":")
	}
	
	if colonIdx < 0 {
		// No colon found, return the whole input as file path
		return input, 0, 0, nil
	}
	
	file = input[:colonIdx]
	rangePart := input[colonIdx+1:]
	
	if rangePart == "" {
		// Colon but no range, return file with 0 start/end
		return file, 0, 0, nil
	}
	
	// Parse the range part
	parts := strings.SplitN(rangePart, "-", 2)
	
	if len(parts) == 1 {
		// Single line number like ":23"
		line, parseErr := strconv.Atoi(parts[0])
		if parseErr != nil {
			return input, 0, 0, fmt.Errorf("invalid line number: %s", parts[0])
		}
		if line < 1 {
			return input, 0, 0, fmt.Errorf("line number must be >= 1, got %d", line)
		}
		return file, line, line, nil
	}
	
	// Range like ":1-3"
	startLine, parseErr := strconv.Atoi(parts[0])
	if parseErr != nil {
		return input, 0, 0, fmt.Errorf("invalid start line: %s", parts[0])
	}
	
	endLine, parseErr := strconv.Atoi(parts[1])
	if parseErr != nil {
		return input, 0, 0, fmt.Errorf("invalid end line: %s", parts[1])
	}
	
	if startLine < 1 {
		return input, 0, 0, fmt.Errorf("start line must be >= 1, got %d", startLine)
	}
	
	if endLine < startLine {
		return input, 0, 0, fmt.Errorf("end line (%d) must be >= start line (%d)", endLine, startLine)
	}
	
	return file, startLine, endLine, nil
}
