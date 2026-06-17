package betools

import (
	"fmt"
	"strings"
)

func ResolveTargetSpan(path string, target ContentTarget, opts ...Option) (TargetSpan, error) {
	lines, _, err := readLines(path, opts...)
	if err != nil {
		return TargetSpan{}, readPath(path, err)
	}
	if len(lines) == 0 {
		return TargetSpan{}, invalidArg("target: empty file")
	}

	switch target.Kind {
	case "line":
		line := parseTargetLine(target.Value)
		if line < 1 || line > len(lines) {
			return TargetSpan{}, invalidArg(fmt.Sprintf("target line %d out of range (1..%d)", line, len(lines)))
		}
		return TargetSpan{Start: line, End: line}, nil
	case "marker":
		needle := strings.TrimSpace(target.Value)
		if needle == "" {
			return TargetSpan{}, invalidArg("marker must not be empty")
		}
		for idx, line := range lines {
			if strings.Contains(line, needle) {
				return TargetSpan{Start: idx + 1, End: idx + 1}, nil
			}
		}
		return TargetSpan{}, invalidArg("marker not found: " + needle)
	case "function":
		needle := strings.TrimSpace(target.Value)
		if needle == "" {
			return TargetSpan{}, invalidArg("function must not be empty")
		}
		found := 0
		// First pass: language-aware definition prefixes.
		for idx, line := range lines {
			if isFunctionDefinition(line, needle) {
				found = idx + 1
				break
			}
		}
		// Fallback: match calls or definitions that lack an obvious prefix.
		if found == 0 {
			for idx, line := range lines {
				if strings.Contains(line, needle+"(") {
					found = idx + 1
					break
				}
			}
		}
		if found == 0 {
			return TargetSpan{}, invalidArg("function not found: " + needle)
		}
		start, end, err := functionRangeRaw(path, found, opts...)
		if err != nil {
			return TargetSpan{}, err
		}
		return TargetSpan{Start: start, End: end}, nil
	case "tag":
		needle := strings.TrimSpace(target.Value)
		if needle == "" {
			return TargetSpan{}, invalidArg("tag must not be empty")
		}
		found := 0
		for idx, line := range lines {
			if strings.Contains(line, "<"+needle) || strings.Contains(line, "</"+needle) {
				found = idx + 1
				break
			}
		}
		if found == 0 {
			return TargetSpan{}, invalidArg("tag not found: " + needle)
		}
		tag, err := TagRange(path, found, opts...)
		if err != nil {
			return TargetSpan{}, err
		}
		return TargetSpan{Start: tag.Start, End: tag.End}, nil
	default:
		return TargetSpan{}, invalidArg("未知 target 类型: " + target.Kind)
	}
}

func parseTargetLine(value string) int {
	if value == "" {
		return 0
	}
	var n int
	_, _ = fmt.Sscanf(value, "%d", &n)
	return n
}

// languageDefinitionPrefixes lists common function-definition keywords.
var languageDefinitionPrefixes = []string{"func ", "def ", "fn ", "function "}

// isFunctionDefinition reports whether line looks like a definition of needle
// rather than a call site. It recognizes language-specific definition prefixes
// (e.g., Go's "func ", Python's "def ", Rust's "fn ") and tolerates a leading
// Go receiver list.
func isFunctionDefinition(line, needle string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	for _, prefix := range languageDefinitionPrefixes {
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		after := strings.TrimLeft(trimmed[len(prefix):], " \t")
		// Handle Go method receivers: func (r *Receiver) Name(...)
		if strings.HasPrefix(after, "(") {
			if idx := strings.Index(after, ")"); idx >= 0 {
				after = strings.TrimLeft(after[idx+1:], " \t")
			}
		}
		if !strings.HasPrefix(after, needle) {
			continue
		}
		next := ""
		if len(after) > len(needle) {
			next = after[len(needle):]
		}
		// Needle must terminate at the start of the parameter/type list or body.
		if next == "" || strings.Contains(" \t(<[{", string(next[0])) {
			return true
		}
	}
	return false
}

func readText(path string, opts ...Option) (string, error) {
	lines, _, err := readLines(path, opts...)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, ""), nil
}
