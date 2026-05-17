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
	srv := New(app.LangFromEnv())
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
	srv := New("en")
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no output, got %s", out.String())
	}
}

func TestToolCallRunsEdit(t *testing.T) {
	srv := New("en")
	dir := t.TempDir()
	path := dir + "/a.txt"
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-insert","arguments":{"file":"` + path + `","after_line":0,"content":"x","raw":true,"preview":false}}}`
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
	srv := New("en")
	dir := t.TempDir()
	path := dir + "/read.txt"
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-read","arguments":{"file":"` + path + `","start":1,"end":"auto"}}}`
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
	srv := New("en")
	dir := t.TempDir()
	path := dir + "/show.txt"
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-read","arguments":{"file":"` + path + `","start":1,"end":"auto"}}}`
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
	srv := New("en")
	dir := t.TempDir()
	insertPath := dir + "/insert.txt"
	writePath := dir + "/write.txt"
	if err := os.WriteFile(insertPath, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write insert file: %v", err)
	}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-insert","arguments":{"file":"` + insertPath + `","after_line":1,"content":"x","raw":true,"preview":false}}}`
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
	srv := New("en")
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
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-delete","arguments":{"file":"` + deletePath + `","start":2,"end":3,"preview":false}}}`
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
	req = `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"be-replace","arguments":{"file":"` + replacePath + `","start":2,"end":2,"old":"b\n","content":"x\n","preview":false}}}`
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

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(data)
}
