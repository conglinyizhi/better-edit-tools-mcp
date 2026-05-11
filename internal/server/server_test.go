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
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"be-insert","arguments":{"file":"` + path + `","line":1,"content":"x","raw":true,"preview":false}}}`
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
	if string(data) != "a\nx\nb\n" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}
