package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/conglinyizhi/better-edit-tools-mcp/internal/app"
	betools "github.com/conglinyizhi/better-edit-tools-mcp/pkg/betools"
)

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

type Server struct {
	lang string
	opts []betools.Option
}

func Run(cfg app.Config) error {
	return New(cfg.Lang).Serve(os.Stdin, os.Stdout)
}

func New(lang string, opts ...betools.Option) *Server {
	return &Server{lang: lang, opts: opts}
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
		// Save a chip on error if the args were substantial
		betools.SaveChip(params.Name, args, err.Error())
		return s.ok(req.ID, map[string]any{
			"isError": true,
			"content": []map[string]any{{
				"type": "text",
				"text": err.Error(),
			}},
		})
	}
	// Parse the JSON string back into a map for structured response
	var structured map[string]any
	if json.Unmarshal([]byte(out), &structured) == nil {
		return s.ok(req.ID, map[string]any{
			"content": []map[string]any{{
				"type":  "text",
				"text":  out,
				"_data": structured,
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
		s.readTool("be-read", localizedDescription(s.lang, "be-read")),
		{
			Name:        "be-replace",
			Description: localizedDescription(s.lang, "be-replace"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":    map[string]any{"type": "string"},
					"start":   map[string]any{"type": "integer", "minimum": 1},
					"end":     map[string]any{"type": "integer", "minimum": 1},
					"old":     map[string]any{"type": "string"},
					"content": map[string]any{"type": "string"},
					"raw":     map[string]any{"type": "boolean", "description": "set to true when your content has literal \\n (visible backslash-n) that needs to become real newlines; normally keep false (standard JSON escaping)"},
					"format":  map[string]any{"type": "string"},
					"preview": map[string]any{"type": "boolean"},
					"brief":   map[string]any{"type": "boolean", "description": "return minimal response (omit diff)"},
					"target": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"kind":  map[string]any{"type": "string", "enum": []string{"line", "function", "marker", "tag"}},
							"value": map[string]any{"type": "string"},
						},
					},
				},
				"required": []string{"file", "start", "end"},
			},
		},
		{
			Name:        "be-insert",
			Description: localizedDescription(s.lang, "be-insert"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":       map[string]any{"type": "string"},
					"line":       map[string]any{"type": "integer", "minimum": 0},
					"after_line": map[string]any{"type": "integer", "minimum": 0},
					"content":    map[string]any{"type": "string"},
					"raw":        map[string]any{"type": "boolean", "description": "set to true when your content has literal \\n (visible backslash-n) that needs to become real newlines; normally keep false (standard JSON escaping)"},
					"format":     map[string]any{"type": "string"},
					"preview":    map[string]any{"type": "boolean"},
					"brief":      map[string]any{"type": "boolean", "description": "return minimal response (omit diff)"},
					"target": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"kind":  map[string]any{"type": "string", "enum": []string{"line", "function", "marker", "tag"}},
							"value": map[string]any{"type": "string"},
						},
					},
				},
				"required": []string{"file", "line", "content"},
			},
		},
		{
			Name:        "be-delete",
			Description: localizedDescription(s.lang, "be-delete"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":       map[string]any{"type": "string"},
					"start":      map[string]any{"type": "integer", "minimum": 1},
					"start_line": map[string]any{"type": "integer", "minimum": 1},
					"end":        map[string]any{"type": "integer", "minimum": 1},
					"end_line":   map[string]any{"type": "integer", "minimum": 1},
					"line":       map[string]any{"type": "integer", "minimum": 1},
					"lines":      map[string]any{"type": "string"},
					"format":     map[string]any{"type": "string"},
					"preview":    map[string]any{"type": "boolean"},
					"brief":      map[string]any{"type": "boolean", "description": "return minimal response (omit diff)"},
					"target": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"kind":  map[string]any{"type": "string", "enum": []string{"line", "function", "marker", "tag"}},
							"value": map[string]any{"type": "string"},
						},
					},
				},
				"required": []string{"file"},
			},
		},
		{
			Name:        "be-batch",
			Description: localizedDescription(s.lang, "be-batch"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"spec": map[string]any{
						"type":        "string",
						"description": "JSON spec describing edits. Format: [{\"file\": \"path\", \"edits\": [{\"action\": \"replace-lines|insert-after|delete-lines\", \"start\": N, \"end\": N, \"content\": \"...\"}]}]",
					},
					"preview": map[string]any{"type": "boolean"},
					"brief":   map[string]any{"type": "boolean", "description": "return minimal response"},
				},
				"required": []string{"spec"},
			},
		},
		{
			Name:        "be-write",
			Description: localizedDescription(s.lang, "be-write"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":    map[string]any{"type": "string"},
					"content": map[string]any{"type": "string"},
					"files": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"file":    map[string]any{"type": "string"},
								"content": map[string]any{"type": "string"},
							},
							"required": []string{"file", "content"},
						},
					},
					"preview": map[string]any{"type": "boolean"},
					"brief":   map[string]any{"type": "boolean", "description": "return minimal response (omit per-file details)"},
					"raw":     map[string]any{"type": "boolean", "description": "set to true when your content has literal \\n (visible backslash-n) that needs to become real newlines; normally keep false (standard JSON escaping)"},
				},
			},
		},
		{
			Name:        "be-func-range",
			Description: localizedDescription(s.lang, "be-func-range"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file": map[string]any{"type": "string"},
					"line": map[string]any{"type": "integer", "minimum": 1},
				},
				"required": []string{"file", "line"},
			},
		},
		{
			Name:        "be-tag-range",
			Description: localizedDescription(s.lang, "be-tag-range"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file": map[string]any{"type": "string"},
					"line": map[string]any{"type": "integer", "minimum": 1},
				},
				"required": []string{"file", "line"},
			},
		},
		{
			Name:        "be-balance",
			Description: localizedDescription(s.lang, "be-balance"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":    map[string]any{"type": "string"},
					"verbose": map[string]any{"type": "boolean", "description": "false: show only unbalanced items; true: full report including matched pairs"},
				},
			},
		},
		{
			Name:        "be-insert-chip",
			Description: localizedDescription(s.lang, "be-insert-chip"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"from": map[string]any{
						"type":        "string",
						"description": "Source: file:///absolute/path or chip://{id}. Empty/omit to list all chip IDs.",
					},
					"to": map[string]any{
						"type":        "string",
						"description": "Target: file:///absolute/path:line_number",
					},
				},
			},
		},
		{
			Name:        "be-trx-commit",
			Description: localizedDescription(s.lang, "be-trx-commit"),
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "be-trx-rollback",
			Description: localizedDescription(s.lang, "be-trx-rollback"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"step": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"default":     1,
						"description": "number of snapshots to roll back from most recent",
					},
				},
				"required": []string{"step"},
			},
		},
		{
			Name:        "be-trx-status",
			Description: localizedDescription(s.lang, "be-trx-status"),
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
	return specs
}

func (s *Server) readTool(name, description string) Tool {
	return Tool{
		Name:        name,
		Description: description,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file":    map[string]any{"type": "string"},
				"start":   map[string]any{"type": "integer", "minimum": 1},
				"end":     map[string]any{"oneOf": []any{map[string]any{"type": "integer"}, map[string]any{"type": "string", "enum": []string{"auto"}}}},
				"preview": map[string]any{"type": "boolean"},
				"brief":   map[string]any{"type": "boolean", "description": "return only metadata, no content"},
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
	}
}

func localizedDescription(lang, name string) string {
	zh := map[string]string{
		"be-read":         "按行号读取文件内容。",
		"be-replace":      "替换文件中的指定行范围。",
		"be-insert":       "在指定行后插入内容。",
		"be-delete":       "删除单行、范围或批量指定的行号。",
		"be-batch":        "一次执行多处编辑，支持多文件。",
		"be-write":        "直接写入文件内容，JSON 失败时会尽量降级解析。",
		"be-func-range":   "定位某一行所在的函数或花括号范围。",
		"be-tag-range":    "定位某一行所在的 XML/HTML/Vue 标签配对范围。",
		"be-balance":      "检查括号、标签和引号的配对情况。",
		"be-insert-chip":  "插入内容，支持从文件（file://）或 chip 缓存（chip://）读取。不传 from 参数时列出所有 chip ID。",
		"be-trx-commit":   "提交所有待确认的编辑快照（清空队列）。",
		"be-trx-rollback": "回滚最近 N 个编辑快照。step 从 1 开始，表示回滚最近几个操作。",
		"be-trx-status":   "查看待处理的编辑快照及队列使用情况。",
	}
	en := map[string]string{
		"be-read":         "Read file content with line numbers.",
		"be-replace":      "Replace a precise line range in a file.",
		"be-insert":       "Insert content after a specific line.",
		"be-delete":       "Delete one line, a line range, or a batch of line numbers.",
		"be-batch":        "Apply multiple edits in one call, including multi-file edits.",
		"be-write":        "Write raw content to file(s), with a degraded parser for malformed JSON payloads.",
		"be-func-range":   "Find the enclosing function or brace block for a given line.",
		"be-tag-range":    "Find the enclosing XML/HTML/Vue tag pair for a given line.",
		"be-balance":      "Check bracket, brace, parenthesis, tag, and quote balance.",
		"be-insert-chip":  "Insert content from a file (file://) or chip cache (chip://). Omit from to list all chip IDs.",
		"be-trx-commit":   "Commit all pending edit snapshots (clear the queue).",
		"be-trx-rollback": "Roll back the last N edit snapshots from the queue.",
		"be-trx-status":   "Show pending edit snapshots and queue usage.",
	}
	if lang == "zh" {
		return zh[name]
	}
	return en[name]
}

func (s *Server) callTool(name string, args map[string]any) (string, error) {
	b, _ := json.Marshal(args)
	switch name {
	case "be-read":
		var p struct {
			File    string         `json:"file"`
			Start   int            `json:"start"`
			End     any            `json:"end"`
			Target  *editTargetArg `json:"target"`
			Preview bool           `json:"preview"`
			Brief   bool           `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		start := p.Start
		if start < 1 {
			start = 1
		}
		var endLine int
		if p.Target != nil {
			span, err := betools.ResolveTargetSpan(p.File, p.Target.toContentTarget(), s.opts...)
			if err != nil {
				return "", err
			}
			start = span.Start
			endLine = span.End
		} else {
			switch v := p.End.(type) {
			case string:
				if v == "auto" {
					endLine = -1
				}
			case float64:
				endLine = int(v)
			}
		}
		res, sessionID, err := betools.Read(p.File, start, endLine, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		resultMap := mustJSONToMap(res)
		resultMap["viewed_code_id"] = sessionID
		return mustJSON(resultMap), nil
	case "be-replace":
		var p struct {
			File         string         `json:"file"`
			Start        int            `json:"start"`
			End          int            `json:"end"`
			StartLn      int            `json:"start_line"`
			EndLn        int            `json:"end_line"`
			Old          *string        `json:"old"`
			OldText      *string        `json:"old_text"`
			Content      string         `json:"content"`
			Raw          bool           `json:"raw"`
			Format       string         `json:"format"`
			Target       *editTargetArg `json:"target"`
			Preview      bool           `json:"preview"`
			ViewedCodeID string         `json:"viewed_code_id"`
			Brief        bool           `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		start, end := p.Start, p.End
		if p.StartLn != 0 {
			start = p.StartLn
		}
		if p.EndLn != 0 {
			end = p.EndLn
		}
		if p.Target != nil {
			span, err := betools.ResolveTargetSpan(p.File, p.Target.toContentTarget(), s.opts...)
			if err != nil {
				return "", err
			}
			start, end = span.Start, span.End
		}
		old := p.Old
		if old == nil {
			old = p.OldText
		}
		res, err := betools.Replace(p.File, start, end, old, p.Content, p.Raw, defaultFormat(p.Format), p.Preview, p.ViewedCodeID, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-insert":
		var p struct {
			File    string         `json:"file"`
			Line    int            `json:"line"`
			After   *int           `json:"after_line"`
			Content string         `json:"content"`
			Raw     bool           `json:"raw"`
			Format  string         `json:"format"`
			Target  *editTargetArg `json:"target"`
			Preview bool           `json:"preview"`
			Brief   bool           `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		after := p.Line
		if p.After != nil {
			after = *p.After
		} else if p.Line > 0 {
			// line=N inserts BEFORE line N (user-intuitive: "at line N")
			after = p.Line - 1
		}
		if p.Target != nil {
			span, err := betools.ResolveTargetSpan(p.File, p.Target.toContentTarget(), s.opts...)
			if err != nil {
				return "", err
			}
			after = span.End
		}
		res, err := betools.Insert(p.File, after, p.Content, p.Raw, defaultFormat(p.Format), p.Preview, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-delete":
		var p struct {
			File    string         `json:"file"`
			Start   *int           `json:"start"`
			End     *int           `json:"end"`
			StartLn *int           `json:"start_line"`
			EndLn   *int           `json:"end_line"`
			Line    *int           `json:"line"`
			Lines   *string        `json:"lines"`
			Format  string         `json:"format"`
			Target  *editTargetArg `json:"target"`
			Preview bool           `json:"preview"`
			Brief   bool           `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		var start, end, line int
		if p.Start != nil {
			start = *p.Start
		}
		if p.StartLn != nil {
			start = *p.StartLn
		}
		if p.End != nil {
			end = *p.End
		}
		if p.EndLn != nil {
			end = *p.EndLn
		}
		if p.Line != nil {
			line = *p.Line
		}
		if p.Target != nil {
			span, err := betools.ResolveTargetSpan(p.File, p.Target.toContentTarget(), s.opts...)
			if err != nil {
				return "", err
			}
			start, end = span.Start, span.End
		}
		res, err := betools.Delete(p.File, start, end, line, p.Lines, defaultFormat(p.Format), p.Preview, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-batch":
		var p struct {
			Spec    string `json:"spec"`
			Preview bool   `json:"preview"`
			Brief   bool   `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		res, err := betools.Batch(p.Spec, p.Preview, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-write":
		var p struct {
			File    string `json:"file"`
			Content string `json:"content"`
			Spec    string `json:"spec"`
			Files   []struct {
				File    string `json:"file"`
				Content string `json:"content"`
			} `json:"files"`
			Preview bool `json:"preview"`
			Raw     bool `json:"raw"`
			Brief   bool `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		var spec string
		switch {
		case p.Spec != "":
			spec = p.Spec
		case p.Content != "" && strings.HasPrefix(p.Content, "{"):
			spec = p.Content
		case p.File != "" || p.Content != "":
			spec = mustJSON(map[string]any{
				"file":    p.File,
				"content": p.Content,
			})
		case len(p.Files) > 0:
			files := make([]map[string]any, 0, len(p.Files))
			for _, item := range p.Files {
				files = append(files, map[string]any{
					"file":    item.File,
					"content": item.Content,
				})
			}
			spec = mustJSON(map[string]any{"files": files})
		}
		res, err := betools.Write(spec, p.Preview, p.Raw, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		// degraded writes auto-save to chip — content is incomplete/unreliable
		if res.Degraded {
			args := map[string]any{
				"tool":    "be-write",
				"spec":    spec,
				"preview": p.Preview,
			}
			betools.SaveChip("be-write", args, "")
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
		res, err := betools.FuncRange(p.File, p.Line, s.opts...)
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
		res, err := betools.TagRange(p.File, p.Line, s.opts...)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-balance":
		var p struct {
			File    string `json:"file"`
			Verbose bool   `json:"verbose"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		return betools.CheckStructureBalance(p.File, p.Verbose, s.opts...)
	case "be-insert-chip":
		var p struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}

		// If no from/to provided, list chips
		if p.From == "" && p.To == "" {
			ids := betools.ListChips()
			if len(ids) == 0 {
				return mustJSON(map[string]any{"status": "ok", "chips": []string{}, "message": "no chips recorded"}), nil
			}
			return mustJSON(map[string]any{"status": "ok", "chips": ids}), nil
		}

		// Resolve from: file:// or chip://
		var content string
		var readErr error
		var fileBytes []byte
		switch {
		case strings.HasPrefix(p.From, "file://"):
			fileBytes, readErr = os.ReadFile(strings.TrimPrefix(p.From, "file://"))
			if readErr != nil {
				return "", readErr
			}
			content = string(fileBytes)
		case strings.HasPrefix(p.From, "chip://"):
			id := strings.TrimPrefix(p.From, "chip://")
			rec, err := betools.GetChip(id)
			if err != nil {
				return "", err
			}
			argsJSON, _ := json.MarshalIndent(rec.Args, "", "  ")
			content = fmt.Sprintf("// Chip %s from tool %q\n// Original arguments:\n%s", rec.ID, rec.Tool, string(argsJSON))
		default:
			return "", fmt.Errorf("invalid from format: use file:///path or chip://{id}")
		}

		// Resolve to: file:///path:line
		if !strings.HasPrefix(p.To, "file://") {
			return "", fmt.Errorf("invalid to format: use file:///absolute/path:line_number")
		}
		rest := strings.TrimPrefix(p.To, "file://")
		colonIdx := strings.LastIndex(rest, ":")
		if colonIdx < 0 {
			return "", fmt.Errorf("invalid to format: missing :line_number after file:///path")
		}
		targetFile := rest[:colonIdx]
		lineStr := rest[colonIdx+1:]
		lineNum, convErr := strconv.Atoi(lineStr)
		if convErr != nil {
			return "", fmt.Errorf("invalid line number: %s", lineStr)
		}

		// Use the core betools.Insert with resolved content
		res, err := betools.Insert(targetFile, lineNum, content, true, "plain", false, false, s.opts...)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-trx-commit":
		n := betools.CommitSnapshots()
		return mustJSON(map[string]any{
			"status":    "ok",
			"committed": n,
		}), nil
	case "be-trx-rollback":
		var p struct {
			Step int `json:"step"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		if p.Step < 1 {
			p.Step = 1
		}
		count, errs := betools.RollbackSnapshots(p.Step)
		if len(errs) > 0 {
			errStrs := make([]string, len(errs))
			for i, e := range errs {
				errStrs[i] = e.Error()
			}
			status := "ok"
			if count < p.Step {
				status = "partial"
			}
			return mustJSON(map[string]any{
				"status":      status,
				"rolled_back": count,
				"errors":      errStrs,
			}), nil
		}
		return mustJSON(map[string]any{
			"status":      "ok",
			"rolled_back": count,
		}), nil
	case "be-trx-status":
		stats := betools.SnapshotQueueStats()
		pending := betools.ListSnapshots()
		return mustJSON(map[string]any{
			"status":    "ok",
			"queue":     stats,
			"snapshots": pending,
		}), nil
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
		return fmt.Sprintf("json.Marshal: %v", err)
	}
	return string(data)
}

func mustJSONToMap(v any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		return map[string]any{"__error": err.Error()}
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{"__error": err.Error()}
	}
	return m
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

func (t *editTargetArg) toContentTarget() betools.ContentTarget {
	if t == nil {
		return betools.ContentTarget{}
	}
	return betools.ContentTarget{Kind: t.Kind, Value: t.Value}
}
