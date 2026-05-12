package betools

import (
	"encoding/json"
	"fmt"
	"strings"
)

func Write(spec string, preview bool, raw bool) (WriteResult, error) {
	var result WriteResult
	var writeSpecs []WriteSpecItem
	degraded := false

	var rawVal any
	if err := json.Unmarshal([]byte(spec), &rawVal); err == nil {
		ws, err := parseWriteValue(rawVal)
		if err != nil {
			return WriteResult{}, err
		}
		writeSpecs = ws
	} else {
		ws, err := parseSpecRaw(spec)
		if err != nil {
			return WriteResult{}, err
		}
		writeSpecs = ws
		degraded = true
	}
	if raw {
		for i := range writeSpecs {
			writeSpecs[i].Content = strings.ReplaceAll(writeSpecs[i].Content, "\\n", "\n")
		}
	}

	if !preview {
		if err := writeFilesAtomic(writeSpecs); err != nil {
			path := spec
			if len(writeSpecs) > 0 {
				path = writeSpecs[0].File
			}
			return WriteResult{}, writePath(path, err)
		}
	}

	results := make([]WriteFileResult, 0, len(writeSpecs))
	for _, item := range writeSpecs {
		results = append(results, WriteFileResult{
			File:  item.File,
			Lines: countRustLines(item.Content),
			Bytes: len(item.Content),
		})
	}
	result = WriteResult{
		Status:  "ok",
		Files:   len(results),
		Results: results,
		Preview: preview,
	}
	if degraded {
		result.Degraded = true
		result.Warning = "JSON contains unescaped control characters (tab/newline). Content extracted via degraded parser — may be incomplete. Re-read source file and verify before continuing"
	}
	return result, nil
}

func parseWriteValue(raw any) ([]WriteSpecItem, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, invalidArg("write: JSON must be an object or contain a files array")
	}

	if files, ok := m["files"].([]any); ok {
		specs := make([]WriteSpecItem, 0, len(files))
		for _, item := range files {
			spec, err := parseOneWriteItem(item)
			if err != nil {
				return nil, err
			}
			specs = append(specs, spec)
		}
		return specs, nil
	}

	spec, err := parseOneWriteItem(m)
	if err != nil {
		return nil, err
	}
	return []WriteSpecItem{spec}, nil
}

func parseOneWriteItem(raw any) (WriteSpecItem, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return WriteSpecItem{}, invalidArg("missing file field")
	}
	file, _ := m["file"].(string)
	if file == "" {
		return WriteSpecItem{}, invalidArg("missing file field")
	}
	content := ""
	switch cv := m["content"].(type) {
	case string:
		content = cv
	case fmt.Stringer:
		content = cv.String()
	default:
		if m["content"] != nil {
			content = fmt.Sprint(m["content"])
		}
	}
	if m["extract"] == true {
		content = extractCodeBlocks(content)
	}
	return WriteSpecItem{File: file, Content: content}, nil
}

func parseSpecRaw(spec string) ([]WriteSpecItem, error) {
	if idx := strings.Index(spec, "\"files\""); idx >= 0 {
		afterFiles := spec[idx+8:]
		bracket := strings.Index(afterFiles, "[")
		if bracket < 0 {
			return nil, invalidArg("files field not found after files key")
		}
		arrayStart := idx + 8 + bracket
		arrayEnd, ok := findMatching(spec, arrayStart, '[', ']')
		if !ok {
			return nil, invalidArg("no matching ] for files array")
		}
		arrayBody := spec[arrayStart+1 : arrayEnd]
		var results []WriteSpecItem
		searchPos := 0
		for {
			pos := strings.Index(arrayBody[searchPos:], "{\"file\"")
			if pos < 0 {
				break
			}
			absStart := searchPos + pos
			elemEnd, ok := findMatching(arrayBody, absStart, '{', '}')
			if !ok {
				break
			}
			elem := arrayBody[absStart : elemEnd+1]
			fp := extractFileRaw(elem)
			ct := extractContentRaw(elem)
			results = append(results, WriteSpecItem{File: fp, Content: ct})
			searchPos = elemEnd + 1
		}
		if len(results) == 0 {
			return nil, invalidArg("parsed 0 valid items from files array")
		}
		return results, nil
	}

	fp := extractFileRaw(spec)
	ct, ok := extractContentRawMaybe(spec)
	if !ok {
		return nil, invalidArg("content field not found in write spec")
	}
	// degraded mode: scan for extract flag after content
	if strings.Contains(spec, `"extract":true`) || strings.Contains(spec, `"extract": true`) {
		ct = extractCodeBlocks(ct)
	}
	return []WriteSpecItem{{File: fp, Content: ct}}, nil
}

func extractContentRawMaybe(spec string) (string, bool) {
	key := "\"content\""
	idx := strings.Index(spec, key)
	if idx < 0 {
		return "", false
	}
	afterKey := spec[idx+len(key):]
	colon := strings.Index(afterKey, ":")
	if colon < 0 {
		return "", false
	}
	pos := idx + len(key) + colon + 1
	for pos < len(spec) && isSpace(spec[pos]) {
		pos++
	}
	if pos >= len(spec) || spec[pos] != '"' {
		return "", false
	}
	pos++
	var b strings.Builder
	for pos < len(spec) {
		switch spec[pos] {
		case '\\':
			if pos+1 >= len(spec) {
				b.WriteByte('\\')
				pos++
				continue
			}
			switch spec[pos+1] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte(spec[pos+1])
			}
			pos += 2
		case '"':
			return b.String(), true
		default:
			b.WriteByte(spec[pos])
			pos++
		}
	}
	return b.String(), true
}

func extractContentRaw(spec string) string {
	v, _ := extractContentRawMaybe(spec)
	return v
}

func extractFileRaw(spec string) string {
	key := "\"file\""
	idx := strings.Index(spec, key)
	if idx < 0 {
		return ""
	}
	afterKey := spec[idx+len(key):]
	colon := strings.Index(afterKey, ":")
	if colon < 0 {
		return ""
	}
	pos := idx + len(key) + colon + 1
	for pos < len(spec) && isSpace(spec[pos]) {
		pos++
	}
	if pos >= len(spec) || spec[pos] != '"' {
		return ""
	}
	pos++
	var b strings.Builder
	for pos < len(spec) {
		switch spec[pos] {
		case '\\':
			if pos+1 >= len(spec) {
				b.WriteByte('\\')
				pos++
				continue
			}
			b.WriteByte(spec[pos+1])
			pos += 2
		case '"':
			return b.String()
		default:
			b.WriteByte(spec[pos])
			pos++
		}
	}
	return b.String()
}

func parseContentBlocks(text string) string {
	return extractCodeBlocks(text)
}

func extractCodeBlocks(text string) string {
	var b strings.Builder
	inBlock := false
	capture := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inBlock {
				inBlock = false
				capture = false
			} else {
				inBlock = true
				capture = true
			}
			continue
		}
		if capture {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if b.Len() == 0 {
		return text
	}
	return strings.TrimRight(b.String(), "\n")
}

func findMatching(s string, start int, open, close byte) (int, bool) {
	depth := 0
	inStr := false
	for i := start; i < len(s); i++ {
		b := s[i]
		if b == '\\' && inStr {
			i++
			continue
		}
		if b == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if b == open {
			depth++
		} else if b == close {
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func countRustLines(content string) int {
	return rustLineCount(content)
}
