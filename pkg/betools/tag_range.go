package betools

import (
	"fmt"
	"strings"
)

func TagRange(path string, line int, opts ...Option) (TagRangeResult, error) {
	content, err := readText(path, opts...)
	if err != nil {
		return TagRangeResult{}, readPath(path, err)
	}
	lines := strings.Split(content, "\n")
	if line < 1 || line > len(lines) {
		return TagRangeResult{}, invalidArg(fmt.Sprintf("target line %d out of range (1..%d)", line, len(lines)))
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
			fullTag, nextIndex, ok := readFullTag(chars, cursor)
			if !ok {
				continue
			}
			cursor = nextIndex
			if strings.HasPrefix(fullTag, "<!") || strings.HasPrefix(fullTag, "<?") {
				continue
			}
			if strings.HasPrefix(fullTag, "</") {
				tagName := extractTagName(fullTag, true)
				if tagName == "" {
					continue
				}
				tagName = strings.ToLower(tagName)
				if len(stack) > 0 {
					last := stack[len(stack)-1]
					if last.name == tagName {
						stack = stack[:len(stack)-1]
						openRanges = append(openRanges, TagRangeResult{Start: last.line, End: lineNo, Kind: last.name, Tag: last.name})
					}
				}
				continue
			}
			tagName := extractTagName(fullTag, false)
			if tagName == "" {
				continue
			}
			trimmed := strings.TrimSpace(fullTag)
			if strings.HasSuffix(trimmed, "/>") {
				continue
			}
			if _, ok := voidElements[strings.ToLower(tagName)]; !ok {
				stack = append(stack, tagEntry{name: strings.ToLower(tagName), line: lineNo})
			}
		}
	}

	for _, rg := range openRanges {
		if rg.Start <= line && line <= rg.End {
			return rg, nil
		}
	}
	return TagRangeResult{}, invalidArg(fmt.Sprintf("line %d is not inside any paired tag", line))
}

