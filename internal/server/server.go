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
					"file":           map[string]any{"type": "string", "description": "File path with line range, e.g. file.go:10 or file.go:5-15"},
					"old":            map[string]any{"type": "string", "description": localizedDescription(s.lang, "be-replace-old")},
					"content":        map[string]any{"type": "string"},
					"format":         map[string]any{"type": "string", "enum": []string{"plain", "diff"}, "description": "Diff output format; omit or use plain for default"},
					"preview":        map[string]any{"type": "boolean"},
					"brief":          map[string]any{"type": "boolean", "description": "return minimal response (omit diff)"},
					"viewed_code_id": map[string]any{"type": "string", "description": localizedDescription(s.lang, "be-replace-viewed-code-id")},
				},
				"required": []string{"file"},
			},
		},
		{
			Name:        "be-insert",
			Description: localizedDescription(s.lang, "be-insert"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":    map[string]any{"type": "string", "description": "File path with insert position, e.g. file.go:10 (insert after line 10)"},
					"content": map[string]any{"type": "string"},
					"format":  map[string]any{"type": "string", "enum": []string{"plain", "diff"}, "description": "Diff output format; omit or use plain for default"},
					"preview": map[string]any{"type": "boolean"},
					"brief":   map[string]any{"type": "boolean", "description": "return minimal response (omit diff)"},
				},
				"required": []string{"file", "content"},
			},
		},
		{
			Name:        "be-delete",
			Description: localizedDescription(s.lang, "be-delete"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":    map[string]any{"type": "string", "description": "File path with line range, e.g. file.go:10, file.go:5-15, or file.go:ALL"},
					"format":  map[string]any{"type": "string", "enum": []string{"plain", "diff"}, "description": "Diff output format; omit or use plain for default"},
					"preview": map[string]any{"type": "boolean"},
					"brief":   map[string]any{"type": "boolean", "description": "return minimal response (omit diff)"},
				},
				"required": []string{"file"},
			},
		},
		{
			Name:        "be-write",
			Description: localizedDescription(s.lang, "be-write"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":    map[string]any{"type": "string", "description": "File path to write"},
					"content": map[string]any{"type": "string", "description": "File content"},
					"preview": map[string]any{"type": "boolean"},
					"brief":   map[string]any{"type": "boolean", "description": "return minimal response (omit per-file details)"},
				},
				"required": []string{"file", "content"},
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
				"required": []string{"file"},
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
			Name:        "be-trx",
			Description: localizedDescription(s.lang, "be-trx"),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"enum":        []string{"commit", "rollback", "status"},
						"description": localizedDescription(s.lang, "be-trx-action"),
					},
					"step": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"default":     1,
						"description": localizedDescription(s.lang, "be-trx-step"),
					},
				},
				"required": []string{"action"},
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
				"file":  map[string]any{"type": "string", "description": "File path with optional line range, e.g. file.go, file.go:23, file.go:1-3"},
				"brief": map[string]any{"type": "boolean", "description": "return only metadata, no content"},
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
			Target  *editTargetArg `json:"target"`
			Preview bool           `json:"preview"`
			Brief   bool           `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}

		// Parse file path with optional line range (e.g., file.go:23 or file.go:1-3)
		filePath, start, endLine, parseErr := betools.ParseFileRange(p.File)
		if parseErr != nil {
			return "", parseErr
		}

		if p.Target != nil {
			span, err := betools.ResolveTargetSpan(filePath, p.Target.toContentTarget(), s.opts...)
			if err != nil {
				return "", err
			}
			start = span.Start
			endLine = span.End
		}

		// If no line range specified and no target, read entire file
		if start == 0 && endLine == 0 && p.Target == nil {
			start = 1
			endLine = -1 // auto mode
		} else if start > 0 && endLine == 0 {
			// Single line specified
			endLine = start
		}

		res, sessionID, err := betools.Read(filePath, start, endLine, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		resultMap := mustJSONToMap(res)
		resultMap["viewed_code_id"] = sessionID
		return mustJSON(resultMap), nil
	case "be-replace":
		var p struct {
			File         string  `json:"file"`
			Old          *string `json:"old"`
			OldText      *string `json:"old_text"`
			Content      string  `json:"content"`
			Format       string  `json:"format"`
			Preview      bool    `json:"preview"`
			ViewedCodeID string  `json:"viewed_code_id"`
			Brief        bool    `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		if !betools.HasFileRange(p.File) {
			return "", fmt.Errorf("replace: file must include a line range, e.g. file.go:10 or file.go:5-15")
		}
		filePath, start, end, parseErr := betools.ParseFileRange(p.File)
		if parseErr != nil {
			return "", parseErr
		}
		old := p.Old
		if old == nil {
			old = p.OldText
		}
		res, err := betools.Replace(filePath, start, end, old, p.Content, defaultFormat(p.Format), p.Preview, p.ViewedCodeID, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-insert":
		var p struct {
			File    string `json:"file"`
			Content string `json:"content"`
			Format  string `json:"format"`
			Preview bool   `json:"preview"`
			Brief   bool   `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		if !betools.HasFileRange(p.File) {
			return "", fmt.Errorf("insert: file must include a line number, e.g. file.go:10")
		}
		filePath, after, endLine, parseErr := betools.ParseFileRange(p.File)
		if parseErr != nil {
			return "", parseErr
		}
		if endLine != after {
			return "", fmt.Errorf("insert: file must specify a single line, e.g. file.go:10")
		}
		res, err := betools.Insert(filePath, after, p.Content, defaultFormat(p.Format), p.Preview, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-delete":
		var p struct {
			File    string `json:"file"`
			Format  string `json:"format"`
			Preview bool   `json:"preview"`
			Brief   bool   `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		if !betools.HasFileRange(p.File) {
			return "", fmt.Errorf("delete: file must include a line range, e.g. file.go:10 or file.go:5-15")
		}
		filePath, start, end, parseErr := betools.ParseFileRange(p.File)
		if parseErr != nil {
			return "", parseErr
		}
		if start < 0 || end < 0 {
			// :ALL means delete the whole file.
			showRes, _, err := betools.Read(filePath, 1, -1, true, s.opts...)
			if err != nil {
				return "", err
			}
			start, end = 1, showRes.Total
		}
		res, err := betools.Delete(filePath, start, end, defaultFormat(p.Format), p.Preview, p.Brief, s.opts...)
		if err != nil {
			return "", err
		}
		return mustJSON(res), nil
	case "be-write":
		var p struct {
			File    string `json:"file"`
			Content string `json:"content"`
			Preview bool   `json:"preview"`
			Brief   bool   `json:"brief"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		if _, hasFile := args["file"]; !hasFile {
			return "", fmt.Errorf("write: missing file argument")
		}
		if _, hasContent := args["content"]; !hasContent {
			return "", fmt.Errorf("write: missing content argument")
		}
		if p.File == "" {
			return "", fmt.Errorf("write: file argument is empty")
		}
		spec := mustJSON(map[string]any{
			"file":    p.File,
			"content": p.Content,
		})
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
	case "be-trx":
		var p struct {
			Action string `json:"action"`
			Step   int    `json:"step"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return "", err
		}
		switch p.Action {
		case "commit":
			n := betools.CommitSnapshots()
			return mustJSON(map[string]any{
				"status":    "ok",
				"committed": n,
			}), nil
		case "rollback":
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
		case "status":
			stats := betools.SnapshotQueueStats()
			pending := betools.ListSnapshots()
			return mustJSON(map[string]any{
				"status":    "ok",
				"queue":     stats,
				"snapshots": pending,
			}), nil
		default:
			return "", fmt.Errorf("unknown trx action: %s", p.Action)
		}
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
