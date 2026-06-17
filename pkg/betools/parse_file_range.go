package betools

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParseFileRange parses a file path with optional line range specification.
// Format: "path/to/file.go" or "path/to/file.go:23" or "path/to/file.go:1-3"
// Returns: file path, start line (0 if not specified), end line (0 if not specified), error
func ParseFileRange(input string) (file string, start int, end int, err error) {
	searchStart := 0

	// Explicitly detect Windows drive letters (e.g., C:\path\file.go).
	// The drive-letter colon must not be treated as a range separator.
	if len(input) >= 3 && input[1] == ':' &&
		((input[0] >= 'A' && input[0] <= 'Z') || (input[0] >= 'a' && input[0] <= 'z')) &&
		(input[2] == '\\' || input[2] == '/') {
		searchStart = 2
	} else if protoEnd := strings.Index(input, "://"); protoEnd >= 0 {
		// For URL-like inputs (file://host:port/path), only consider colons
		// that appear in the path portion, after the host:port authority.
		authorityStart := protoEnd + 3
		if slashIdx := strings.Index(input[authorityStart:], "/"); slashIdx >= 0 {
			searchStart = authorityStart + slashIdx
		} else {
			// No path component; the whole string is an authority, not a file path.
			return input, 0, 0, nil
		}
	}

	// Find the last colon within the searchable portion of the input.
	colonIdx := strings.LastIndex(input[searchStart:], ":")
	if colonIdx >= 0 {
		colonIdx += searchStart
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
		// Single line number like ":23" or the special ":ALL" marker.
		if parts[0] == "ALL" {
			return file, -1, -1, nil
		}
		line, parseErr := strconv.Atoi(parts[0])
		if parseErr != nil {
			return input, 0, 0, invalidArg(fmt.Sprintf("invalid line number: %s", parts[0]))
		}
		if line < 0 {
			return input, 0, 0, invalidArg(fmt.Sprintf("line number must be >= 0, got %d", line))
		}
		return file, line, line, nil
	}

	// Range like ":1-3"
	startLine, parseErr := strconv.Atoi(parts[0])
	if parseErr != nil {
		return input, 0, 0, invalidArg(fmt.Sprintf("invalid start line: %s", parts[0]))
	}

	endLine, parseErr := strconv.Atoi(parts[1])
	if parseErr != nil {
		return input, 0, 0, invalidArg(fmt.Sprintf("invalid end line: %s", parts[1]))
	}

	if startLine < 0 {
		return input, 0, 0, invalidArg(fmt.Sprintf("start line must be >= 0, got %d", startLine))
	}

	if endLine < startLine {
		return input, 0, 0, invalidArg(fmt.Sprintf("end line (%d) must be >= start line (%d)", endLine, startLine))
	}

	return file, startLine, endLine, nil
}

// fileRangeSuffix matches an explicit line range suffix such as :10, :5-15 or :ALL.
var fileRangeSuffix = regexp.MustCompile(`:(?:\d+(-\d+)?|ALL)$`)

// HasFileRange reports whether the input string ends with an explicit line
// range suffix like :10, :5-15 or :ALL.
func HasFileRange(file string) bool {
	return fileRangeSuffix.MatchString(file)
}
