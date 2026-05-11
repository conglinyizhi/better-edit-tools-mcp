package edit

import (
	"fmt"
	"os"
	"strings"
)

func ResolveTargetSpan(path string, target ContentTarget) (TargetSpan, error) {
	lines, _, err := ReadLines(path)
	if err != nil {
		return TargetSpan{}, ReadPath(path, err)
	}
	if len(lines) == 0 {
		return TargetSpan{}, InvalidArg("target: 文件为空")
	}

	switch target.Kind {
	case "line":
		line := parseTargetLine(target.Value)
		if line < 1 || line > len(lines) {
			return TargetSpan{}, InvalidArg(fmt.Sprintf("target line %d 超出文件范围 (1..%d)", line, len(lines)))
		}
		return TargetSpan{Start: line, End: line}, nil
	case "marker":
		needle := strings.TrimSpace(target.Value)
		if needle == "" {
			return TargetSpan{}, InvalidArg("marker 不能为空")
		}
		for idx, line := range lines {
			if strings.Contains(line, needle) {
				return TargetSpan{Start: idx + 1, End: idx + 1}, nil
			}
		}
		return TargetSpan{}, InvalidArg("未找到 marker: " + needle)
	case "function":
		needle := strings.TrimSpace(target.Value)
		if needle == "" {
			return TargetSpan{}, InvalidArg("function 不能为空")
		}
		found := 0
		for idx, line := range lines {
			if strings.Contains(line, "fn "+needle) || strings.Contains(line, needle+"(") {
				found = idx + 1
				break
			}
		}
		if found == 0 {
			return TargetSpan{}, InvalidArg("未找到 function: " + needle)
		}
		start, end, err := FunctionRangeRaw(path, found)
		if err != nil {
			return TargetSpan{}, err
		}
		return TargetSpan{Start: start, End: end}, nil
	case "tag":
		needle := strings.TrimSpace(target.Value)
		if needle == "" {
			return TargetSpan{}, InvalidArg("tag 不能为空")
		}
		found := 0
		for idx, line := range lines {
			if strings.Contains(line, "<"+needle) || strings.Contains(line, "</"+needle) {
				found = idx + 1
				break
			}
		}
		if found == 0 {
			return TargetSpan{}, InvalidArg("未找到 tag: " + needle)
		}
		tag, err := TagRange(path, found)
		if err != nil {
			return TargetSpan{}, err
		}
		return TargetSpan{Start: tag.Start, End: tag.End}, nil
	default:
		return TargetSpan{}, InvalidArg("未知 target 类型: " + target.Kind)
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

func ReadText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
