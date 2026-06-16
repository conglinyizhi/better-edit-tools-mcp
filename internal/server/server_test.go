package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/conglinyizhi/better-edit-tools-mcp/internal/app"
)

func TestToolsListAndCall(t *testing.T) {
	srv := New(app.LangFromEnv(), false)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	scanner := bufio.NewScanner(&out)
	if !scanner.Scan() {
		t.Fatalf("no response")
	}
	var resp map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp["jsonrpc"] != "2.0" {
		t.Fatalf("unexpected jsonrpc: %v", resp["jsonrpc"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok || len(result["tools"].([]any)) == 0 {
		t.Fatalf("tools missing: %#v", resp["result"])
	}
	seenRead := false
	for _, item := range result["tools"].([]any) {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch m["name"] {
		case "be-read":
			seenRead = true
		}
	}
	if !seenRead {
		t.Fatalf("expected read tool, got: %#v", result["tools"])
	}
}

func TestInitializedNotificationNoResponse(t *testing.T) {
	srv := New("en", false)
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no output, got %s", out.String())
	}
}

func TestToolCallRunsEdit(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	path := dir + "/a.txt"
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-insert","arguments":{"file":"` + path + `:0","content":"x","preview":false}}}`
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	scanner := bufio.NewScanner(&out)
	if !scanner.Scan() {
		t.Fatalf("no response")
	}
	var resp map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected error response: %#v", result)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "x\na\nb\n" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestToolCallSupportsReadAlias(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	path := dir + "/read.txt"
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-read","arguments":{"file":"` + path + `"}}}`
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve read: %v", err)
	}
	scanner := bufio.NewScanner(&out)
	if !scanner.Scan() {
		t.Fatalf("no read response")
	}
	var resp map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("read json: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing read result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected read error response: %#v", result)
	}
}

func TestToolCallSupportsShowCompatibilityAlias(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	path := dir + "/show.txt"
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-read","arguments":{"file":"` + path + `"}}}`
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve show: %v", err)
	}
	scanner := bufio.NewScanner(&out)
	if !scanner.Scan() {
		t.Fatalf("no show response")
	}
	var resp map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("show json: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing show result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected show error response: %#v", result)
	}
}

func TestToolCallSupportsInsertAliasAndWriteDirectContent(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	insertPath := dir + "/insert.txt"
	writePath := dir + "/write.txt"
	if err := os.WriteFile(insertPath, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write insert file: %v", err)
	}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-insert","arguments":{"file":"` + insertPath + `:1","content":"x","preview":false}}}`
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve insert: %v", err)
	}
	scanner := bufio.NewScanner(&out)
	if !scanner.Scan() {
		t.Fatalf("no insert response")
	}
	var resp map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("insert json: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing insert result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected insert error response: %#v", result)
	}
	if got := mustReadFile(t, insertPath); got != "a\nx\nb\n" {
		t.Fatalf("unexpected insert file content: %q", got)
	}

	out.Reset()
	req = `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"be-write","arguments":{"file":"` + writePath + `","content":"hello"}}}`
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve write: %v", err)
	}
	scanner = bufio.NewScanner(&out)
	if !scanner.Scan() {
		t.Fatalf("no write response")
	}
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("write json: %v", err)
	}
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing write result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected write error response: %#v", result)
	}
	if got := mustReadFile(t, writePath); got != "hello" {
		t.Fatalf("unexpected write file content: %q", got)
	}
}

func TestToolCallSupportsDeleteAliasesAndReplaceOld(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	deletePath := dir + "/delete.txt"
	replacePath := dir + "/replace.txt"
	if err := os.WriteFile(deletePath, []byte("1\n2\n3\n4\n"), 0o644); err != nil {
		t.Fatalf("write delete file: %v", err)
	}
	if err := os.WriteFile(replacePath, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("write replace file: %v", err)
	}

	var out bytes.Buffer
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-delete","arguments":{"file":"` + deletePath + `:2-3","preview":false}}}`
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve delete: %v", err)
	}
	scanner := bufio.NewScanner(&out)
	if !scanner.Scan() {
		t.Fatalf("no delete response")
	}
	var resp map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("delete json: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing delete result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected delete error response: %#v", result)
	}
	if got := mustReadFile(t, deletePath); got != "1\n4\n" {
		t.Fatalf("unexpected delete file content: %q", got)
	}

	out.Reset()
	req = `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"be-replace","arguments":{"file":"` + replacePath + `:2","old":"b\n","content":"x\n","preview":false}}}`
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve replace: %v", err)
	}
	scanner = bufio.NewScanner(&out)
	if !scanner.Scan() {
		t.Fatalf("no replace response")
	}
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("replace json: %v", err)
	}
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing replace result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected replace error response: %#v", result)
	}
	if got := mustReadFile(t, replacePath); got != "a\nx\nc\n" {
		t.Fatalf("unexpected replace file content: %q", got)
	}
}

func TestToolCall_DeleteAll_ClearsFileAndSavesChip(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	path := dir + "/delete_all.txt"
	if err := os.WriteFile(path, []byte("1\n2\n3\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-delete","arguments":{"file":"` + path + `:ALL","preview":false}}}`
	out := callServer(t, srv, req)
	text := mustTextResult(t, out)
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if got := mustReadFile(t, path); got != "" {
		t.Fatalf("expected empty file after :ALL delete, got %q", got)
	}

	warnings, _ := result["warnings"].([]any)
	hasChipWarning := false
	for _, w := range warnings {
		if s, ok := w.(string); ok && strings.HasPrefix(s, "deleted content saved as chip://") {
			hasChipWarning = true
			break
		}
	}
	if !hasChipWarning {
		t.Fatalf("expected deleted content saved as chip warning, got warnings: %v", warnings)
	}
}

func TestToolCall_DeleteRange_SavesChip(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	path := dir + "/delete_range.txt"
	if err := os.WriteFile(path, []byte("1\n2\n3\n4\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-delete","arguments":{"file":"` + path + `:2-3","preview":false}}}`
	out := callServer(t, srv, req)
	text := mustTextResult(t, out)
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if got := mustReadFile(t, path); got != "1\n4\n" {
		t.Fatalf("expected remaining content \"1\\n4\\n\", got %q", got)
	}

	warnings, _ := result["warnings"].([]any)
	hasChipWarning := false
	for _, w := range warnings {
		if s, ok := w.(string); ok && strings.HasPrefix(s, "deleted content saved as chip://") {
			hasChipWarning = true
			break
		}
	}
	if !hasChipWarning {
		t.Fatalf("expected deleted content saved as chip warning, got warnings: %v", warnings)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(data)
}

// ──────────────────────────────────────────────
// MCP 集成测试：错误响应 + 结构校验
// ──────────────────────────────────────────────

func TestToolCall_MissingFile_ReturnsError(t *testing.T) {
	srv := New("en", false)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-read","arguments":{"file":""}}}`
	out := callServer(t, srv, req)
	resp := parseResp(t, out)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); !isError {
		t.Fatal("expected isError=true for missing file param")
	}
}

func TestToolCall_UnknownTool_ReturnsError(t *testing.T) {
	srv := New("en", false)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-nonexistent","arguments":{}}}`
	out := callServer(t, srv, req)
	resp := parseResp(t, out)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); !isError {
		t.Fatal("expected isError=true for unknown tool")
	}
}

func TestToolCall_ReadBrief_HasEmptyContent(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	path := dir + "/brief.txt"
	os.WriteFile(path, []byte("a\nb\n"), 0o644)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-read","arguments":{"file":"` + path + `:1-2","brief":true}}}`
	out := callServer(t, srv, req)
	var result map[string]any
	if err := json.Unmarshal([]byte(mustTextResult(t, out)), &result); err != nil {
		t.Fatalf("json result: %v", err)
	}
	if result["brief"] != true {
		t.Fatal("expected brief=true")
	}
	if content, _ := result["content"].(string); content != "" {
		t.Fatalf("expected empty content in brief mode, got %q", content)
	}
}

func TestToolCall_ReplacePreview_FileUnchanged(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	path := dir + "/preview.txt"
	os.WriteFile(path, []byte("keep\n"), 0o644)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-replace","arguments":{"file":"` + path + `:1","content":"changed\n","preview":true}}}`
	out := callServer(t, srv, req)
	mustSuccess(t, out)
	got := mustReadFile(t, path)
	if got != "keep\n" {
		t.Fatalf("expected file unchanged in preview, got %q", got)
	}
}

func TestToolCall_BalanceVerbose_HasMatchedPairs(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	path := dir + "/balanced.js"
	os.WriteFile(path, []byte("function a() {}\n"), 0o644)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-balance","arguments":{"file":"` + path + `","verbose":true}}}`
	out := callServer(t, srv, req)
	var result map[string]any
	if err := json.Unmarshal([]byte(mustTextResult(t, out)), &result); err != nil {
		t.Fatalf("json: %v", err)
	}
	if _, ok := result["matched"]; !ok {
		t.Fatalf("expected 'matched' in verbose balance, got keys: %v", keysOf(result))
	}
}

func TestToolCall_FuncRange_ReturnsRange(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	path := dir + "/demofunc.go"
	os.WriteFile(path, []byte("package main\n\nfunc demo() {\n\tx := 1\n}\n"), 0o644)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-func-range","arguments":{"file":"` + path + `","line":3}}}`
	out := callServer(t, srv, req)
	var result map[string]any
	if err := json.Unmarshal([]byte(mustTextResult(t, out)), &result); err != nil {
		t.Fatalf("json: %v", err)
	}
	if start, _ := result["start"].(float64); start != 3 {
		t.Fatalf("expected start=3, got %v", result["start"])
	}
	if end, _ := result["end"].(float64); end != 5 {
		t.Fatalf("expected end=5, got %v", result["end"])
	}
}

func TestToolCall_TagRange_ReturnsTag(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	path := dir + "/tagtest.html"
	os.WriteFile(path, []byte("<div>\n<p>text</p>\n</div>\n"), 0o644)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-tag-range","arguments":{"file":"` + path + `","line":2}}}`
	out := callServer(t, srv, req)
	var result map[string]any
	if err := json.Unmarshal([]byte(mustTextResult(t, out)), &result); err != nil {
		t.Fatalf("json: %v", err)
	}
	if result["tag"] != "p" {
		t.Fatalf("expected tag 'p', got %q", result["tag"])
	}
}

func TestToolCall_InsertChip_EmptyFrom_ReturnsChipList(t *testing.T) {
	srv := New("en", false)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-insert-chip","arguments":{}}}`
	out := callServer(t, srv, req)
	var result map[string]any
	if err := json.Unmarshal([]byte(mustTextResult(t, out)), &result); err != nil {
		t.Fatalf("json: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
}

func TestToolCall_TrxStatus_ReturnsQueue(t *testing.T) {
	srv := New("en", false)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-trx-status","arguments":{}}}`
	out := callServer(t, srv, req)
	var result map[string]any
	if err := json.Unmarshal([]byte(mustTextResult(t, out)), &result); err != nil {
		t.Fatalf("json: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
	if _, ok := result["queue"]; !ok {
		t.Fatalf("expected 'queue' in trx-status")
	}
}

func TestToolCall_Write_FileContent_ReturnsResult(t *testing.T) {
	srv := New("en", false)
	dir := t.TempDir()
	p1 := dir + "/a.txt"
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-write","arguments":{"file":"` + p1 + `","content":"hello"}}}`
	out := callServer(t, srv, req)
	var result map[string]any
	if err := json.Unmarshal([]byte(mustTextResult(t, out)), &result); err != nil {
		t.Fatalf("json: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
	if files, _ := result["files"].(float64); files != 1 {
		t.Fatalf("expected 1 file, got %v", files)
	}
	if got := mustReadFile(t, p1); got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func callServer(t *testing.T, srv *Server, req string) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	return &out
}

func parseResp(t *testing.T, out *bytes.Buffer) map[string]any {
	t.Helper()
	scanner := bufio.NewScanner(out)
	if !scanner.Scan() {
		t.Fatalf("no response")
	}
	var resp map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	return resp
}

func mustSuccess(t *testing.T, out *bytes.Buffer) {
	t.Helper()
	resp := parseResp(t, out)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected error: %#v", result)
	}
}

func mustTextResult(t *testing.T, out *bytes.Buffer) string {
	t.Helper()
	resp := parseResp(t, out)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected error: %#v", result)
	}
	contents, ok := result["content"].([]any)
	if !ok || len(contents) == 0 {
		t.Fatalf("missing content array: %#v", result)
	}
	first, ok := contents[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] not object: %#v", contents[0])
	}
	text, _ := first["text"].(string)
	return text
}

func keysOf(m map[string]any) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// ──────────────────────────────────────────────
// --no-prefix 功能测试
// ──────────────────────────────────────────────

func TestNoPrefix_ToolNamesStripped(t *testing.T) {
	srv := New("en", true)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	scanner := bufio.NewScanner(&out)
	if !scanner.Scan() {
		t.Fatalf("no response")
	}
	var resp map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %#v", resp)
	}
	tools := result["tools"].([]any)
	for _, item := range tools {
		m := item.(map[string]any)
		name := m["name"].(string)
		if strings.HasPrefix(name, "be-") {
			t.Fatalf("expected no be- prefix with --no-prefix, got %q", name)
		}
	}
}

func TestNoPrefix_ToolCallWorks(t *testing.T) {
	srv := New("en", true)
	dir := t.TempDir()
	path := dir + "/noprefix.txt"
	os.WriteFile(path, []byte("a\nb\n"), 0o644)
	// Call with unprefixed name "read" instead of "be-read"
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read","arguments":{"file":"` + path + `","start":1,"end":"auto"}}}`
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	scanner := bufio.NewScanner(&out)
	if !scanner.Scan() {
		t.Fatalf("no response")
	}
	var resp map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %#v", resp)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("unexpected error: %#v", result)
	}
}

func TestNoPrefix_False_KeepsBePrefix(t *testing.T) {
	srv := New("en", false)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(req+"\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	scanner := bufio.NewScanner(&out)
	if !scanner.Scan() {
		t.Fatalf("no response")
	}
	var resp map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	result := resp["result"].(map[string]any)
	tools := result["tools"].([]any)
	found := false
	for _, item := range tools {
		m := item.(map[string]any)
		name := m["name"].(string)
		if name == "be-read" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected be-read tool with prefix enabled")
	}
}
