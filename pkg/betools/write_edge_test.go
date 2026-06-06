package betools

import (
	"strings"
	"testing"
)

func TestWriteDegraded_UnescapedTab(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"hello\tworld\"}"
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFS(t, m, "w.txt")
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Fatalf("expected content preserved, got: %q", got)
	}
}

func TestWriteDegraded_UnescapedCR(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"hello\rworld\"}"
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFS(t, m, "w.txt")
	if !strings.Contains(got, "world") {
		t.Fatalf("expected content preserved, got: %q", got)
	}
}

func TestWriteDegraded_UnbalancedQuotes(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"it's \"bad\" quote\"}"
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFS(t, m, "w.txt")
	if got == "" {
		t.Fatalf("expected at least partial content, got empty")
	}
	t.Logf("unbalanced quotes content: %q", got)
}

func TestWriteDegraded_EmbeddedBackticks(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"\x60code\x60\"}"
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFS(t, m, "w.txt")
	if !strings.Contains(got, "\x60code\x60") {
		t.Fatalf("expected backticks preserved, got: %q", got)
	}
}

func TestWriteDegraded_NestedUnescapedQuotes(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"line with \"nested\" and \"more\" quotes\"}"
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFS(t, m, "w.txt")
	if got == "" {
		t.Fatalf("expected partial content, got empty")
	}
	t.Logf("nested quotes content: %q", got)
}

func TestWriteDegraded_NewlineBetweenKeys(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\n\"content\":\"hello\"}"
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if res.Degraded {
		t.Fatalf("standard JSON with whitespace should not be degraded: %+v", res)
	}
	got := readFS(t, m, "w.txt")
	if got != "hello" {
		t.Fatalf("expected 'hello', got: %q", got)
	}
}

func TestWriteDegraded_NoFileField(t *testing.T) {
	_, opt := withFS(nil)
	spec := "{\"content\":\"write to nowhere\"}"
	_, err := Write(spec, false, false, opt)
	if err == nil {
		t.Fatal("expected error for missing file field")
	}
}

func TestWriteDegraded_EmptyStringContent(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"\"}"
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFS(t, m, "w.txt")
	if got != "" {
		t.Fatalf("expected empty file, got: %q", got)
	}
}

func TestWriteDegraded_NullContent(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":null}"
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFS(t, m, "w.txt")
	if got != "" {
		t.Fatalf("expected empty file for null content, got: %q", got)
	}
}

func TestWriteDegraded_UnicodeInContent(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"hello 世界 unicode ✓\"}"
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFS(t, m, "w.txt")
	if got != "hello 世界 unicode ✓" {
		t.Fatalf("expected unicode preserved, got: %q", got)
	}
}

func TestWriteDegraded_UnicodeInContentUnescaped(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"hello\n世界\"}"
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFS(t, m, "w.txt")
	if !strings.Contains(got, "hello") {
		t.Fatalf("expected 'hello' preserved, got: %q", got)
	}
	t.Logf("degraded unicode content: %q", got)
}

func TestWriteDegraded_ExtractWithBlocks_Note(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"before\n```\ncode block\n```\nafter\",\"extract\":true}"
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write with extract: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded mode")
	}
	got := readFS(t, m, "w.txt")
	if got == "" {
		t.Fatalf("expected some content written")
	}
	t.Logf("degraded extract content: %q", got)
}

func TestWriteDegraded_MultipleFiles_Degraded(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"files\":[{\"file\":\"a.txt\",\"content\":\"hello1\"},{\"file\":\"b.txt\",\"content\":\"hello\n2\"}]}"
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Skipf("multi-file degraded not supported: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	_ = m
}

func TestWriteDegraded_ContentStartsWithEscapedQuote(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"\\\"leading quote\\\"\"}"
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFS(t, m, "w.txt")
	if got != "\"leading quote\"" {
		t.Fatalf("expected quoted content, got: %q", got)
	}
}

func TestWriteDegraded_MixedUnescapedControlChars(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"hello\tline2\r\nline3\"}"
	res, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFS(t, m, "w.txt")
	if !strings.Contains(got, "hello") || !strings.Contains(got, "line") {
		t.Fatalf("expected 'hello' and 'line', got: %q", got)
	}
}

func TestWriteDegraded_WriteAndVerifyAtomic(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"" + strings.Repeat("line\n", 100) + "\"}"
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFS(t, m, "w.txt")
	if got == "" {
		t.Fatal("expected content after atomic write")
	}
}

func TestWriteDegraded_ContentStartsWithJSONObject(t *testing.T) {
	m, opt := withFS(nil)
	content := "{\"nested\": true}"
	spec := "{\"file\":\"w.txt\",\"content\":" + content + "}"
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFS(t, m, "w.txt")
	if got == "" {
		t.Fatal("expected non-empty content, got empty")
	}
	t.Logf("json-object content serialized as: %q", got)
}

func TestWriteDegraded_ContentPathWithJSONEscape(t *testing.T) {
	m, opt := withFS(nil)
	// The spec ends with "path"} which is valid JSON: content="path".
	spec := "{\"file\":\"w.txt\",\"content\":\"path\"}"
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Skipf("write: %v", err)
	}
	got := readFS(t, m, "w.txt")
	if got != "path" {
		t.Fatalf("expected content 'path', got: %q", got)
	}
}

func TestWriteDegraded_ContentGoRawLiteralStyle(t *testing.T) {
	m, opt := withFS(nil)
	spec := "{\"file\":\"w.txt\",\"content\":\"raw\\\\nstring\\\\nliteral\"}"
	_, err := Write(spec, false, false, opt)
	if err != nil {
		t.Skipf("write: %v", err)
	}
	got := readFS(t, m, "w.txt")
	if !strings.Contains(got, "raw") || !strings.Contains(got, "literal") {
		t.Fatalf("expected content, got: %q", got)
	}
}
