package edit

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func Show(path string, start int, end *ShowEnd) (ShowResult, error) {
	lines, _, err := ReadLines(path)
	if err != nil {
		return ShowResult{}, ReadPath(path, err)
	}
	total := len(lines)
	if total == 0 {
		return ShowResult{}, InvalidArg("show: 文件为空")
	}
	if start < 1 || start > total {
		return ShowResult{}, InvalidArg(fmt.Sprintf("show: start 超出范围 (1..%d)", total))
	}

	s := start
	var e int
	if end == nil || end.Auto {
		if rs, re, err := FunctionRangeRaw(path, s); err == nil {
			s = rs
			e = re
		} else {
			ctxBefore := 5
			ctxAfter := 5
			minLines := 20
			ctxStart := max(1, s-ctxBefore)
			ctxEnd := min(total, s+ctxAfter)
			if ctxEnd-ctxStart+1 < minLines {
				extra := (minLines - (ctxEnd - ctxStart + 1) + 1) / 2
				ctxEnd = min(total, ctxEnd+extra)
			}
			s = ctxStart
			e = ctxEnd
		}
	} else {
		e = end.Line
	}
	if e < s {
		return ShowResult{}, InvalidArg(fmt.Sprintf("show: end 不能小于 start (%d..%d)", s, total))
	}
	if e > total {
		e = total
	}

	var b strings.Builder
	for i := s - 1; i < e; i++ {
		fmt.Fprintf(&b, "%d\t%s", i+1, strings.TrimRight(lines[i], "\n\r"))
		if i < e-1 {
			b.WriteByte('\n')
		}
	}
	return ShowResult{
		Status:  "ok",
		File:    filepath.Clean(path),
		Start:   s,
		End:     e,
		Total:   total,
		Content: b.String(),
	}, nil
}

func Replace(path string, start, end int, old *string, content string, raw bool, format string, preview bool) (ReplaceResult, error) {
	lines, le, err := ReadLines(path)
	if err != nil {
		return ReplaceResult{}, ReadPath(path, err)
	}
	total := len(lines)
	if start < 1 || start > total {
		return ReplaceResult{}, InvalidArg(fmt.Sprintf("replace: start 超出范围 (1..%d)", total))
	}
	if end < start || end > total {
		return ReplaceResult{}, InvalidArg(fmt.Sprintf("replace: end 超出范围 (%d..%d)", start, total))
	}
	if old != nil {
		current := normalizeLineBlock(strings.Join(lines[start-1:end], ""))
		expected := normalizeLineBlock(*old)
		if current != expected {
			return ReplaceResult{}, InvalidArg(fmt.Sprintf("replace: old 内容不匹配 (line %d-%d)", start, end))
		}
	}
	beforeStart := max(1, start-5)
	beforeEnd := min(total, end+5)
	beforeContent := append([]string(nil), lines[beforeStart-1:beforeEnd]...)
	newLines := PrepareContentLines(content, le, raw)
	out := make([]string, 0, len(lines)-((end-start)+1)+len(newLines))
	out = append(out, lines[:start-1]...)
	out = append(out, newLines...)
	out = append(out, lines[end:]...)
	newContent := strings.Join(out, "")
	if !preview {
		if err := WriteFileAtomic(path, newContent); err != nil {
			return ReplaceResult{}, WritePath(path, err)
		}
	}
	delta := len(newLines) - (end - start + 1)
	afterTotal := len(out)
	afterEnd := min(afterTotal, beforeEnd+delta)
	afterContent := append([]string(nil), out[beforeStart-1:afterEnd]...)
	diff := BuildDiff(beforeContent, afterContent, beforeStart, format)
	balance := QuickBalanceCheck(strings.Join(out, ""))
	return ReplaceResult{
		Status:   "ok",
		File:     filepath.Clean(path),
		Removed:  end - start + 1,
		Added:    len(newLines),
		Total:    afterTotal,
		Diff:     diff,
		Balance:  balance,
		Affected: fmt.Sprintf("行 %d-%d（当前共 %d 行）", beforeStart, afterEnd, afterTotal),
		Preview:  boolPtr(preview),
	}, nil
}

func normalizeLineBlock(content string) string {
	return strings.TrimRight(strings.ReplaceAll(content, "\r\n", "\n"), "\r\n")
}

func Insert(path string, after int, content string, raw bool, format string, preview bool) (InsertResult, error) {
	lines, le, err := ReadLines(path)
	if err != nil {
		return InsertResult{}, ReadPath(path, err)
	}
	total := len(lines)
	if after > total {
		return InsertResult{}, InvalidArg(fmt.Sprintf("insert: line (%d) 超出范围 (0..%d)", after, total))
	}
	beforeStart := max(1, after-5+1)
	beforeEnd := min(total, after+5)
	beforeContent := append([]string(nil), lines[beforeStart-1:beforeEnd]...)
	newLines := PrepareContentLines(content, le, raw)
	result := make([]string, 0, len(lines)+len(newLines))
	result = append(result, lines[:after]...)
	result = append(result, newLines...)
	result = append(result, lines[after:]...)
	newContent := strings.Join(result, "")
	if !preview {
		if err := WriteFileAtomic(path, newContent); err != nil {
			return InsertResult{}, WritePath(path, err)
		}
	}
	afterTotal := len(result)
	afterEnd := min(afterTotal, beforeEnd+len(newLines))
	afterContent := append([]string(nil), result[beforeStart-1:afterEnd]...)
	diff := BuildDiff(beforeContent, afterContent, beforeStart, format)
	balance := QuickBalanceCheck(strings.Join(result, ""))
	return InsertResult{
		Status:   "ok",
		File:     filepath.Clean(path),
		After:    after,
		Added:    len(newLines),
		Total:    afterTotal,
		Diff:     diff,
		Balance:  balance,
		Affected: fmt.Sprintf("行 %d-%d（当前共 %d 行）", beforeStart, afterEnd, afterTotal),
		Preview:  boolPtr(preview),
	}, nil
}

func Delete(path string, start, end, line int, linesJSON *string, format string, preview bool) (DeleteResult, error) {
	fileLines, _, err := ReadLines(path)
	if err != nil {
		return DeleteResult{}, ReadPath(path, err)
	}
	total := len(fileLines)
	if linesJSON != nil {
		var nums []int
		if err := jsonUnmarshal(*linesJSON, &nums); err != nil {
			return DeleteResult{}, JsonParse(err)
		}
		valid := make([]int, 0, len(nums))
		seen := make(map[int]struct{}, len(nums))
		for _, n := range nums {
			if n >= 1 && n <= total {
				if _, ok := seen[n]; !ok {
					seen[n] = struct{}{}
					valid = append(valid, n)
				}
			}
		}
		if len(valid) == 0 {
			return DeleteResult{}, InvalidArg(fmt.Sprintf("delete: 所有行号均超出文件范围 (1..%d)", total))
		}
		minDel, maxDel := valid[0], valid[0]
		for _, n := range valid[1:] {
			if n < minDel {
				minDel = n
			}
			if n > maxDel {
				maxDel = n
			}
		}
		beforeStart := max(1, minDel-5)
		beforeEnd := min(total, maxDel+5)
		beforeContent := append([]string(nil), fileLines[beforeStart-1:beforeEnd]...)
		toDelete := make(map[int]struct{}, len(valid))
		for _, n := range valid {
			toDelete[n] = struct{}{}
		}
		filtered := make([]string, 0, len(fileLines)-len(valid))
		for idx, line := range fileLines {
			if _, ok := toDelete[idx+1]; ok {
				continue
			}
			filtered = append(filtered, line)
		}
		newContent := strings.Join(filtered, "")
		if !preview {
			if err := WriteFileAtomic(path, newContent); err != nil {
				return DeleteResult{}, WritePath(path, err)
			}
		}
		afterTotal := len(filtered)
		afterEnd := min(afterTotal, max(0, beforeEnd-len(valid)))
		afterContent := append([]string(nil), filtered[beforeStart-1:afterEnd]...)
		diff := BuildDiff(beforeContent, afterContent, beforeStart, format)
		balance := QuickBalanceCheck(newContent)
		return DeleteResult{
			Status:   "ok",
			File:     filepath.Clean(path),
			Total:    afterTotal,
			Diff:     diff,
			Balance:  balance,
			Affected: fmt.Sprintf("行 %d-%d（当前共 %d 行）", beforeStart, afterEnd, afterTotal),
			Preview:  boolPtr(preview),
		}, nil
	}

	s := start
	if s == 0 {
		s = line
	}
	if s == 0 {
		s = 1
	}
	e := end
	if e == 0 {
		e = line
	}
	if e == 0 {
		e = s
	}
	if s < 1 || s > total {
		return DeleteResult{}, InvalidArg(fmt.Sprintf("delete: start (%d) 超出范围 (1..%d)", s, total))
	}
	if e < s || e > total {
		return DeleteResult{}, InvalidArg(fmt.Sprintf("delete: end (%d) 超出范围 (%d..%d)", e, s, total))
	}
	beforeStart := max(1, s-5)
	beforeEnd := min(total, e+5)
	beforeContent := append([]string(nil), fileLines[beforeStart-1:beforeEnd]...)
	deleted := e - s + 1
	result := append([]string(nil), fileLines[:s-1]...)
	result = append(result, fileLines[e:]...)
	newContent := strings.Join(result, "")
	if !preview {
		if err := WriteFileAtomic(path, newContent); err != nil {
			return DeleteResult{}, WritePath(path, err)
		}
	}
	afterTotal := len(result)
	afterEnd := min(afterTotal, max(0, beforeEnd-deleted))
	afterContent := append([]string(nil), result[beforeStart-1:afterEnd]...)
	diff := BuildDiff(beforeContent, afterContent, beforeStart, format)
	balance := QuickBalanceCheck(strings.Join(result, ""))
	return DeleteResult{
		Status:   "ok",
		File:     filepath.Clean(path),
		Total:    afterTotal,
		Diff:     diff,
		Balance:  balance,
		Affected: fmt.Sprintf("行 %d-%d（当前共 %d 行）", beforeStart, afterEnd, afterTotal),
		Preview:  boolPtr(preview),
	}, nil
}

func Batch(spec string, preview bool) (BatchResult, error) {
	var raw any
	if err := jsonUnmarshal(spec, &raw); err != nil {
		return BatchResult{}, JsonParse(err)
	}
	fileSpecs, err := parseBatchSpec(raw)
	if err != nil {
		return BatchResult{}, err
	}
	results := make([]BatchFileResult, 0, len(fileSpecs))
	for _, fileSpec := range fileSpecs {
		lines, le, err := ReadLines(fileSpec.File)
		if err != nil {
			return BatchResult{}, ReadPath(fileSpec.File, err)
		}
		edits := fileSpec.Edits
		sortEditsDesc(edits)
		for _, edit := range edits {
			switch edit.Action {
			case "replace-lines":
				s, ok1 := asInt(edit.Start)
				e, ok2 := asInt(edit.End)
				if !ok1 {
					return BatchResult{}, InvalidArg("replace-lines: 缺少 start")
				}
				if !ok2 {
					return BatchResult{}, InvalidArg("replace-lines: 缺少 end")
				}
				if s < 1 || s > len(lines) {
					return BatchResult{}, InvalidArg(fmt.Sprintf("batch/replace: start (%d) 超出范围 (1..%d)", s, len(lines)))
				}
				if e < s || e > len(lines) {
					return BatchResult{}, InvalidArg(fmt.Sprintf("batch/replace: end (%d) 超出范围 (%d..%d)", e, s, len(lines)))
				}
				newLines := PrepareContentLines(edit.Content, le, true)
				tmp := append([]string(nil), lines[:s-1]...)
				tmp = append(tmp, newLines...)
				tmp = append(tmp, lines[e:]...)
				lines = tmp
			case "insert-after":
				ln, ok := asInt(edit.Line)
				if !ok {
					return BatchResult{}, InvalidArg("insert-after: 缺少 line")
				}
				if ln > len(lines) {
					return BatchResult{}, InvalidArg(fmt.Sprintf("batch/insert: line (%d) 超出范围 (0..%d)", ln, len(lines)))
				}
				newLines := PrepareContentLines(edit.Content, le, true)
				tmp := append([]string(nil), lines[:ln]...)
				tmp = append(tmp, newLines...)
				tmp = append(tmp, lines[ln:]...)
				lines = tmp
			case "delete-lines":
				s, ok1 := asInt(edit.Start)
				e, ok2 := asInt(edit.End)
				if !ok1 {
					return BatchResult{}, InvalidArg("delete-lines: 缺少 start")
				}
				if !ok2 {
					return BatchResult{}, InvalidArg("delete-lines: 缺少 end")
				}
				if s < 1 || s > len(lines) {
					return BatchResult{}, InvalidArg(fmt.Sprintf("batch/delete: start (%d) 超出范围 (1..%d)", s, len(lines)))
				}
				if e < s || e > len(lines) {
					return BatchResult{}, InvalidArg(fmt.Sprintf("batch/delete: end (%d) 超出范围 (%d..%d)", e, s, len(lines)))
				}
				tmp := append([]string(nil), lines[:s-1]...)
				tmp = append(tmp, lines[e:]...)
				lines = tmp
			default:
				return BatchResult{}, InvalidArg(fmt.Sprintf("batch: 未知操作 %q，支持: replace-lines, insert-after, delete-lines", edit.Action))
			}
		}
		newContent := strings.Join(lines, "")
		if !preview {
			if err := WriteFileAtomic(fileSpec.File, newContent); err != nil {
				return BatchResult{}, WritePath(fileSpec.File, err)
			}
		}
		results = append(results, BatchFileResult{File: fileSpec.File, Edits: len(edits), Total: len(lines)})
	}
	return BatchResult{Status: "ok", Files: len(results), Results: results, Preview: boolPtr(preview)}, nil
}

func parseBatchSpec(raw any) ([]BatchFileSpec, error) {
	switch v := raw.(type) {
	case []any:
		out := make([]BatchFileSpec, 0, len(v))
		for _, item := range v {
			spec, err := parseBatchFileSpec(item)
			if err != nil {
				return nil, err
			}
			out = append(out, spec)
		}
		return out, nil
	case map[string]any:
		if files, ok := v["files"].([]any); ok {
			out := make([]BatchFileSpec, 0, len(files))
			for _, item := range files {
				spec, err := parseBatchFileSpec(item)
				if err != nil {
					return nil, err
				}
				out = append(out, spec)
			}
			return out, nil
		}
		spec, err := parseBatchFileSpec(v)
		if err != nil {
			return nil, err
		}
		return []BatchFileSpec{spec}, nil
	default:
		return nil, InvalidArg("batch: 不支持的 JSON 格式，需要数组或对象")
	}
}

func parseBatchFileSpec(raw any) (BatchFileSpec, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return BatchFileSpec{}, InvalidArg("batch: 缺少 \"file\" 字段")
	}
	file, _ := m["file"].(string)
	if file == "" {
		return BatchFileSpec{}, InvalidArg("batch: 缺少 \"file\" 字段")
	}
	rawEdits, ok := m["edits"].([]any)
	if !ok {
		return BatchFileSpec{}, InvalidArg(fmt.Sprintf("batch: 缺少 \"edits\" 数组字段 (file: %s)", file))
	}
	if len(rawEdits) == 0 {
		return BatchFileSpec{}, InvalidArg("batch: edits 数组为空")
	}
	edits := make([]BatchEditSpec, 0, len(rawEdits))
	for _, item := range rawEdits {
		em, ok := item.(map[string]any)
		if !ok {
			return BatchFileSpec{}, InvalidArg("batch: 缺少 action 字段")
		}
		edits = append(edits, BatchEditSpec{
			Action:  asString(em["action"]),
			Start:   em["start"],
			End:     em["end"],
			Line:    em["line"],
			Content: asString(em["content"]),
		})
	}
	return BatchFileSpec{File: file, Edits: edits}, nil
}

func sortEditsDesc(edits []BatchEditSpec) {
	sort.SliceStable(edits, func(i, j int) bool {
		return editKey(edits[i]) > editKey(edits[j])
	})
}

func editKey(edit BatchEditSpec) int {
	if n, ok := asInt(edit.Start); ok {
		return n
	}
	if n, ok := asInt(edit.Line); ok {
		return n
	}
	return 0
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case int32:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func jsonUnmarshal(data string, v any) error {
	dec := json.NewDecoder(strings.NewReader(data))
	dec.UseNumber()
	return dec.Decode(v)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
