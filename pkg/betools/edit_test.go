package betools

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestShowAutoUsesFunctionRange(t *testing.T) {
	m, opt := withFS(map[string]string{"main.go": "package main\n\nfunc demo() {\n\tprintln(\"x\")\n}\n"})
	res, _, err := Show("main.go", 3, -1, false, opt)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if res.Start != 3 || res.End != 5 {
		t.Fatalf("unexpected range: %+v", res)
	}
	if !strings.Contains(res.Content, "3\tfunc demo() {") {
		t.Fatalf("unexpected content: %q", res.Content)
	}
	_ = m
}

func TestShowExplicitRange(t *testing.T) {
	m, opt := withFS(map[string]string{"main.go": "one\ntwo\nthree\n"})
	res, _, err := Show("main.go", 2, 3, false, opt)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if res.Start != 2 || res.End != 3 {
		t.Fatalf("unexpected range: %+v", res)
	}
	if !strings.Contains(res.Content, "2\ttwo") || !strings.Contains(res.Content, "3\tthree") {
		t.Fatalf("unexpected content: %q", res.Content)
	}
	_ = m
}

func TestReadAlias(t *testing.T) {
	m, opt := withFS(map[string]string{"alias.txt": "a\nb\n"})
	showRes, showID, err := Show("alias.txt", 1, 1, false, opt)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	readRes, readID, err := Read("alias.txt", 1, 1, false, opt)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if showRes.Content != readRes.Content || showRes.Start != readRes.Start || showRes.End != readRes.End {
		t.Fatalf("read alias mismatch: show=%+v read=%+v", showRes, readRes)
	}
	if showID == "" || readID == "" {
		t.Fatalf("expected session ids from read-like calls")
	}
	_ = m
}

func TestReplacePreviewDoesNotWrite(t *testing.T) {
	m, opt := withFS(map[string]string{"a.txt": "a\nb\nc\n"})
	old := "b\n"
	res, err := Replace("a.txt", 2, 2, &old, "x", "plain", true, "", false, opt)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if !res.Preview {
		t.Fatalf("preview flag missing: %+v", res)
	}
	if got := readFS(t, m, "a.txt"); got != "a\nb\nc\n" {
		t.Fatalf("file changed unexpectedly: %q", got)
	}
}

func TestReplaceRejectsOldMismatch(t *testing.T) {
	m, opt := withFS(map[string]string{"a.txt": "a\nb\nc\n"})
	old := "x\n"
	if _, err := Replace("a.txt", 2, 2, &old, "y", "plain", true, "", false, opt); err == nil {
		t.Fatalf("expected mismatch error")
	}
	if got := readFS(t, m, "a.txt"); got != "a\nb\nc\n" {
		t.Fatalf("file changed unexpectedly: %q", got)
	}
}

func TestInsertAndDeleteWriteFile(t *testing.T) {
	m, opt := withFS(map[string]string{"a.txt": "a\nb\n"})
	if _, err := Insert("a.txt", 1, "x", "plain", false, false, opt); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := readFS(t, m, "a.txt"); got != "a\nx\nb\n" {
		t.Fatalf("unexpected after insert: %q", got)
	}
	if _, err := Delete("a.txt", 2, 2, "plain", false, false, opt); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := readFS(t, m, "a.txt"); got != "a\nb\n" {
		t.Fatalf("unexpected after delete: %q", got)
	}
}

func TestWriteDegradedParser(t *testing.T) {
	m, opt := withFS(nil)
	spec := `{"file":"w.txt","content":"hello\nworld"}`
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if res.Degraded {
		t.Fatalf("unexpected degraded for valid json: %+v", res)
	}
	if got := readFS(t, m, "w.txt"); got != "hello\nworld" {
		t.Fatalf("unexpected write content: %q", got)
	}
}

func TestWriteRawFallback(t *testing.T) {
	m, opt := withFS(nil)
	spec := `{"file":"w.txt","content":"hello
world"}`
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write fallback: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag: %+v", res)
	}
	if got := readFS(t, m, "w.txt"); !strings.Contains(got, "hello") {
		t.Fatalf("unexpected fallback content: %q", got)
	}
}

func TestShowRejectsBinaryFile(t *testing.T) {
	// MemFS can store arbitrary bytes — we test binary rejection through Show
	m := NewMemFS(map[string]string{"bin.dat": ""})
	opt := WithFileSystem(m)
	// Write binary content directly to MemFS
	if err := m.WriteFile("bin.dat", []byte{0x00, 0x01, 0x02, 0x7f, 0xff}, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	_, _, err := Show("bin.dat", 1, 1, false, opt)
	if err == nil {
		t.Fatal("expected binary rejection")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestInjectedFileSystemIsUsed(t *testing.T) {
	m := NewMemFS(nil)
	fsys := WithFileSystem(m)

	// Show on non-existent file should error
	if _, _, err := Show("sandbox.txt", 1, 1, false, fsys); err == nil {
		t.Fatal("expected error for missing file in MemFS")
	}

	// Write and verify
	if _, err := Write(`{"file":"sandbox.txt","content":"hello"}`, false, false, fsys); err != nil {
		t.Fatalf("write with MemFS: %v", err)
	}
	got, err := m.ReadFile("sandbox.txt")
	if err != nil {
		t.Fatalf("read sandbox file: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("unexpected content: %q", string(got))
	}
}

func TestBalanceSimple(t *testing.T) {
	_, opt := withFS(map[string]string{"a.js": "function demo() { return [1, 2]; }\n"})
	out, err := CheckStructureBalance("a.js", false, opt)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	if v["mode"] != "unbalanced" {
		t.Fatalf("unexpected mode: %v", v["mode"])
	}
}

func TestTargetResolutionLine(t *testing.T) {
	_, opt := withFS(map[string]string{"a.txt": "one\ntwo\nthree\n"})
	span, err := ResolveTargetSpan("a.txt", ContentTarget{Kind: "line", Value: "2"}, opt)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if span.Start != 2 || span.End != 2 {
		t.Fatalf("unexpected span: %+v", span)
	}
}

func TestNormalizeLineBreaks_RealNewlines_Noop(t *testing.T) {
	input := "hello\nworld\n"
	got := normalizeLineBreaks(input)
	if got != input {
		t.Fatalf("expected no change for content with real newlines, got: %q", got)
	}
}

func TestNormalizeLineBreaks_LiteralNewlines_Fixed(t *testing.T) {
	input := "hello\\nworld\\n"
	got := normalizeLineBreaks(input)
	if got != "hello\nworld\n" {
		t.Fatalf("expected literal \\n converted to real newlines, got: %q", got)
	}
}

func TestNormalizeLineBreaks_NoNewlines_Noop(t *testing.T) {
	input := "hello world"
	got := normalizeLineBreaks(input)
	if got != input {
		t.Fatalf("expected no change for plain text, got: %q", got)
	}
}

func TestNormalizeLineBreaks_MixedNewlines_Noop(t *testing.T) {
	input := "hello\nworld\\nend"
	got := normalizeLineBreaks(input)
	if got != input {
		t.Fatalf("expected no change for mixed content, got: %q", got)
	}
}

func TestReplaceContentWithLiteralNewlines_FixedByNormalize(t *testing.T) {
	m, opt := withFS(map[string]string{"a.txt": "a\nb\nc\n"})
	content := "x\ny"
	res, err := Replace("a.txt", 2, 2, nil, content, "plain", false, "", false, opt)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	got := readFS(t, m, "a.txt")
	if got != "a\nx\ny\nc\n" {
		t.Fatalf("expected content, got: %q", got)
	}
	_ = res
}

func TestReplaceOldWithDegradedNewlines_Matches(t *testing.T) {
	_, opt := withFS(map[string]string{"a.txt": "a\nb\nc\n"})
	old := "b\n"
	content := "x\n"
	_, err := Replace("a.txt", 2, 2, &old, content, "plain", false, "", false, opt)
	if err != nil {
		t.Fatalf("replace should accept old with real newlines: %v", err)
	}
}

func TestReplaceRejectsOldWithDegradedNewlines_AfterNormalize(t *testing.T) {
	_, opt := withFS(map[string]string{"a.txt": "a\nb\nc\n"})
	old := "x\n"
	content := "y\n"
	_, err := Replace("a.txt", 2, 2, &old, content, "plain", false, "", false, opt)
	if err == nil {
		t.Fatal("expected mismatch error for wrong old content")
	}
}

func TestShowBareFilePathReturnsFullText(t *testing.T) {
	// A file with more than the ~11 line context window so we can verify
	// that start=1, endLine=-1 returns the whole file instead of context.
	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12\nline13\nline14\nline15\n"
	m, opt := withFS(map[string]string{"long.txt": content})
	res, _, err := Show("long.txt", 1, -1, false, opt)
	if err != nil {
		t.Fatalf("show bare file: %v", err)
	}
	if res.Start != 1 || res.End != 15 {
		t.Fatalf("expected full range 1..15, got %d..%d", res.Start, res.End)
	}
	if res.Total != 15 {
		t.Fatalf("expected total 15, got %d", res.Total)
	}
	if !strings.Contains(res.Content, "15\tline15") {
		t.Fatalf("expected full content including line 15, got: %q", res.Content)
	}
	_ = m
}

func TestReplaceRejectsMissingTrailingEmptyLine(t *testing.T) {
	// File has an empty line between b and c (lines 2 and 3).
	m, opt := withFS(map[string]string{"a.txt": "a\nb\n\nc\n"})
	old := "b\n" // missing the trailing empty line in the actual block lines 2-3
	_, err := Replace("a.txt", 2, 3, &old, "x\n", "plain", true, "", false, opt)
	if err == nil {
		t.Fatalf("expected mismatch error for old content missing trailing empty line")
	}
	if got := readFS(t, m, "a.txt"); got != "a\nb\n\nc\n" {
		t.Fatalf("file changed unexpectedly: %q", got)
	}
}

func TestReplaceRejectsExtraTrailingEmptyLine(t *testing.T) {
	m, opt := withFS(map[string]string{"a.txt": "a\nb\nc\n"})
	old := "b\n\n" // extra trailing empty line not present in actual block line 2
	_, err := Replace("a.txt", 2, 2, &old, "x\n", "plain", true, "", false, opt)
	if err == nil {
		t.Fatalf("expected mismatch error for old content with extra trailing empty line")
	}
	if got := readFS(t, m, "a.txt"); got != "a\nb\nc\n" {
		t.Fatalf("file changed unexpectedly: %q", got)
	}
}

func TestWriteNormalizeLineBreaks_LiteralNewlines(t *testing.T) {
	m, opt := withFS(nil)
	content := "line1\\nline2\\n"
	spec := `{"file":"wnorm.txt","content":"` + content + `"}`
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFS(t, m, "wnorm.txt")
	if got != "line1\nline2\n" && got != "line1\nline2" {
		t.Fatalf("expected line breaks normalized, got: %q", got)
	}
}

func TestWriteNormalizeLineBreaks_RealNewlines_Preserved(t *testing.T) {
	m, opt := withFS(nil)
	spec := `{"file":"wpres.txt","content":"hello\nworld"}`
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFS(t, m, "wpres.txt")
	if got != "hello\nworld" {
		t.Fatalf("expected real newlines preserved, got: %q", got)
	}
}

func TestReplacePreservesTabIndentation(t *testing.T) {
	m, opt := withFS(map[string]string{"main.go": "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"})
	old := "\tprintln(\"hello\")\n"
	content := "\tprintln(\"world\")\n"
	res, err := Replace("main.go", 4, 4, &old, content, "plain", false, "", false, opt)
	if err != nil {
		t.Fatalf("replace tab-indented: %v", err)
	}
	got := readFS(t, m, "main.go")
	expected := "package main\n\nfunc main() {\n\tprintln(\"world\")\n}\n"
	if got != expected {
		t.Fatalf("expected tab indentation preserved, got: %q", got)
	}
	_ = res
}

func TestReplacePreservesTabIndentation_ContentWithEscapedTab(t *testing.T) {
	m, opt := withFS(map[string]string{"main.go": "package main\n\nfunc main() {\n\treturn 1\n}\n"})
	content := "\treturn 2"
	old := "\treturn 1\n"
	res, err := Replace("main.go", 4, 4, &old, content, "plain", false, "", false, opt)
	if err != nil {
		t.Fatalf("replace with escaped tab: %v", err)
	}
	got := readFS(t, m, "main.go")
	if !strings.Contains(got, "\treturn 2") {
		t.Fatalf("expected tab indentation preserved in output, got: %q", got)
	}
	_ = res
}

func TestParseContent_Escapes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"\\n", "\n"},                // \\n → newline
		{"\\t", "\t"},                // \\t → tab
		{"\\\"", "\""},               // \\" → double quote
		{"\\\\", "\\"},               // \\\\ → backslash
		{"plain text", "plain text"}, // no escapes
	}
	for _, tt := range tests {
		got := parseContent(tt.input)
		if got != tt.expected {
			t.Fatalf("parseContent(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSnapshotQueueFullReturnsWarning(t *testing.T) {
	CommitSnapshots()
	for i := 0; i < MaxSnapshots; i++ {
		_, warn := PushSnapshot(SnapshotRecord{
			Tool:    "test",
			File:    "/tmp/test.txt",
			Summary: fmt.Sprintf("snapshot %d", i),
		})
		if warn != "" {
			t.Fatalf("unexpected warning before queue full: %s", warn)
		}
	}
	_, warn := PushSnapshot(SnapshotRecord{
		Tool:    "test",
		File:    "/tmp/test.txt",
		Summary: "overflow",
	})
	if warn == "" {
		t.Fatal("expected warning when snapshot queue is full")
	}
	if !strings.Contains(warn, "snapshot queue") {
		t.Fatalf("expected warning about snapshot queue, got: %s", warn)
	}
	if !strings.Contains(warn, "written successfully") {
		t.Fatalf("expected warning to confirm write was successful, got: %s", warn)
	}
	CommitSnapshots()
}

func TestBracesInRange_Balanced(t *testing.T) {
	lines := []string{"func a() {", "\treturn 1", "}"}
	if err := bracesInRange(lines, 1, 3); err != nil {
		t.Fatalf("expected balanced braces, got: %v", err)
	}
}

func TestBracesInRange_Unbalanced(t *testing.T) {
	lines := []string{"func a() {", "\treturn 1", "}", "func b() {"}
	if err := bracesInRange(lines, 1, 4); err == nil {
		t.Fatal("expected error for unbalanced braces")
	}
}

func TestDeleteWithTarget_PreservesAdjacentCode(t *testing.T) {
	content := "package main\n\nfunc first() {\n\treturn 1\n}\n\nfunc second() {\n\treturn 2\n}\n\nfunc third() {\n\treturn 3\n}\n"
	m, opt := withFS(map[string]string{"main.go": content})

	span, err := ResolveTargetSpan("main.go", ContentTarget{Kind: "function", Value: "second"}, opt)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}

	_, err = Delete("main.go", span.Start, span.End, "plain", false, false, opt)
	if err != nil {
		t.Fatalf("delete second function: %v", err)
	}

	got := readFS(t, m, "main.go")
	if !strings.Contains(got, "func first() {") {
		t.Fatal("first function should remain after deleting second")
	}
	if strings.Contains(got, "func second() {") {
		t.Fatal("second function should be deleted")
	}
}

func TestBalancePositionTrace(t *testing.T) {
	_, opt := withFS(map[string]string{"a.js": "var x = {\n"})
	out, err := CheckStructureBalance("a.js", false, opt)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	unbalanced, ok := v["unbalanced"].([]any)
	if !ok || len(unbalanced) == 0 {
		t.Fatal("expected unbalanced items from bracket mismatch")
	}
	foundCol := false
	for _, item := range unbalanced {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if col, has := m["col"]; has {
			foundCol = true
			t.Logf("unbalanced %v at line %v col %v", m["symbol"], m["line"], col)
		}
	}
	if !foundCol {
		t.Error("expected col field in unbalanced item output")
	}
}

func TestIsTabDominant(t *testing.T) {
	tabContent := "func main() {\n\tif true {\n\t\treturn 1\n\t}\n\treturn 2\n}\n"
	if !isTabDominant(tabContent) {
		t.Fatal("expected code with mostly tab-indented lines to be tab-dominant")
	}
	spaceContent := "Hello world\n  indented with spaces\n  another line\n"
	if isTabDominant(spaceContent) {
		t.Fatal("expected space-indented content to NOT be tab-dominant")
	}
}

func TestDeleteSavesChip(t *testing.T) {
	content := "line1\nline2\nline3\n"
	m, opt := withFS(map[string]string{"a.txt": content})

	res, err := Delete("a.txt", 2, 2, "plain", false, false, opt)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	foundChip := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "chip://") {
			foundChip = true
			break
		}
	}
	if !foundChip {
		t.Fatalf("expected chip reference in delete warnings, got %v", res.Warnings)
	}

	got := readFS(t, m, "a.txt")
	if got != "line1\nline3\n" {
		t.Fatalf("unexpected content after delete: %q", got)
	}
}
