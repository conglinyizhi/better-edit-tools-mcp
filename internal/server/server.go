package server

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/conglinyizhi/better-edit-tools-mcp/internal/app"
	betools "github.com/conglinyizhi/better-edit-tools-mcp/pkg/betools"
)

//go:embed i18n/*.json
var i18nFS embed.FS

var i18nData = loadI18N()

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

type Server struct {
	lang     string
	opts     []betools.Option
	noPrefix bool
}

func Run(cfg app.Config) error {
	return New(cfg.Lang, cfg.NoPrefix).Serve(os.Stdin, os.Stdout)
}

func New(lang string, noPrefix bool, opts ...betools.Option) *Server {
	return &Server{lang: lang, noPrefix: noPrefix, opts: opts}
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
		chipName := params.Name
		if s.noPrefix && !strings.HasPrefix(chipName, "be-") {
			chipName = "be-" + chipName
		}
		betools.SaveChip(chipName, args, err.Error())
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
					"file":           map[string]any{"type": "string"},
					"start":          map[string]any{"type": "integer", "minimum": 1},
					"end":            map[string]any{"type": "integer", "minimum": 1},
					"old":            map[string]any{"type": "string", "description": localizedDescription(s.lang, "be-replace-old")},
					"content":        map[string]any{"type": "string"},
					"format":         map[string]any{"type": "string"},
					"preview":        map[string]any{"type": "boolean"},
					"brief":          map[string]any{"type": "boolean", "description": "return minimal response (omit diff)"},
					"viewed_code_id": map[string]any{"type": "string", "description": localizedDescription(s.lang, "be-replace-viewed-code-id")},
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
					"after_line": map[string]any{"type": "integer", "minimum": 0},
					"content":    map[string]any{"type": "string"},
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
				"required": []string{"file", "after_line", "content"},
			},
		},
		{
			Name:        "be-delete",
			Description: localizedDescription(s.lang, "be-delete"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":  map[string]any{"type": "string"},
					"start": map[string]any{"type": "integer", "minimum": 1},
					"end":   map[string]any{"type": "integer", "minimum": 1},
					"target": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"kind":  map[string]any{"type": "string", "enum": []string{"line", "function", "marker", "tag"}},
							"value": map[string]any{"type": "string"},
						},
					},
					"format":  map[string]any{"type": "string"},
					"preview": map[string]any{"type": "boolean"},
					"brief":   map[string]any{"type": "boolean", "description": "return minimal response (omit diff)"},
				},
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
	if s.noPrefix {
		for i := range specs {
			specs[i].Name = strings.TrimPrefix(specs[i].Name, "be-")
		}
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

func loadI18N() map[string]map[string]string {
	data := make(map[string]map[string]string)
	entries, err := i18nFS.ReadDir("i18n")
	if err != nil {
		return data
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		lang := strings.TrimSuffix(name, ".json")
		content, err := i18nFS.ReadFile("i18n/" + name)
		if err != nil {
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(content, &m); err != nil {
			continue
		}
		data[lang] = m
	}
	return data
}

func localizedDescription(lang, name string) string {
	if m, ok := i18nData[lang]; ok {
		if v, ok := m[name]; ok {
			return v
		}
	}
	if m, ok := i18nData["en"]; ok {
		return m[name]
	}
	return ""
}

func (s *Server) callTool(name string, args map[string]any) (string, error) {
	// When noPrefix is active, clients send unprefixed names; normalize back
	if s.noPrefix && !strings.HasPrefix(name, "be-") {
		name = "be-" + name
	}
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
		res, err := betools.Replace(p.File, start, end, old, p.Content, defaultFormat(p.Format), p.Preview, p.ViewedCodeID, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-insert":
		var p struct {
			File    string         `json:"file"`
			After   *int           `json:"after_line"`
			Content string         `json:"content"`
			Format  string         `json:"format"`
			Target  *editTargetArg `json:"target"`
			Preview bool           `json:"preview"`
			Brief   bool           `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		after := 0
		if p.After != nil {
			after = *p.After
		}
		if p.Target != nil {
			span, err := betools.ResolveTargetSpan(p.File, p.Target.toContentTarget(), s.opts...)
			if err != nil {
				return "", err
			}
			after = span.End
		}
		res, err := betools.Insert(p.File, after, p.Content, defaultFormat(p.Format), p.Preview, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-delete":
		var p struct {
			File    string         `json:"file"`
			Start   *int           `json:"start"`
			End     *int           `json:"end"`
			Format  string         `json:"format"`
			Target  *editTargetArg `json:"target"`
			Preview bool           `json:"preview"`
			Brief   bool           `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		var start, end int
		if p.Start != nil {
			start = *p.Start
		}
		if p.End != nil {
			end = *p.End
		}
		if p.Target != nil {
			span, err := betools.ResolveTargetSpan(p.File, p.Target.toContentTarget(), s.opts...)
			if err != nil {
				return "", err
			}
			start, end = span.Start, span.End
		}
		res, err := betools.Delete(p.File, start, end, defaultFormat(p.Format), p.Preview, p.Brief, s.opts...)
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
		res, err := betools.Write(spec, p.Preview, p.Brief, s.opts...)
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
		res, err := betools.Insert(targetFile, lineNum, content, "plain", false, false, s.opts...)
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
