package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/conglinyizhi/better-edit-tools-mcp/internal/app"
	"github.com/conglinyizhi/better-edit-tools-mcp/internal/edit"
)

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

type Server struct {
	lang string
}

func Run(cfg app.Config) error {
	return New(cfg.Lang).Serve(os.Stdin, os.Stdout)
}

func New(lang string) *Server {
	return &Server{lang: lang}
}

func (s *Server) Serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		resp, emit := s.Handle(req)
		if !emit {
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	ClientInfo      map[string]any `json:"clientInfo,omitempty"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]any         `json:"arguments,omitempty"`
	ArgMap    map[string]any         `json:"args,omitempty"`
	Raw       map[string]interface{} `json:"-"`
}

func (s *Server) Handle(req rpcRequest) (rpcResponse, bool) {
	switch req.Method {
	case "initialize":
		return s.ok(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]any{
				"name":    app.Name,
				"version": app.Version,
			},
			"capabilities": map[string]any{
				"tools": map[string]any{
					"listChanged": false,
				},
			},
		}), true
	case "notifications/initialized":
		return rpcResponse{}, false
	case "tools/list":
		return s.ok(req.ID, map[string]any{
			"tools": s.listTools(),
		}), true
	case "tools/call":
		return s.handleToolCall(req), true
	default:
		return s.err(req.ID, -32601, "method not found"), true
	}
}

func (s *Server) handleToolCall(req rpcRequest) rpcResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.err(req.ID, -32602, err.Error())
	}
	args := params.Arguments
	if args == nil {
		args = params.ArgMap
	}
	out, err := s.callTool(params.Name, args)
	if err != nil {
		return s.ok(req.ID, map[string]any{
			"isError": true,
			"content": []map[string]any{{
				"type": "text",
				"text": err.Error(),
			}},
		})
	}
	return s.ok(req.ID, map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": out,
		}},
	})
}

func (s *Server) listTools() []Tool {
	specs := []Tool{
		{
			Name:        "be-show",
			Description: localizedDescription(s.lang, "be-show"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":    map[string]any{"type": "string"},
					"start":   map[string]any{"type": "integer", "minimum": 1},
					"end":     map[string]any{"oneOf": []any{map[string]any{"type": "integer"}, map[string]any{"type": "string", "enum": []string{"auto"}}}},
					"preview": map[string]any{"type": "boolean"},
					"target": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"kind":  map[string]any{"type": "string", "enum": []string{"line", "function", "marker", "tag"}},
							"value": map[string]any{"type": "string"},
						},
					},
				},
				"required": []string{"file", "start"},
			},
		},
		{
			Name:        "be-replace",
			Description: localizedDescription(s.lang, "be-replace"),
			InputSchema: map[string]any{"type": "object"},
		},
		{
			Name:        "be-insert",
			Description: localizedDescription(s.lang, "be-insert"),
			InputSchema: map[string]any{"type": "object"},
		},
		{
			Name:        "be-delete",
			Description: localizedDescription(s.lang, "be-delete"),
			InputSchema: map[string]any{"type": "object"},
		},
		{
			Name:        "be-batch",
			Description: localizedDescription(s.lang, "be-batch"),
			InputSchema: map[string]any{"type": "object"},
		},
		{
			Name:        "be-write",
			Description: localizedDescription(s.lang, "be-write"),
			InputSchema: map[string]any{"type": "object"},
		},
		{
			Name:        "be-func-range",
			Description: localizedDescription(s.lang, "be-func-range"),
			InputSchema: map[string]any{"type": "object"},
		},
		{
			Name:        "be-tag-range",
			Description: localizedDescription(s.lang, "be-tag-range"),
			InputSchema: map[string]any{"type": "object"},
		},
		{
			Name:        "be-balance",
			Description: localizedDescription(s.lang, "be-balance"),
			InputSchema: map[string]any{"type": "object"},
		},
	}
	return specs
}

func localizedDescription(lang, name string) string {
	zh := map[string]string{
		"be-show":       "按行号展示文件内容。",
		"be-replace":    "替换文件中的指定行范围。",
		"be-insert":     "在指定行后插入内容。",
		"be-delete":     "删除单行、范围或批量指定的行号。",
		"be-batch":      "一次执行多处编辑，支持多文件。",
		"be-write":      "直接写入文件内容，JSON 失败时会尽量降级解析。",
		"be-func-range": "定位某一行所在的函数或花括号范围。",
		"be-tag-range":  "定位某一行所在的 XML/HTML/Vue 标签配对范围。",
		"be-balance":    "检查括号、标签和引号的配对情况。",
	}
	en := map[string]string{
		"be-show":       "Display file content with line numbers.",
		"be-replace":    "Replace a precise line range in a file.",
		"be-insert":     "Insert content after a specific line.",
		"be-delete":     "Delete one line, a line range, or a batch of line numbers.",
		"be-batch":      "Apply multiple edits in one call, including multi-file edits.",
		"be-write":      "Write raw content to file(s), with a degraded parser for malformed JSON payloads.",
		"be-func-range": "Find the enclosing function or brace block for a given line.",
		"be-tag-range":  "Find the enclosing XML/HTML/Vue tag pair for a given line.",
		"be-balance":    "Check bracket, brace, parenthesis, tag, and quote balance.",
	}
	if lang == "zh" {
		return zh[name]
	}
	return en[name]
}

func (s *Server) callTool(name string, args map[string]any) (string, error) {
	b, _ := json.Marshal(args)
	switch name {
	case "be-show":
		var p struct {
			File    string         `json:"file"`
			Start   int            `json:"start"`
			End     any            `json:"end"`
			Target  *editTargetArg `json:"target"`
			Preview bool           `json:"preview"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		start := p.Start
		var end *edit.ShowEnd
		if p.Target != nil {
			span, err := edit.ResolveTargetSpan(p.File, p.Target.toContentTarget())
			if err != nil {
				return "", err
			}
			start = span.Start
			end = &edit.ShowEnd{Line: span.End}
		} else {
			switch v := p.End.(type) {
			case string:
				if v == "auto" {
					end = &edit.ShowEnd{Auto: true}
				}
			case float64:
				end = &edit.ShowEnd{Line: int(v)}
			}
		}
		res, err := edit.Show(p.File, start, end)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-replace":
		var p struct {
			File    string         `json:"file"`
			Start   int            `json:"start"`
			End     int            `json:"end"`
			Content string         `json:"content"`
			Raw     bool           `json:"raw"`
			Format  string         `json:"format"`
			Target  *editTargetArg `json:"target"`
			Preview bool           `json:"preview"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		start, end := p.Start, p.End
		if p.Target != nil {
			span, err := edit.ResolveTargetSpan(p.File, p.Target.toContentTarget())
			if err != nil {
				return "", err
			}
			start, end = span.Start, span.End
		}
		res, err := edit.Replace(p.File, start, end, p.Content, p.Raw, defaultFormat(p.Format), p.Preview)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-insert":
		var p struct {
			File    string         `json:"file"`
			Line    int            `json:"line"`
			Content string         `json:"content"`
			Raw     bool           `json:"raw"`
			Format  string         `json:"format"`
			Target  *editTargetArg `json:"target"`
			Preview bool           `json:"preview"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		after := p.Line
		if p.Target != nil {
			span, err := edit.ResolveTargetSpan(p.File, p.Target.toContentTarget())
			if err != nil {
				return "", err
			}
			after = span.End
		}
		res, err := edit.Insert(p.File, after, p.Content, p.Raw, defaultFormat(p.Format), p.Preview)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-delete":
		var p struct {
			File    string         `json:"file"`
			Start   *int           `json:"start"`
			End     *int           `json:"end"`
			Line    *int           `json:"line"`
			Lines   *string        `json:"lines"`
			Format  string         `json:"format"`
			Target  *editTargetArg `json:"target"`
			Preview bool           `json:"preview"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		var start, end, line int
		if p.Start != nil {
			start = *p.Start
		}
		if p.End != nil {
			end = *p.End
		}
		if p.Line != nil {
			line = *p.Line
		}
		if p.Target != nil {
			span, err := edit.ResolveTargetSpan(p.File, p.Target.toContentTarget())
			if err != nil {
				return "", err
			}
			start, end = span.Start, span.End
		}
		res, err := edit.Delete(p.File, start, end, line, p.Lines, defaultFormat(p.Format), p.Preview)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-batch":
		var p struct {
			Spec    string `json:"spec"`
			Preview bool   `json:"preview"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		res, err := edit.Batch(p.Spec, p.Preview)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-write":
		var p struct {
			Spec    string `json:"spec"`
			Preview bool   `json:"preview"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		res, err := edit.Write(p.Spec, p.Preview)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-func-range":
		var p struct {
			File string `json:"file"`
			Line int    `json:"line"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		res, err := edit.FuncRange(p.File, p.Line)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-tag-range":
		var p struct {
			File string `json:"file"`
			Line int    `json:"line"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		res, err := edit.TagRange(p.File, p.Line)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-balance":
		var p struct {
			File string `json:"file"`
			Mode string `json:"mode"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		mode := p.Mode
		if mode == "" {
			mode = "unbalanced"
		}
		return edit.CheckStructureBalance(p.File, mode)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func defaultFormat(v string) string {
	if v == "" {
		return "plain"
	}
	return v
}

func mustJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("JSON 序列化失败: %v", err)
	}
	return string(data)
}

func (s *Server) ok(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func (s *Server) err(id json.RawMessage, code int, msg string) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

type editTargetArg struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

func (t *editTargetArg) toContentTarget() edit.ContentTarget {
	if t == nil {
		return edit.ContentTarget{}
	}
	return edit.ContentTarget{Kind: t.Kind, Value: t.Value}
}
