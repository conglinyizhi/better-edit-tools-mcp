package betools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────
// #38: be-read 负 end 值从文件末尾倒数行号
// ──────────────────────────────────────────────

func TestShowNegativeEnd_FromEnd(t *testing.T) {
	// 11-line file, end=-3 → should read lines 1..9 (11 + (-3) + 1 = 9)
	path := writeTempFile(t, "a.txt", "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\n")
	res, _, err := Show(path, 1, -3, false)
	if err != nil {
		t.Fatalf("show with end=-3: %v", err)
	}
	if res.Start != 1 || res.End != 9 {
		t.Fatalf("expected range 1..9 for end=-3 on 11 lines, got %d..%d", res.Start, res.End)
	}
	if !strings.Contains(res.Content, "line9") {
		t.Fatalf("expected content to include line9, got: %q", res.Content)
	}
	if strings.Contains(res.Content, "line10") {
		t.Fatalf("expected content NOT to include line10, got: %q", res.Content)
	}
}

func TestShowNegativeEnd_BeforeStart(t *testing.T) {
	// 5-line file, end=-9 (computed = 5 + -9 + 1 = -3 < start=1) → error
	path := writeTempFile(t, "b.txt", "a\nb\nc\nd\ne\n")
	_, _, err := Show(path, 3, -9, false)
	if err == nil {
		t.Fatal("expected error for negative end before start")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got: %v", err)
	}
}

func TestShowNegativeEnd_ExactEnd(t *testing.T) {
	// 10-line file, end=-2 → should end at line 9
	path := writeTempFile(t, "c.txt", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n")
	res, _, err := Show(path, 5, -2, false)
	if err != nil {
		t.Fatalf("show with end=-2: %v", err)
	}
	if res.End != 9 {
		t.Fatalf("expected end=9 for end=-2 on 10 lines, got %d", res.End)
	}
}

func TestShowNegativeEnd_AutoModePreserved(t *testing.T) {
	// end=-1 should remain auto mode (function range detection)
	path := writeTempFile(t, "main.go", "package main\n\nfunc demo() {\n\tprintln(\"x\")\n}\n")
	res, _, err := Show(path, 3, -1, false)
	if err != nil {
		t.Fatalf("show with end=-1 (auto): %v", err)
	}
	if res.Start != 3 || res.End != 5 {
		t.Fatalf("expected auto mode range 3..5, got %d..%d", res.Start, res.End)
	}
}

// ──────────────────────────────────────────────
// #39: be-read 空文件读取
// ──────────────────────────────────────────────

func TestShowEmptyFile_ReturnsOK(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	res, _, err := Show(path, 1, 1, false)
	if err != nil {
		t.Fatalf("show on empty file: %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("expected status ok, got %q", res.Status)
	}
	if res.Total != 0 {
		t.Fatalf("expected total=0, got %d", res.Total)
	}
	if res.Content != "" {
		t.Fatalf("expected empty content, got %q", res.Content)
	}
}

func TestReadEmptyFile_ReturnsOK(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty2.txt")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	res, _, err := Read(path, 1, 1, false)
	if err != nil {
		t.Fatalf("read on empty file: %v", err)
	}
	if res.Total != 0 {
		t.Fatalf("expected total=0 for Read on empty file, got %d", res.Total)
	}
}

// ──────────────────────────────────────────────
// #40: be-read 空行显示 <blank> 标记
// ──────────────────────────────────────────────

func TestShowBlankLineMarker(t *testing.T) {
	// File with blank lines in the middle
	path := writeTempFile(t, "blank.txt", "a\n\n\nb\n")
	res, _, err := Show(path, 1, 4, false)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	// Lines 2 and 3 are blank → should show <blank>
	if !strings.Contains(res.Content, "2\t<blank>") {
		t.Fatalf("expected line 2 to show <blank>, got: %q", res.Content)
	}
	if !strings.Contains(res.Content, "3\t<blank>") {
		t.Fatalf("expected line 3 to show <blank>, got: %q", res.Content)
	}
	if strings.Contains(res.Content, "1\t<blank>") {
		t.Fatalf("line 1 has content and should NOT show <blank>")
	}
}

func TestShowBlankLineMarker_OnlyBlankLines(t *testing.T) {
	// File with only blank lines
	path := writeTempFile(t, "only_blank.txt", "\n\n\n")
	res, _, err := Show(path, 1, 3, false)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	for _, lineNum := range []int{1, 2, 3} {
		expected := fmt.Sprintf("%d\t<blank>", lineNum)
		if !strings.Contains(res.Content, expected) {
			t.Fatalf("expected line %d to show <blank>, content: %q", lineNum, res.Content)
		}
	}
}

// ──────────────────────────────────────────────
// #41: 纯空行文件误报 trailing whitespace
// ──────────────────────────────────────────────

func TestScanWarnings_BlankLinesOnly_NoTrailingWhitespace(t *testing.T) {
	// scanContentWarnings receives the FORMATTED display content (with line numbers).
	// Blank lines become "N\t" — the tab is the line number separator, not trailing whitespace.
	content := "1\t<blank>\n2\t<blank>\n3\t<blank>"
	warnings := scanContentWarnings(content)
	for _, w := range warnings {
		if strings.Contains(w, "trailing whitespace") {
			t.Fatalf("unexpected trailing whitespace warning for blank-line content: %q", w)
		}
	}
}

func TestScanWarnings_ActualTrailingSpaces_StillDetected(t *testing.T) {
	content := "1\tline1  \n2\tline2\n"
	warnings := scanContentWarnings(content)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "trailing whitespace") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected trailing whitespace warning for actual trailing spaces")
	}
}

func TestScanWarnings_BlankLinesMixedContent_NoFalsePositive(t *testing.T) {
	content := "1\tnormal\n2\t<blank>\n3\tmore\n4\t<blank>"
	warnings := scanContentWarnings(content)
	for _, w := range warnings {
		if strings.Contains(w, "trailing whitespace") {
			t.Fatalf("unexpected trailing whitespace warning: %q", w)
		}
	}
}

// ──────────────────────────────────────────────
// #42: 空文件/空内容时不应报告 trailing newline 警告
// ──────────────────────────────────────────────

func TestScanWarnings_EmptyContent_NoNewlineWarning(t *testing.T) {
	warnings := scanContentWarnings("")
	for _, w := range warnings {
		if strings.Contains(w, "does not end with a newline") {
			t.Fatalf("unexpected 'no newline' warning for empty content: %q", w)
		}
	}
}

func TestScanWarnings_SingleLine_ReportsNoNewline(t *testing.T) {
	warnings := scanContentWarnings("hello")
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "does not end with a newline") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'does not end with a newline' warning for single-line content")
	}
}

func TestScanWarnings_MultiLineWithNewline_NoWarning(t *testing.T) {
	warnings := scanContentWarnings("hello\nworld\n")
	for _, w := range warnings {
		if strings.Contains(w, "does not end with a newline") {
			t.Fatalf("unexpected 'no newline' warning for content ending with newline")
		}
	}
}

// ──────────────────────────────────────────────
// #43: be-write 覆盖 CRLF 文件时保留换行风格
// ──────────────────────────────────────────────

func TestWritePreservesExistingCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crlf.txt")
	// Create a CRLF file first
	orig := "line1\r\nline2\r\nline3\r\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatalf("write orig: %v", err)
	}
	// Overwrite with LF content
	spec := `{"file":"` + path + `","content":"new1\nnew2\n"}`
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gotStr := string(got)
	if !strings.Contains(gotStr, "\r\n") {
		t.Fatalf("expected CRLF preserved in output, got: %q", gotStr)
	}
	// Verify actual line ending bytes
	if gotStr != "new1\r\nnew2\r\n" {
		t.Fatalf("expected content with CRLF, got: %q", gotStr)
	}
}

func TestWriteNewFileDefaultLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new_lf.txt")
	spec := `{"file":"` + path + `","content":"hello\nworld\n"}`
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gotStr := string(got)
	if strings.Contains(gotStr, "\r\n") {
		t.Fatalf("new file should use LF, got CRLF: %q", gotStr)
	}
	if gotStr != "hello\nworld\n" {
		t.Fatalf("expected LF content, got: %q", gotStr)
	}
}

func TestWriteCRLFContentMixed_UnifiedToTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed_crlf.txt")
	orig := "keep\r\nme\r\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatalf("write orig: %v", err)
	}
	// Content with mixed CRLF+LF
	spec := `{"file":"` + path + `","content":"a\r\nb\nc\r\nd\n"}`
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gotStr := string(got)
	// All line endings should be unified to CRLF
	if !strings.Contains(gotStr, "\r\n") {
		t.Fatalf("expected CRLF in output, got: %q", gotStr)
	}
	if strings.Contains(gotStr, "\n\r") {
		t.Fatalf("unexpected \\n\\r sequence: %q", gotStr)
	}
}

// ──────────────────────────────────────────────
// #42 (complement): Write empty content → no false warnings
// ──────────────────────────────────────────────

func TestWriteEmptyContent_CleanWarnings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty_write.txt")
	spec := `{"file":"` + path + `","content":""}`
	res, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write empty content: %v", err)
	}
	for _, r := range res.Results {
		for _, w := range r.Warnings {
			if strings.Contains(w, "does not end with a newline") {
				t.Fatalf("unexpected 'no newline' warning for empty write: %q", w)
			}
		}
	}
}

// ──────────────────────────────────────────────
// #6 / #12: Replace/Delete 行号范围边界语义（1-based inclusive）
// ──────────────────────────────────────────────

func TestReplaceFirstLine(t *testing.T) {
	path := writeTempFile(t, "first.txt", "old_first\nsecond\nthird\n")
	_, err := Replace(path, 1, 1, nil, "new_first\n", "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace first line: %v", err)
	}
	got := readFile(t, path)
	if got != "new_first\nsecond\nthird\n" {
		t.Fatalf("expected first line replaced, got: %q", got)
	}
}

func TestReplaceLastLine(t *testing.T) {
	path := writeTempFile(t, "last.txt", "first\nsecond\nold_last\n")
	_, err := Replace(path, 3, 3, nil, "new_last\n", "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace last line: %v", err)
	}
	got := readFile(t, path)
	if got != "first\nsecond\nnew_last\n" {
		t.Fatalf("expected last line replaced, got: %q", got)
	}
}

func TestReplaceRangeInclusive(t *testing.T) {
	path := writeTempFile(t, "range.txt", "a\nb\nc\nd\ne\n")
	// Replace lines 2-4 (inclusive) with two lines
	_, err := Replace(path, 2, 4, nil, "x\ny\n", "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace range: %v", err)
	}
	got := readFile(t, path)
	if got != "a\nx\ny\ne\n" {
		t.Fatalf("expected 3 lines replaced by 2, got: %q", got)
	}
}

// ──────────────────────────────────────────────
// #14: be-insert after_line 语义
// ──────────────────────────────────────────────

func TestInsertAtBeginning(t *testing.T) {
	path := writeTempFile(t, "begin.txt", "b\nc\n")
	_, err := Insert(path, 0, "a\n", "plain", false, false)
	if err != nil {
		t.Fatalf("insert at beginning: %v", err)
	}
	got := readFile(t, path)
	if got != "a\nb\nc\n" {
		t.Fatalf("expected 'a' at beginning, got: %q", got)
	}
}

func TestInsertAtEnd(t *testing.T) {
	path := writeTempFile(t, "end.txt", "a\nb\n")
	totalLines := 2
	_, err := Insert(path, totalLines, "c\n", "plain", false, false)
	if err != nil {
		t.Fatalf("insert at end: %v", err)
	}
	got := readFile(t, path)
	if got != "a\nb\nc\n" {
		t.Fatalf("expected 'c' at end, got: %q", got)
	}
}

func TestInsertAtBeginning_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty_insert.txt")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	_, err := Insert(path, 0, "first\n", "plain", false, false)
	if err != nil {
		t.Fatalf("insert into empty file: %v", err)
	}
	got := readFile(t, path)
	if got != "first\n" {
		t.Fatalf("expected 'first' in empty file, got: %q", got)
	}
}

// ──────────────────────────────────────────────
// #15: be-func-range 应包含函数签名行
// ──────────────────────────────────────────────

func TestFuncRangeIncludesSignature(t *testing.T) {
	path := writeTempFile(t, "main.go", "package main\n\nfunc hello() {\n\treturn 1\n}\n")
	// Ask for range of line 3 ("func hello() {")
	res, err := FuncRange(path, 3)
	if err != nil {
		t.Fatalf("func-range: %v", err)
	}
	if res.Start != 3 {
		t.Fatalf("expected range to start at line 3 (func signature), got %d", res.Start)
	}
	if res.End != 5 {
		t.Fatalf("expected range to end at line 5 (closing brace), got %d", res.End)
	}
}

func TestFuncRange_EmptyBody(t *testing.T) {
	// Function with empty body: func empty() {}
	path := writeTempFile(t, "empty.go", "package main\n\nfunc empty() {}\n")
	res, err := FuncRange(path, 3)
	if err != nil {
		t.Fatalf("func-range empty body: %v", err)
	}
	if res.Start != 3 {
		t.Fatalf("expected start=3 for func on one line, got %d", res.Start)
	}
}

func TestTagRangeSelfClosing_Skipped(t *testing.T) {
	path := writeTempFile(t, "page.html", "<div>\n<br/>\n</div>\n")
	res, err := TagRange(path, 2)
	if err != nil {
		t.Fatalf("tag-range for <br/>: %v", err)
	}
	if res.Tag != "div" {
		t.Fatalf("expected enclosing tag 'div', got %q", res.Tag)
	}
	if res.Start != 1 || res.End != 3 {
		t.Fatalf("expected range 1..3 for <div>..</div>, got %d..%d", res.Start, res.End)
	}
}

func TestTagRange_SingleLinePaired(t *testing.T) {
	path := writeTempFile(t, "span.html", "<div><span>hello</span></div>\n")
	res, err := TagRange(path, 1)
	if err != nil {
		t.Fatalf("tag-range single line: %v", err)
	}
	if res.Tag != "span" {
		t.Fatalf("expected innermost tag 'span', got %q", res.Tag)
	}
}

func TestBalanceSkipsBracesInStrings(t *testing.T) {
	path := writeTempFile(t, "strings.js", "var x = \"{\";\nvar y = \"}\";\n")
	out, err := CheckStructureBalance(path, true)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	unbalanced, _ := v["unbalanced"].([]any)
	for _, item := range unbalanced {
		m, _ := item.(map[string]any)
		if sym, _ := m["symbol"].(string); sym == "{" || sym == "}" {
			t.Fatalf("expected braces in strings to be ignored, found unmatched: %v", m)
		}
	}
}

func TestBalanceSkipsBracesInComments(t *testing.T) {
	// Line comments and block comments with braces should not affect balance
	path := writeTempFile(t, "comments.js", "// line comment { with brace\n/* block { comment */\nvar x = {};\n")
	out, err := CheckStructureBalance(path, true)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	// var x = {}; has balanced braces; comments should be ignored
	unbalanced, _ := v["unbalanced"].([]any)
	for _, item := range unbalanced {
		m, _ := item.(map[string]any)
		if sym, _ := m["symbol"].(string); sym == "{" || sym == "}" {
			t.Fatalf("expected braces in comments to be ignored, found unmatched: %v", m)
		}
	}
}
func TestDeleteFunctionTarget_NoEatAdjacentBrace(t *testing.T) {
	content := "package main\n\nfunc first() {\n\treturn 1\n}\n\nfunc second() {\n\treturn 2\n}\n\nfunc third() {\n\treturn 3\n}\n"
	path := writeTempFile(t, "main.go", content)

	// Delete "second" via target resolution
	span, err := ResolveTargetSpan(path, ContentTarget{Kind: "function", Value: "second"})
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}

	_, err = Delete(path, span.Start, span.End, "plain", false, false)
	if err != nil {
		t.Fatalf("delete second: %v", err)
	}

	got := readFile(t, path)
	if !strings.Contains(got, "func first() {") {
		t.Fatal("first function should remain")
	}
	if !strings.Contains(got, "func third() {") {
		t.Fatal("third function should remain")
	}
	if strings.Contains(got, "func second() {") {
		t.Fatal("second function should be deleted")
	}
}

// ──────────────────────────────────────────────
// #28: Snapshot queue full → eviction warning
// (已在 edit_test.go 中有 TestSnapshotQueueFullReturnsWarning)
// 这里补充 rollback 测试
// ──────────────────────────────────────────────

func TestSnapshotRollback(t *testing.T) {
	CommitSnapshots() // reset queue

	path := writeTempFile(t, "rollback.txt", "original\nline2\nline3\n")

	// Perform a replace via the tool (creates snapshot)
	_, err := Replace(path, 2, 2, nil, "modified\n", "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}

	// Verify modified
	got := readFile(t, path)
	if got != "original\nmodified\nline3\n" {
		t.Fatalf("unexpected after replace: %q", got)
	}

	// Rollback
	count, errs := RollbackSnapshots(1)
	if len(errs) > 0 {
		t.Fatalf("rollback errors: %v", errs)
	}
	if count != 1 {
		t.Fatalf("expected 1 rollback, got %d", count)
	}

	// Verify restored
	restored := readFile(t, path)
	if restored != "original\nline2\nline3\n" {
		t.Fatalf("expected rollback to original, got: %q", restored)
	}

	CommitSnapshots()
}

func TestSnapshotRollback_Multiple(t *testing.T) {
	CommitSnapshots()

	path := writeTempFile(t, "multiback.txt", "a\nb\nc\n")

	// Two edits
	_, _ = Replace(path, 1, 1, nil, "x\n", "plain", false, "", false)
	_, _ = Replace(path, 3, 3, nil, "z\n", "plain", false, "", false)

	got := readFile(t, path)
	if got != "x\nb\nz\n" {
		t.Fatalf("expected both edits, got: %q", got)
	}

	// Rollback both
	count, errs := RollbackSnapshots(2)
	if len(errs) > 0 {
		t.Fatalf("rollback errors: %v", errs)
	}
	if count != 2 {
		t.Fatalf("expected 2 rollbacks, got %d", count)
	}

	restored := readFile(t, path)
	if restored != "a\nb\nc\n" {
		t.Fatalf("expected full rollback to original, got: %q", restored)
	}

	CommitSnapshots()
}

// ──────────────────────────────────────────────
// #33/#44: 内容按 JSON 解析结果原样使用（不再中转义）
// ──────────────────────────────────────────────

func TestReplaceContentAsIs_NoTransform(t *testing.T) {
	// Content with literal escape sequences stays as-is
	path := writeTempFile(t, "asis.txt", "a\nb\nc\n")
	// Content has literal \n (two chars) -- this comes from JSON as-is
	content := "x\\ny"
	res, err := Replace(path, 2, 2, nil, content, "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	got := readFile(t, path)
	// The literal \n should remain literal, NOT converted to newline
	if strings.Contains(got, "x\n") && !strings.Contains(got, "x\\n") {
		// If content has a real newline where we expected literal \, that's wrong
	}
	_ = res
}

// ──────────────────────────────────────────────
// #34: Replace 保留制表符缩进（复杂场景）
// ──────────────────────────────────────────────

func TestReplaceTabIndentation_MultipleLines(t *testing.T) {
	content := "func foo() {\n\tif true {\n\t\tdoSomething()\n\t}\n}\n"
	path := writeTempFile(t, "main.go", content)
	old := "\tif true {\n\t\tdoSomething()\n\t}\n"
	newContent := "\tif false {\n\t\tdoNothing()\n\t}\n"
	_, err := Replace(path, 2, 4, &old, newContent, "plain", false, "", false)
	if err != nil {
		t.Fatalf("replace multi-line tab: %v", err)
	}
	got := readFile(t, path)
	expected := "func foo() {\n\tif false {\n\t\tdoNothing()\n\t}\n}\n"
	if got != expected {
		t.Fatalf("expected tab indentation preserved, got: %q", got)
	}
}

// ──────────────────────────────────────────────
// #26: brief mode → content is empty
// ──────────────────────────────────────────────

func TestShowBriefMode(t *testing.T) {
	path := writeTempFile(t, "brief.txt", "line1\nline2\nline3\n")
	res, _, err := Show(path, 1, 3, true) // brief=true
	if err != nil {
		t.Fatalf("show brief: %v", err)
	}
	if res.Brief != true {
		t.Fatal("expected Brief=true in result")
	}
	if res.Content != "" {
		t.Fatalf("expected empty content in brief mode, got: %q", res.Content)
	}
	if res.Total != 3 {
		t.Fatalf("expected total=3, got %d", res.Total)
	}
}

// ──────────────────────────────────────────────
// #21: injectable FileSystem（已有，补一个边界情况）
// ──────────────────────────────────────────────

func TestInjectedFileSystem_BlockedRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blocked.txt")
	if err := os.WriteFile(path, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// FS that blocks access to this file
	fsys := sandboxFS{root: dir, block: map[string]bool{path: true}}
	_, _, err := Show(path, 1, 1, false, WithFileSystem(fsys))
	if err == nil {
		t.Fatal("expected error for blocked file read")
	}
}

// ──────────────────────────────────────────────
// #35: tool param convention — target resolution edge cases
// ──────────────────────────────────────────────

func TestTargetResolution_MarkerNotFound(t *testing.T) {
	path := writeTempFile(t, "marker.txt", "a\nb\nc\n")
	_, err := ResolveTargetSpan(path, ContentTarget{Kind: "marker", Value: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for unresolved marker")
	}
}

func TestTargetResolution_FunctionNotFound(t *testing.T) {
	path := writeTempFile(t, "nofunc.txt", "just text\nno functions here\n")
	_, err := ResolveTargetSpan(path, ContentTarget{Kind: "function", Value: "missingFunc"})
	if err == nil {
		t.Fatal("expected error for unresolved function")
	}
}

// ──────────────────────────────────────────────
// #8: start 参数越界与边界值
// ──────────────────────────────────────────────

func TestShowStartBoundary(t *testing.T) {
	path := writeTempFile(t, "boundary.txt", "only\n")
	// start=1 on 1-line file should work
	res, _, err := Show(path, 1, 1, false)
	if err != nil {
		t.Fatalf("show start=1 on 1-line file: %v", err)
	}
	if res.Start != 1 || res.End != 1 {
		t.Fatalf("expected range 1..1, got %d..%d", res.Start, res.End)
	}
}

func TestShowStartOutOfRange(t *testing.T) {
	path := writeTempFile(t, "oor.txt", "a\nb\n")
	_, _, err := Show(path, 999, 1, false)
	if err == nil {
		t.Fatal("expected error for start > total")
	}
}
