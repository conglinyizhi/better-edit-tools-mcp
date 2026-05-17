package betools

import (
	"fmt"
	"path/filepath"
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
			Tool: "be-replace",
			File: filepath.Clean(path),
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

// Insert adds content after the specified line number (0 = beginning of file).
// The `after` parameter is the only position specifier — there is no implicit
// line-1 conversion (unlike the legacy `line` parameter which was removed to
// eliminate tool confusion; models could not reliably predict the -1 offset).
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
			Tool: "be-insert",
			File: filepath.Clean(path),
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

// Delete removes lines start..end (inclusive) from the file.
//
// Only start/end range deletion is supported. Historic aliases (line, lines,
// start_line, end_line) were removed because offering 5 ways to express the same
// action created high tool confusion for LLM callers — models consistently
// struggled to pick the correct combination of parameters.
func Delete(path string, start, end int, format string, preview bool, brief bool, opts ...Option) (DeleteResult, error) {
	fileLines, _, err := readLines(path, opts...)
	if err != nil {
		return DeleteResult{}, readPath(path, err)
	}
	total := len(fileLines)

	if start < 1 || start > total {
		return DeleteResult{}, invalidArg(fmt.Sprintf("delete: start (%d) out of range (1..%d)", start, total))
	}
	if end < start || end > total {
		return DeleteResult{}, invalidArg(fmt.Sprintf("delete: end (%d) out of range (%d..%d)", end, start, total))
	}

	// Validate brace balance in the target range to prevent corrupting adjacent code
	if err := bracesInRange(fileLines, start, end); err != nil {
		return DeleteResult{}, err
	}

	beforeStart := max(1, start-5)
	beforeEnd := min(total, end+5)
	beforeContent := append([]string(nil), fileLines[beforeStart-1:beforeEnd]...)
	deleted := end - start + 1
	result := append([]string(nil), fileLines[:start-1]...)
	result = append(result, fileLines[end:]...)
	newContent := strings.Join(result, "")
	warnings := scanContentWarnings(newContent)
	if !preview {
		// Save deleted content as a chip for potential recovery
		deletedContent := strings.Join(fileLines[start-1:end], "")
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
			Tool: "be-delete",
			File: filepath.Clean(path),
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
			Summary: fmt.Sprintf("be-delete on %s lines %d-%d", filepath.Base(path), start, end),
		})
		res.EventID = id
		res.QueueFull = queueFull
	}
	return res, nil
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
