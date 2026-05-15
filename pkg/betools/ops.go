package betools

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func Show(path string, start int, endLine int, brief bool, opts ...Option) (ShowResult, string, error) {
	cfg := withCallConfig(opts...)
	if err := rejectBinary(path, cfg.fs); err != nil {
		return ShowResult{}, "", err
	}
	lines, _, err := readLines(path, opts...)
	if err != nil {
		return ShowResult{}, "", readPath(path, err)
	}
	total := len(lines)
	if total == 0 {
		return ShowResult{}, "", invalidArg("show: empty file")
	}
	if start < 1 || start > total {
		return ShowResult{}, "", invalidArg(fmt.Sprintf("show: start out of range (1..%d)", total))
	}

	s := start
	var e int
	if endLine <= 0 {
		// auto mode: detect function range or show context
		if rs, re, err := functionRangeRaw(path, s, opts...); err == nil {
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
		e = endLine
	}
	if e < s {
		return ShowResult{}, "", invalidArg(fmt.Sprintf("show: end must not be less than start (%d..%d)", s, total))
	}
	if e > total {
		e = total
	}

	var content string
	if !brief {
		var b strings.Builder
		for i := s - 1; i < e; i++ {
			fmt.Fprintf(&b, "%d\t%s", i+1, strings.TrimRight(lines[i], "\n\r"))
			if i < e-1 {
				b.WriteByte('\n')
			}
		}
		content = b.String()
	}

	sessionID := CreateSession(path, s, e, opts...)
	var warnings []string
	if !brief {
		warnings = scanContentWarnings(content)
	}
	return ShowResult{
		Status:   "ok",
		File:     filepath.Clean(path),
		Start:    s,
		End:      e,
		Total:    total,
		Content:  content,
		Brief:    brief,
		Warnings: warnings,
	}, sessionID, nil
}

func Read(path string, start int, endLine int, brief bool, opts ...Option) (ShowResult, string, error) {
	return Show(path, start, endLine, brief, opts...)
}

func Replace(path string, start, end int, old *string, content string, raw bool, format string, preview bool, sessionID string, brief bool, opts ...Option) (ReplaceResult, error) {
	lines, le, err := readLines(path, opts...)
	if err != nil {
		return ReplaceResult{}, readPath(path, err)
	}
	total := len(lines)
	if start < 1 || start > total {
		return ReplaceResult{}, invalidArg(fmt.Sprintf("replace: start out of range (1..%d)", total))
	}
	if end < start || end > total {
		return ReplaceResult{}, invalidArg(fmt.Sprintf("replace: end out of range (%d..%d)", start, total))
	}

	// Validate session if provided.
	var sessionWarning string
	if sessionID != "" {
		if _, warn := SessionFromCache(sessionID, opts...); warn != "" {
			sessionWarning = warn
		}
	}
	if old != nil {
		normalizedOld := normalizeLineBreaks(*old)
		current := normalizeLineBlock(strings.Join(lines[start-1:end], ""))
		expected := normalizeLineBlock(normalizedOld)
		if current != expected {
			return ReplaceResult{}, invalidArg(fmt.Sprintf("replace: old content does not match (line %d-%d)", start, end))
		}
	}
	beforeStart := max(1, start-5)
	beforeEnd := min(total, end+5)
	beforeContent := append([]string(nil), lines[beforeStart-1:beforeEnd]...)
	newLines := prepareContentLines(content, le, raw)
	out := make([]string, 0, len(lines)-((end-start)+1)+len(newLines))
	out = append(out, lines[:start-1]...)
	out = append(out, newLines...)
	out = append(out, lines[end:]...)
	newContent := strings.Join(out, "")
	warnings := scanContentWarnings(newContent)
	if !preview {
		if err := writeFileAtomic(path, newContent, opts...); err != nil {
			return ReplaceResult{}, writePath(path, err)
		}
	}
	delta := len(newLines) - (end - start + 1)
	afterTotal := len(out)
	afterEnd := min(afterTotal, beforeEnd+delta)
	afterContent := append([]string(nil), out[beforeStart-1:afterEnd]...)
	var diff string
	var balance string
	if !brief {
		diff = buildDiff(beforeContent, afterContent, beforeStart, format)
		balance = quickBalanceCheck(strings.Join(out, ""))
	}
	res := ReplaceResult{
		Status:   "ok",
		File:     filepath.Clean(path),
		Removed:  end - start + 1,
		Added:    len(newLines),
		Total:    afterTotal,
		Diff:     diff,
		Balance:  balance,
		Affected: fmt.Sprintf("行 %d-%d（当前共 %d 行）", beforeStart, afterEnd, afterTotal),
		Preview:  preview,
		Warning:  sessionWarning,
		Brief:    brief,
		Warnings: warnings,
	}
	if !preview {
		id, queueFull := PushSnapshot(SnapshotRecord{
			Tool:  "be-replace",
			File:  filepath.Clean(path),
			Before: SnapshotRange{
				Start: beforeStart,
				End:   beforeEnd,
				Lines: beforeContent,
			},
			After: SnapshotRange{
				Start: beforeStart,
				End:   afterEnd,
				Lines: afterContent,
			},
			Args: map[string]any{
				"file":  path,
				"start": start,
				"end":   end,
			},
			Summary: fmt.Sprintf("be-replace on %s lines %d-%d", filepath.Base(path), start, end),
		})
		res.EventID = id
		res.QueueFull = queueFull
	}
	return res, nil
}

func normalizeLineBlock(content string) string {
	return strings.TrimRight(strings.ReplaceAll(content, "\r\n", "\n"), "\r\n")
}

func Insert(path string, after int, content string, raw bool, format string, preview bool, brief bool, opts ...Option) (InsertResult, error) {
	lines, le, err := readLines(path, opts...)
	if err != nil {
		return InsertResult{}, readPath(path, err)
	}
	total := len(lines)
	if after > total {
		return InsertResult{}, invalidArg(fmt.Sprintf("insert: line (%d) out of range (0..%d)", after, total))
	}
	beforeStart := max(1, after-5)
	beforeEnd := min(total, after+5)
	beforeContent := append([]string(nil), lines[beforeStart-1:beforeEnd]...)
	newLines := prepareContentLines(content, le, raw)
	result := make([]string, 0, len(lines)+len(newLines))
	result = append(result, lines[:after]...)
	result = append(result, newLines...)
	result = append(result, lines[after:]...)
	newContent := strings.Join(result, "")
	warnings := scanContentWarnings(newContent)
	if !preview {
		if err := writeFileAtomic(path, newContent, opts...); err != nil {
			return InsertResult{}, writePath(path, err)
		}
	}
	afterTotal := len(result)
	afterEnd := min(afterTotal, beforeEnd+len(newLines))
	afterContent := append([]string(nil), result[beforeStart-1:afterEnd]...)
	var diff string
	var balance string
	if !brief {
		diff = buildDiff(beforeContent, afterContent, beforeStart, format)
		balance = quickBalanceCheck(strings.Join(result, ""))
	}
	res := InsertResult{
		Status:   "ok",
		File:     filepath.Clean(path),
		After:    after,
		Added:    len(newLines),
		Total:    afterTotal,
		Diff:     diff,
		Balance:  balance,
		Affected: fmt.Sprintf("行 %d-%d（当前共 %d 行）", beforeStart, afterEnd, afterTotal),
		Preview:  preview,
		Brief:    brief,
		Warnings: warnings,
	}
	if !preview {
		id, queueFull := PushSnapshot(SnapshotRecord{
			Tool:  "be-insert",
			File:  filepath.Clean(path),
			Before: SnapshotRange{
				Start: beforeStart,
				End:   beforeEnd,
				Lines: beforeContent,
			},
			After: SnapshotRange{
				Start: beforeStart,
				End:   afterEnd,
				Lines: afterContent,
			},
			Args: map[string]any{
				"file":  path,
				"after": after,
			},
			Summary: fmt.Sprintf("be-insert on %s after line %d", filepath.Base(path), after),
		})
		res.EventID = id
		res.QueueFull = queueFull
	}
	return res, nil
}

func Delete(path string, start, end, line int, linesJSON *string, format string, preview bool, brief bool, opts ...Option) (DeleteResult, error) {
	fileLines, _, err := readLines(path, opts...)
	if err != nil {
		return DeleteResult{}, readPath(path, err)
	}
	total := len(fileLines)
	if linesJSON != nil {
		var nums []int
		if err := jsonUnmarshal(*linesJSON, &nums); err != nil {
			return DeleteResult{}, jsonParse(err)
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
			return DeleteResult{}, invalidArg(fmt.Sprintf("delete: all line numbers out of range (1..%d)", total))
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
	warnings := scanContentWarnings(newContent)
	if !preview {
		if err := writeFileAtomic(path, newContent, opts...); err != nil {
			return DeleteResult{}, writePath(path, err)
		}
	}
	afterTotal := len(filtered)
	afterEnd := min(afterTotal, max(0, beforeEnd-len(valid)))
	afterContent := append([]string(nil), filtered[beforeStart-1:afterEnd]...)
	var diff string
	var balance string
	if !brief {
		diff = buildDiff(beforeContent, afterContent, beforeStart, format)
		balance = quickBalanceCheck(newContent)
	}
	res := DeleteResult{
		Status:   "ok",
		File:     filepath.Clean(path),
		Total:    afterTotal,
		Diff:     diff,
		Balance:  balance,
		Affected: fmt.Sprintf("行 %d-%d（当前共 %d 行）", beforeStart, afterEnd, afterTotal),
		Preview:  preview,
		Brief:    brief,
		Warnings: warnings,
	}
	if !preview {
		id, queueFull := PushSnapshot(SnapshotRecord{
			Tool:  "be-delete",
			File:  filepath.Clean(path),
			Before: SnapshotRange{
				Start: beforeStart,
				End:   beforeEnd,
				Lines: beforeContent,
			},
			After: SnapshotRange{
				Start: beforeStart,
				End:   afterEnd,
				Lines: afterContent,
			},
			Args: map[string]any{
				"file":  path,
				"lines": len(valid),
			},
			Summary: fmt.Sprintf("be-delete on %s (%d lines)", filepath.Base(path), len(valid)),
		})
		res.EventID = id
		res.QueueFull = queueFull
	}
	return res, nil
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
		return DeleteResult{}, invalidArg(fmt.Sprintf("delete: start (%d) out of range (1..%d)", s, total))
	}
	if e < s || e > total {
		return DeleteResult{}, invalidArg(fmt.Sprintf("delete: end (%d) out of range (%d..%d)", e, s, total))
	}
	// Validate brace balance in the target range to prevent corrupting adjacent code
	if err := bracesInRange(fileLines, s, e); err != nil {
		return DeleteResult{}, err
	}
	beforeStart := max(1, s-5)
	beforeEnd := min(total, e+5)
	beforeContent := append([]string(nil), fileLines[beforeStart-1:beforeEnd]...)
	deleted := e - s + 1
	result := append([]string(nil), fileLines[:s-1]...)
	result = append(result, fileLines[e:]...)
	newContent := strings.Join(result, "")
	warnings := scanContentWarnings(newContent)
	if !preview {
		// Save deleted content as a chip for potential recovery
		deletedContent := strings.Join(fileLines[s-1:e], "")
		if deletedContent != "" {
			chipID, chipWarn := SaveContentChip("be-delete", deletedContent)
			warnings = append(warnings, fmt.Sprintf("deleted content saved as chip://%s", chipID))
			if chipWarn != "" {
				warnings = append(warnings, chipWarn)
			}
		}
		if err := writeFileAtomic(path, newContent, opts...); err != nil {
			return DeleteResult{}, writePath(path, err)
		}
	}
	afterTotal := len(result)
	afterEnd := min(afterTotal, max(0, beforeEnd-deleted))
	afterContent := append([]string(nil), result[beforeStart-1:afterEnd]...)
	var diff string
	var balance string
	if !brief {
		diff = buildDiff(beforeContent, afterContent, beforeStart, format)
		balance = quickBalanceCheck(strings.Join(result, ""))
	}
	res := DeleteResult{
		Status:   "ok",
		File:     filepath.Clean(path),
		Total:    afterTotal,
		Diff:     diff,
		Balance:  balance,
		Affected: fmt.Sprintf("行 %d-%d（当前共 %d 行）", beforeStart, afterEnd, afterTotal),
		Preview:  preview,
		Brief:    brief,
		Warnings: warnings,
	}
	if !preview {
		id, queueFull := PushSnapshot(SnapshotRecord{
			Tool:  "be-delete",
			File:  filepath.Clean(path),
			Before: SnapshotRange{
				Start: beforeStart,
				End:   beforeEnd,
				Lines: beforeContent,
			},
			After: SnapshotRange{
				Start: beforeStart,
				End:   afterEnd,
				Lines: afterContent,
			},
			Args: map[string]any{
				"file":  path,
				"start": s,
				"end":   e,
			},
			Summary: fmt.Sprintf("be-delete on %s lines %d-%d", filepath.Base(path), s, e),
		})
		res.EventID = id
		res.QueueFull = queueFull
	}
	return res, nil
}

func Batch(spec string, preview bool, brief bool, opts ...Option) (BatchResult, error) {
	var raw any
	if err := jsonUnmarshal(spec, &raw); err != nil {
		return BatchResult{}, jsonParse(err)
	}
	fileSpecs, err := parseBatchSpec(raw)
	if err != nil {
		return BatchResult{}, err
	}
	var lastEventID string
	var anyQueueFull string
	results := make([]BatchFileResult, 0, len(fileSpecs))
	for _, fileSpec := range fileSpecs {
		lines, le, err := readLines(fileSpec.File, opts...)
		if err != nil {
			return BatchResult{}, readPath(fileSpec.File, err)
		}
		initialLines := make([]string, len(lines))
		copy(initialLines, lines)
		totalBefore := len(lines)

		edits := fileSpec.Edits
		sortEditsDesc(edits)

		// Compute affected range for snapshot window
		var editMinStart, editMaxEnd int
		for _, edit := range edits {
			switch edit.Action {
			case "replace-lines", "delete-lines":
				if s, ok := asInt(edit.Start); ok {
					if editMinStart == 0 || s < editMinStart {
						editMinStart = s
					}
				}
				if e, ok := asInt(edit.End); ok {
					if editMaxEnd == 0 || e > editMaxEnd {
						editMaxEnd = e
					}
				}
			case "insert-after":
				if ln, ok := asInt(edit.Line); ok {
					ins := ln + 1
					if editMinStart == 0 || ins < editMinStart {
						editMinStart = ins
					}
					if editMaxEnd == 0 || ins > editMaxEnd {
						editMaxEnd = ins
					}
				}
			}
		}
		if editMinStart == 0 {
			editMinStart = 1
		}
		if editMaxEnd == 0 || editMaxEnd < editMinStart {
			editMaxEnd = editMinStart
		}

		bStart := max(1, editMinStart-5)
		bEnd := min(totalBefore, editMaxEnd+5)
		beforeContent := append([]string(nil), initialLines[bStart-1:bEnd]...)

		for _, edit := range edits {
			switch edit.Action {
			case "replace-lines":
				s, ok1 := asInt(edit.Start)
				e, ok2 := asInt(edit.End)
				if !ok1 {
					return BatchResult{}, invalidArg("replace-lines: missing start")
				}
				if !ok2 {
					return BatchResult{}, invalidArg("replace-lines: missing end")
				}
				if s < 1 || s > len(lines) {
					return BatchResult{}, invalidArg(fmt.Sprintf("batch/replace: start (%d) out of range (1..%d)", s, len(lines)))
				}
				if e < s || e > len(lines) {
					return BatchResult{}, invalidArg(fmt.Sprintf("batch/replace: end (%d) out of range (%d..%d)", e, s, len(lines)))
				}
				newLines := prepareContentLines(edit.Content, le, true)
				tmp := append([]string(nil), lines[:s-1]...)
				tmp = append(tmp, newLines...)
				tmp = append(tmp, lines[e:]...)
				lines = tmp
			case "insert-after":
				ln, ok := asInt(edit.Line)
				if !ok {
					return BatchResult{}, invalidArg("insert-after: missing line")
				}
				if ln > len(lines) {
					return BatchResult{}, invalidArg(fmt.Sprintf("batch/insert: line (%d) out of range (0..%d)", ln, len(lines)))
				}
				newLines := prepareContentLines(edit.Content, le, true)
				tmp := append([]string(nil), lines[:ln]...)
				tmp = append(tmp, newLines...)
				tmp = append(tmp, lines[ln:]...)
				lines = tmp
			case "delete-lines":
				s, ok1 := asInt(edit.Start)
				e, ok2 := asInt(edit.End)
				if !ok1 {
					return BatchResult{}, invalidArg("delete-lines: missing start")
				}
				if !ok2 {
					return BatchResult{}, invalidArg("delete-lines: missing end")
				}
				if s < 1 || s > len(lines) {
					return BatchResult{}, invalidArg(fmt.Sprintf("batch/delete: start (%d) out of range (1..%d)", s, len(lines)))
				}
				if e < s || e > len(lines) {
					return BatchResult{}, invalidArg(fmt.Sprintf("batch/delete: end (%d) out of range (%d..%d)", e, s, len(lines)))
				}
				tmp := append([]string(nil), lines[:s-1]...)
				tmp = append(tmp, lines[e:]...)
				lines = tmp
			default:
				return BatchResult{}, invalidArg(fmt.Sprintf("batch: unknown action %q, supported: replace-lines, insert-after, delete-lines", edit.Action))
			}
		}
		newContent := strings.Join(lines, "")
		warnings := scanContentWarnings(newContent)
		if !preview {
			if err := writeFileAtomic(fileSpec.File, newContent, opts...); err != nil {
				return BatchResult{}, writePath(fileSpec.File, err)
			}
		}
		if !preview {
			afterTotal := len(lines)
			deltaLines := afterTotal - totalBefore
			aStart := bStart
			aEnd := min(afterTotal, bEnd+deltaLines)
			afterContent := append([]string(nil), lines[aStart-1:aEnd]...)
			id, queueFull := PushSnapshot(SnapshotRecord{
				Tool: "be-batch",
				File: filepath.Clean(fileSpec.File),
				Before: SnapshotRange{
					Start: bStart,
					End:   bEnd,
					Lines: beforeContent,
				},
				After: SnapshotRange{
					Start: aStart,
					End:   aEnd,
					Lines: afterContent,
				},
				Args: map[string]any{
					"file":  fileSpec.File,
					"edits": len(edits),
				},
				Summary: fmt.Sprintf("be-batch on %s (%d edits)", filepath.Base(fileSpec.File), len(edits)),
			})
			lastEventID = id
			if queueFull != "" {
				anyQueueFull = queueFull
			}
		}
		results = append(results, BatchFileResult{File: fileSpec.File, Edits: len(edits), Total: len(lines), Warnings: warnings})
	}
	res := BatchResult{Status: "ok", Files: len(results), Results: results, Preview: preview, Brief: brief}
	if lastEventID != "" {
		res.EventID = lastEventID
		res.QueueFull = anyQueueFull
	}
	return res, nil
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
		return nil, invalidArg("batch: unsupported JSON format, expected array or object")
	}
}

func parseBatchFileSpec(raw any) (BatchFileSpec, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return BatchFileSpec{}, invalidArg("batch: missing \"file\" field")
	}
	file, _ := m["file"].(string)
	if file == "" {
		return BatchFileSpec{}, invalidArg("batch: missing \"file\" field")
	}
	rawEdits, ok := m["edits"].([]any)
	if !ok {
		return BatchFileSpec{}, invalidArg(fmt.Sprintf("batch: missing \"edits\" array field (file: %s)", file))
	}
	if len(rawEdits) == 0 {
		return BatchFileSpec{}, invalidArg("batch: edits 数组为空")
	}
	edits := make([]BatchEditSpec, 0, len(rawEdits))
	for _, item := range rawEdits {
		em, ok := item.(map[string]any)
		if !ok {
			return BatchFileSpec{}, invalidArg("batch: missing action field")
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

// bracesInRange checks whether the braces within lines start..end are balanced.
// This is used as a safety check on target-resolved ranges to prevent
// off-by-one errors from corrupting adjacent code.
func bracesInRange(lines []string, start, end int) error {
	if start < 1 || start > len(lines) || end < start || end > len(lines) {
		return nil
	}
	var depth int
	for i := start - 1; i < end; i++ {
		for _, ch := range lines[i] {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
			}
		}
	}
	if depth != 0 {
		return invalidArg(fmt.Sprintf("target range lines %d-%d has unbalanced braces (diff=%d). The detected range may include code outside the intended target. Use explicit line numbers instead", start, end, depth))
	}
	return nil
}
