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
		for idx, line := range lines {
			if strings.Contains(line, "fn "+needle) || strings.Contains(line, needle+"(") {
				found = idx + 1
				break
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

func readText(path string, opts ...Option) (string, error) {
	lines, _, err := readLines(path, opts...)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, ""), nil
}
