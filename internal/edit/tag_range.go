package edit

import (
	"fmt"
	"strings"
)

func TagRange(path string, line int) (TagRangeResult, error) {
	content, err := ReadText(path)
	if err != nil {
		return TagRangeResult{}, ReadPath(path, err)
	}
	lines := strings.Split(content, "\n")
	if line < 1 || line > len(lines) {
		return TagRangeResult{}, InvalidArg(fmt.Sprintf("目标行 %d 超出文件范围 (1..%d)", line, len(lines)))
	}

	type tagEntry struct {
		name string
		line int
	}
	var stack []tagEntry
	var openRanges []TagRangeResult
	voidElements := map[string]struct{}{
		"area": {}, "base": {}, "br": {}, "col": {}, "embed": {}, "hr": {}, "img": {},
		"input": {}, "link": {}, "meta": {}, "param": {}, "source": {}, "track": {}, "wbr": {},
	}

	for idx, rawLine := range lines {
		lineNo := idx + 1
		chars := []byte(rawLine)
		for cursor := 0; cursor < len(chars); cursor++ {
			if chars[cursor] != '<' {
				continue
			}
			j := cursor + 1
			if j < len(chars) && chars[j] == '!' {
				for j < len(chars) && chars[j] != '>' {
					j++
				}
				cursor = j
				continue
			}
			if j < len(chars) && chars[j] == '/' {
				j++
				tag := readTagName(chars, &j)
				for j < len(chars) && chars[j] != '>' {
					j++
				}
				if len(stack) > 0 {
					last := stack[len(stack)-1]
					if last.name == tag {
						stack = stack[:len(stack)-1]
						openRanges = append(openRanges, TagRangeResult{Start: last.line, End: lineNo, Kind: last.name, Tag: last.name})
					}
				}
				cursor = j
				continue
			}
			tag := readTagName(chars, &j)
			if tag != "" {
				trimmed := strings.TrimRight(string(chars[cursor:]), " \t")
				if strings.HasSuffix(trimmed, "/>") {
					for j < len(chars) && chars[j] != '>' {
						j++
					}
					cursor = j
					continue
				}
				if _, ok := voidElements[strings.ToLower(tag)]; !ok {
					stack = append(stack, tagEntry{name: strings.ToLower(tag), line: lineNo})
				}
			}
			for j < len(chars) && chars[j] != '>' {
				j++
			}
			cursor = j
		}
	}

	for _, rg := range openRanges {
		if rg.Start <= line && line <= rg.End {
			return rg, nil
		}
	}
	return TagRangeResult{}, InvalidArg(fmt.Sprintf("第 %d 行不在任何可配对 tag 范围内", line))
}

func readTagName(chars []byte, idx *int) string {
	j := *idx
	var tag strings.Builder
	for j < len(chars) {
		ch := chars[j]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == ':' {
			tag.WriteByte(ch)
			j++
			continue
		}
		break
	}
	*idx = j
	return tag.String()
}
