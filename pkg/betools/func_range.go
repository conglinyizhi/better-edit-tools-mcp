package betools

import (
	"fmt"
	"strings"
)

func functionRangeRaw(path string, targetLine int, opts ...Option) (int, int, error) {
	content, err := readText(path, opts...)
	if err != nil {
		return 0, 0, readPath(path, err)
	}
	lines := strings.Split(content, "\n")
	if targetLine < 1 || targetLine > len(lines) {
		return 0, 0, invalidArg(fmt.Sprintf("target line %d out of range (1..%d)", targetLine, len(lines)))
	}

	type commentState int
	const (
		commentNone commentState = iota
		commentLine
		commentBlock
	)
	depth := 0
	inString := false
	var stringChar byte
	escapeNext := false
	state := commentNone
	var currentStart *int
	var ranges [][2]int

	for lineIdx, line := range lines {
		bytes := []byte(line)
		if state == commentBlock {
			// keep block comment until it closes
		}
		if state == commentLine {
			state = commentNone
		}
		for col := 0; col < len(bytes); col++ {
			ch := bytes[col]
			var next byte
			if col+1 < len(bytes) {
				next = bytes[col+1]
			}
			if escapeNext {
				escapeNext = false
				continue
			}
			if !inString && state != commentBlock && ch == '/' && next == '/' {
				state = commentLine
				break
			}
			if !inString && state != commentBlock && ch == '/' && next == '*' {
				state = commentBlock
				col++
				continue
			}
			if state == commentBlock && ch == '*' && next == '/' {
				state = commentNone
				col++
				continue
			}
			if state == commentBlock || state == commentLine {
				continue
			}
			if (ch == '"' || ch == '\'' || ch == '`') && !inString {
				inString = true
				stringChar = ch
				continue
			}
			if inString && ch == stringChar {
				inString = false
				continue
			}
			if inString && ch == '\\' {
				escapeNext = true
				continue
			}
			if inString {
				continue
			}
			if ch == '{' {
				if depth == 0 {
					start := lineIdx + 1
					currentStart = &start
				}
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 && currentStart != nil {
					ranges = append(ranges, [2]int{*currentStart, lineIdx + 1})
					currentStart = nil
				}
				if depth < 0 {
					depth = 0
				}
			}
		}
	}

	for _, rg := range ranges {
		if rg[0] <= targetLine && targetLine <= rg[1] {
			// Walk backwards from the brace block start to find the signature line
			// Stop at function boundaries (lines with '}') or blank lines
			sigStart := rg[0]
			for i := rg[0] - 2; i >= 0; i-- {
				l := strings.TrimSpace(lines[i])
				// Stop at function boundaries — a line with only '}' means we
				// crossed into a preceding function's closing brace
				if l == "}" {
					break
				}
				// Stop at blank lines (they separate function signatures)
				if len(l) == 0 {
					break
				}
				if strings.HasPrefix(l, "func ") || strings.HasPrefix(l, "type ") ||
					strings.HasPrefix(l, "func (") || strings.HasPrefix(l, "func[") ||
					strings.HasPrefix(l, "} func ") || strings.HasPrefix(l, "} func (") {
					sigStart = i + 1
					break
				}
				// Heuristic: a line ending with "{" that's not just "{" is the signature
				if strings.HasSuffix(l, "{") && l != "{" {
					sigStart = i + 1
					break
				}
			}
			return sigStart, rg[1], nil
		}
	}
	return 0, 0, invalidArg(fmt.Sprintf("line %d is not inside any function/block (brace detection)", targetLine))
}
func FuncRange(path string, line int, opts ...Option) (FunctionRangeResult, error) {
	start, end, err := functionRangeRaw(path, line, opts...)
	if err != nil {
		return FunctionRangeResult{}, err
	}
	return FunctionRangeResult{Start: start, End: end}, nil
}
