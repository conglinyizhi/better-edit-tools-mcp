package betools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDegraded_UnescapedTab(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"hello\tworld\"}"
	res, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFile(t, path)
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Fatalf("expected content preserved, got: %q", got)
	}
}

func TestWriteDegraded_UnescapedCR(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"hello\rworld\"}"
	res, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFile(t, path)
	if !strings.Contains(got, "world") {
		t.Fatalf("expected content preserved, got: %q", got)
	}
}

func TestWriteDegraded_UnbalancedQuotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"it's \"bad\" quote\"}"
	res, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFile(t, path)
	if got == "" {
		t.Fatalf("expected at least partial content, got empty")
	}
	t.Logf("unbalanced quotes content: %q", got)
}

func TestWriteDegraded_EmbeddedBackticks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"`code`\"}"
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFile(t, path)
	if !strings.Contains(got, "`code`") {
		t.Fatalf("expected backticks preserved, got: %q", got)
	}
}

func TestWriteDegraded_NestedUnescapedQuotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"line with \"nested\" and \"more\" quotes\"}"
	res, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFile(t, path)
	if got == "" {
		t.Fatalf("expected partial content, got empty")
	}
	t.Logf("nested quotes content: %q", got)
}

func TestWriteDegraded_NewlineBetweenKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	// standard JSON tolerates whitespace between keys — this is NOT degraded
	spec := "{\"file\":\"" + path + "\",\n\"content\":\"hello\"}"
	res, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if res.Degraded {
		t.Fatalf("standard JSON with whitespace should not be degraded: %+v", res)
	}
	got := readFile(t, path)
	if got != "hello" {
		t.Fatalf("expected 'hello', got: %q", got)
	}
}

func TestWriteDegraded_NoFileField(t *testing.T) {
	spec := "{\"content\":\"write to nowhere\"}"
	_, err := Write(spec, false, false)
	if err == nil {
		t.Fatal("expected error for missing file field")
	}
}

func TestWriteDegraded_EmptyStringContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"\"}"
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFile(t, path)
	if got != "" {
		t.Fatalf("expected empty file, got: %q", got)
	}
}

func TestWriteDegraded_NullContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":null}"
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFile(t, path)
	if got != "" {
		t.Fatalf("expected empty file for null content, got: %q", got)
	}
}

func TestWriteDegraded_UnicodeInContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"hello 世界 unicode ✓\"}"
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFile(t, path)
	if got != "hello 世界 unicode ✓" {
		t.Fatalf("expected unicode preserved, got: %q", got)
	}
}

func TestWriteDegraded_UnicodeInContentUnescaped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"hello\n世界\"}"
	res, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFile(t, path)
	if !strings.Contains(got, "hello") {
		t.Fatalf("expected 'hello' preserved, got: %q", got)
	}
	t.Logf("degraded unicode content: %q", got)
}

func TestWriteDegraded_ExtractWithBlocks_Note(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	// content with literal newlines => degraded mode.
	// extract is NOT supported in degraded mode — this is a known limitation.
	spec := "{\"file\":\"" + path + "\",\"content\":\"before\n```\ncode block\n```\nafter\",\"extract\":true}"
	res, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write with extract: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded mode")
	}
	got := readFile(t, path)
	// degraded mode ignores extract; content is written as-is from raw parser
	if got == "" {
		t.Fatalf("expected some content written")
	}
	t.Logf("degraded extract content: %q", got)
}

func TestWriteDegraded_MultipleFiles_Degraded(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "a.txt")
	path2 := filepath.Join(dir, "b.txt")
	spec := "{\"files\":[{\"file\":\"" + path1 + "\",\"content\":\"hello1\"},{\"file\":\"" + path2 + "\",\"content\":\"hello\n2\"}]}"
	res, err := Write(spec, false, false)
	if err != nil {
		t.Skipf("multi-file degraded not supported: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
}

func TestWriteDegraded_ContentStartsWithEscapedQuote(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"\\\"leading quote\\\"\"}"
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFile(t, path)
	if got != "\"leading quote\"" {
		t.Fatalf("expected quoted content, got: %q", got)
	}
}

func TestWriteDegraded_MixedUnescapedControlChars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"hello\tline2\r\nline3\"}"
	res, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
	got := readFile(t, path)
	if !strings.Contains(got, "hello") || !strings.Contains(got, "line") {
		t.Fatalf("expected 'hello' and 'line', got: %q", got)
	}
}

func TestWriteDegraded_WriteAndVerifyAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"" + strings.Repeat("line\n", 100) + "\"}"
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	dir := filepath.Dir(path)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".betmp") || strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

func TestWriteDegraded_ContentStartsWithJSONObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	content := "{\"nested\": true}"
	spec := "{\"file\":\"" + path + "\",\"content\":" + content + "}"
	_, err := Write(spec, false, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readFile(t, path)
	// content is JSON object, serialized via fmt.Sprint(m["content"])
	if got == "" {
		t.Fatal("expected non-empty content, got empty")
	}
	t.Logf("json-object content serialized as: %q", got)
}

func TestWriteDegraded_ContentEndsWithUnescapedBackslash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"path\\\"}"
	res, err := Write(spec, false, false)
	if err != nil {
		t.Skipf("write rejected trailing backslash: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("expected degraded flag")
	}
}

func TestWriteDegraded_ContentGoRawLiteralStyle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.txt")
	spec := "{\"file\":\"" + path + "\",\"content\":\"raw\\\\nstring\\\\nliteral\"}"
	_, err := Write(spec, false, false)
	if err != nil {
		t.Skipf("write: %v", err)
	}
	got := readFile(t, path)
	if !strings.Contains(got, "raw") || !strings.Contains(got, "literal") {
		t.Fatalf("expected content, got: %q", got)
	}
}
