package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	betools "github.com/conglinyizhi/better-edit-tools-mcp/pkg/betools"
)

// ToolConfig holds the parsed arguments for a single CLI tool invocation.
type ToolConfig struct {
	Name string

	// Common flags
	File    string
	Preview bool
	Brief   bool
	Output  string // "text" or "json"

	// Range/position flags
	Start   int
	End     int
	EndAuto bool
	After   int
	Line    int

	// Content/matching flags
	Content     string
	ContentFile string
	Old         string
	OldFile     string
	Format      string

	// Balance-specific
	Verbose bool

	// Write-specific
	Spec string
}

// KnownToolCommands lists the subcommands supported in CLI mode.
var KnownToolCommands = map[string]bool{
	"read":       true,
	"replace":    true,
	"insert":     true,
	"delete":     true,
	"write":      true,
	"balance":    true,
	"func-range": true,
	"tag-range":  true,
}

// ParseToolArgs parses a CLI tool invocation.
// It returns (cfg, ok). If ok is false, the caller should exit without error
// (e.g. after printing help).
func ParseToolArgs(args []string) (ToolConfig, bool) {
	if len(args) == 0 {
		return ToolConfig{}, false
	}

	name := args[0]
	if !KnownToolCommands[name] {
		return ToolConfig{}, false
	}

	cfg := ToolConfig{Name: name, Output: "text"}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help" || arg == "-h":
			printToolHelp(name)
			return ToolConfig{}, false
		case arg == "--file" || arg == "-f":
			cfg.File = nextArg(args, &i, "--file")
		case strings.HasPrefix(arg, "--file="):
			cfg.File = strings.TrimPrefix(arg, "--file=")
		case arg == "--start" || arg == "-s":
			cfg.Start = parseIntArg(args, &i, "--start")
		case strings.HasPrefix(arg, "--start="):
			cfg.Start = mustAtoi(strings.TrimPrefix(arg, "--start="), "--start")
		case arg == "--end" || arg == "-e":
			v := nextArg(args, &i, "--end")
			if v == "auto" {
				cfg.EndAuto = true
			} else {
				cfg.End = mustAtoi(v, "--end")
			}
		case strings.HasPrefix(arg, "--end="):
			v := strings.TrimPrefix(arg, "--end=")
			if v == "auto" {
				cfg.EndAuto = true
			} else {
				cfg.End = mustAtoi(v, "--end")
			}
		case arg == "--after" || arg == "--after-line":
			cfg.After = parseIntArg(args, &i, "--after-line")
		case strings.HasPrefix(arg, "--after="):
			cfg.After = mustAtoi(strings.TrimPrefix(arg, "--after="), "--after")
		case strings.HasPrefix(arg, "--after-line="):
			cfg.After = mustAtoi(strings.TrimPrefix(arg, "--after-line="), "--after-line")
		case arg == "--line" || arg == "-l":
			cfg.Line = parseIntArg(args, &i, "--line")
		case strings.HasPrefix(arg, "--line="):
			cfg.Line = mustAtoi(strings.TrimPrefix(arg, "--line="), "--line")
		case arg == "--content" || arg == "-c":
			cfg.Content = nextArg(args, &i, "--content")
		case strings.HasPrefix(arg, "--content="):
			cfg.Content = strings.TrimPrefix(arg, "--content=")
		case arg == "--content-file":
			cfg.ContentFile = nextArg(args, &i, "--content-file")
		case strings.HasPrefix(arg, "--content-file="):
			cfg.ContentFile = strings.TrimPrefix(arg, "--content-file=")
		case arg == "--old":
			cfg.Old = nextArg(args, &i, "--old")
		case strings.HasPrefix(arg, "--old="):
			cfg.Old = strings.TrimPrefix(arg, "--old=")
		case arg == "--old-file":
			cfg.OldFile = nextArg(args, &i, "--old-file")
		case strings.HasPrefix(arg, "--old-file="):
			cfg.OldFile = strings.TrimPrefix(arg, "--old-file=")
		case arg == "--format":
			cfg.Format = nextArg(args, &i, "--format")
		case strings.HasPrefix(arg, "--format="):
			cfg.Format = strings.TrimPrefix(arg, "--format=")
		case arg == "--spec":
			cfg.Spec = nextArg(args, &i, "--spec")
		case strings.HasPrefix(arg, "--spec="):
			cfg.Spec = strings.TrimPrefix(arg, "--spec=")
		case arg == "--preview":
			cfg.Preview = true
		case arg == "--brief":
			cfg.Brief = true
		case arg == "--verbose" || arg == "-v":
			cfg.Verbose = true
		case arg == "--output" || arg == "-o":
			cfg.Output = nextArg(args, &i, "--output")
		case strings.HasPrefix(arg, "--output="):
			cfg.Output = strings.TrimPrefix(arg, "--output=")
		}
	}

	if cfg.Output != "text" && cfg.Output != "json" {
		fmt.Fprintf(os.Stderr, "--output 必须是 text 或 json\n")
		os.Exit(2)
	}

	return cfg, true
}

func nextArg(args []string, i *int, name string) string {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "%s 需要一个值\n", name)
		os.Exit(2)
	}
	*i++
	return args[*i]
}

func parseIntArg(args []string, i *int, name string) int {
	return mustAtoi(nextArg(args, i, name), name)
}

func mustAtoi(s, name string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s 必须是整数: %s\n", name, s)
		os.Exit(2)
	}
	return n
}

// RunTool executes a CLI tool and prints the result.
func RunTool(cfg ToolConfig) error {
	switch cfg.Name {
	case "read":
		return runRead(cfg)
	case "replace":
		return runReplace(cfg)
	case "insert":
		return runInsert(cfg)
	case "delete":
		return runDelete(cfg)
	case "write":
		return runWrite(cfg)
	case "balance":
		return runBalance(cfg)
	case "func-range":
		return runFuncRange(cfg)
	case "tag-range":
		return runTagRange(cfg)
	default:
		return fmt.Errorf("unknown tool: %s", cfg.Name)
	}
}

func runRead(cfg ToolConfig) error {
	endLine := cfg.End
	if cfg.EndAuto {
		endLine = -1
	}
	res, _, err := betools.Read(cfg.File, cfg.Start, endLine, cfg.Brief)
	if err != nil {
		return err
	}
	return printResult(res, cfg.Output)
}

func runReplace(cfg ToolConfig) error {
	content, err := resolveContent(cfg.Content, cfg.ContentFile)
	if err != nil {
		return err
	}
	var old *string
	if cfg.Old != "" || cfg.OldFile != "" {
		o, err := resolveContent(cfg.Old, cfg.OldFile)
		if err != nil {
			return err
		}
		old = &o
	}
	res, err := betools.Replace(cfg.File, cfg.Start, cfg.End, old, content, cfg.Format, cfg.Preview, "", cfg.Brief)
	if err != nil {
		return err
	}
	return printResult(res, cfg.Output)
}

func runInsert(cfg ToolConfig) error {
	content, err := resolveContent(cfg.Content, cfg.ContentFile)
	if err != nil {
		return err
	}
	res, err := betools.Insert(cfg.File, cfg.After, content, cfg.Format, cfg.Preview, cfg.Brief)
	if err != nil {
		return err
	}
	return printResult(res, cfg.Output)
}

func runDelete(cfg ToolConfig) error {
	res, err := betools.Delete(cfg.File, cfg.Start, cfg.End, cfg.Format, cfg.Preview, cfg.Brief)
	if err != nil {
		return err
	}
	return printResult(res, cfg.Output)
}

func runWrite(cfg ToolConfig) error {
	spec := cfg.Spec
	if spec == "" && cfg.File != "" {
		content, err := resolveContent(cfg.Content, cfg.ContentFile)
		if err != nil {
			return err
		}
		spec = mustJSON(map[string]any{
			"file":    cfg.File,
			"content": normalizeLineBreaks(content),
		})
	}
	if spec == "" {
		return fmt.Errorf("write: 需要 --spec 或 --file 与 --content/--content-file")
	}
	res, err := betools.Write(spec, cfg.Preview, cfg.Brief)
	if err != nil {
		return err
	}
	return printResult(res, cfg.Output)
}

func runBalance(cfg ToolConfig) error {
	out, err := betools.CheckStructureBalance(cfg.File, cfg.Verbose)
	if err != nil {
		return err
	}
	if cfg.Output == "json" {
		return printJSON(map[string]any{"status": "ok", "result": out})
	}
	fmt.Println(out)
	return nil
}

func runFuncRange(cfg ToolConfig) error {
	res, err := betools.FuncRange(cfg.File, cfg.Line)
	if err != nil {
		return err
	}
	return printResult(res, cfg.Output)
}

func runTagRange(cfg ToolConfig) error {
	res, err := betools.TagRange(cfg.File, cfg.Line)
	if err != nil {
		return err
	}
	return printResult(res, cfg.Output)
}

// resolveContent returns content from --content, --content-file, or stdin.
// If content is non-empty, it is used directly. If contentFile is "-",
// content is read from stdin. Otherwise contentFile is read as a file.
func resolveContent(content, contentFile string) (string, error) {
	if content != "" && contentFile != "" {
		return "", fmt.Errorf("--content 和 --content-file 不能同时使用")
	}
	if content != "" {
		return content, nil
	}
	if contentFile == "" {
		return "", nil
	}
	if contentFile == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("读取 stdin 失败: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(contentFile)
	if err != nil {
		return "", fmt.Errorf("读取 content file 失败: %w", err)
	}
	return string(data), nil
}

func printResult(v any, output string) error {
	if output == "json" {
		return printJSON(v)
	}
	return printText(v)
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

// normalizeLineBreaks converts literal escape sequences (\n, \t) introduced by
// shell quoting into real control characters, matching the behaviour agents
// expect when passing content through CLI flags.
func normalizeLineBreaks(s string) string {
	if strings.Contains(s, "\n") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case 't':
				b.WriteByte('\t')
				i++
				continue
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func printText(v any) error {
	switch r := v.(type) {
	case betools.ShowResult:
		fmt.Printf("%s\n", r.Status)
		if !r.Brief {
			fmt.Println(r.Content)
		} else {
			fmt.Printf("行 %d-%d（共 %d 行）\n", r.Start, r.End, r.Total)
		}
		printWarnings(r.Warnings)
	case betools.ReplaceResult:
		fmt.Printf("%s: 删除 %d 行，新增 %d 行，当前共 %d 行\n", r.Status, r.Removed, r.Added, r.Total)
		if r.Warning != "" {
			fmt.Printf("警告: %s\n", r.Warning)
		}
		printDiff(r.Diff)
		printWarnings(r.Warnings)
	case betools.InsertResult:
		fmt.Printf("%s: 在 %d 行后插入 %d 行，当前共 %d 行\n", r.Status, r.After, r.Added, r.Total)
		printDiff(r.Diff)
		printWarnings(r.Warnings)
	case betools.DeleteResult:
		fmt.Printf("%s: 当前共 %d 行\n", r.Status, r.Total)
		printDiff(r.Diff)
		printWarnings(r.Warnings)
	case betools.WriteResult:
		fmt.Printf("%s: 写入 %d 个文件\n", r.Status, r.Files)
		if r.Warning != "" {
			fmt.Printf("警告: %s\n", r.Warning)
		}
		for _, f := range r.Results {
			fmt.Printf("  %s: %d 行，%d 字节\n", f.File, f.Lines, f.Bytes)
			printWarnings(f.Warnings)
		}
	case betools.FunctionRangeResult:
		fmt.Printf("函数范围: %d-%d\n", r.Start, r.End)
	case betools.TagRangeResult:
		fmt.Printf("标签范围: %d-%d", r.Start, r.End)
		if r.Tag != "" {
			fmt.Printf(" (%s)", r.Tag)
		}
		fmt.Println()
	default:
		return printJSON(v)
	}
	return nil
}

func printDiff(diff string) {
	if diff == "" {
		return
	}
	fmt.Println("--- diff ---")
	fmt.Println(diff)
}

func printWarnings(warnings []string) {
	for _, w := range warnings {
		fmt.Printf("警告: %s\n", w)
	}
}

func printToolHelp(name string) {
	fmt.Printf("Usage: %s %s [options]\n\n", Name, name)
	switch name {
	case "read":
		fmt.Println("Options:")
		fmt.Println("  --file, -f <path>     要读取的文件")
		fmt.Println("  --start, -s <n>       起始行号")
		fmt.Println("  --end, -e <n|auto>    结束行号，auto 表示自动扩展到函数范围")
		fmt.Println("  --brief               只返回元数据")
		fmt.Println("  --output, -o text|json  输出格式")
	case "replace":
		fmt.Println("Options:")
		fmt.Println("  --file, -f <path>     要修改的文件")
		fmt.Println("  --start, -s <n>       起始行号")
		fmt.Println("  --end, -e <n>         结束行号")
		fmt.Println("  --content, -c <text>  替换内容")
		fmt.Println("  --content-file <path> 从文件读取替换内容（path 为 - 时从 stdin 读取）")
		fmt.Println("  --old <text>          旧内容（必须完全匹配）")
		fmt.Println("  --old-file <path>     从文件读取旧内容（path 为 - 时从 stdin 读取）")
		fmt.Println("  --preview             只输出 diff，不写入文件")
		fmt.Println("  --brief               返回精简结果")
		fmt.Println("  --output, -o text|json  输出格式")
	case "insert":
		fmt.Println("Options:")
		fmt.Println("  --file, -f <path>     要修改的文件")
		fmt.Println("  --after, --after-line <n>  在此行后插入（0 表示文件开头）")
		fmt.Println("  --content, -c <text>  插入内容")
		fmt.Println("  --content-file <path> 从文件读取插入内容（path 为 - 时从 stdin 读取）")
		fmt.Println("  --preview             只输出 diff，不写入文件")
		fmt.Println("  --brief               返回精简结果")
		fmt.Println("  --output, -o text|json  输出格式")
	case "delete":
		fmt.Println("Options:")
		fmt.Println("  --file, -f <path>     要修改的文件")
		fmt.Println("  --start, -s <n>       起始行号")
		fmt.Println("  --end, -e <n>         结束行号")
		fmt.Println("  --preview             只输出 diff，不写入文件")
		fmt.Println("  --brief               返回精简结果")
		fmt.Println("  --output, -o text|json  输出格式")
	case "write":
		fmt.Println("Options:")
		fmt.Println("  --file, -f <path>     要写入的文件")
		fmt.Println("  --content, -c <text>  文件内容")
		fmt.Println("  --content-file <path> 从文件读取内容（path 为 - 时从 stdin 读取）")
		fmt.Println("  --spec <json>         JSON 规格（优先于 file/content/content-file）")
		fmt.Println("  --preview             只输出结果，不写入文件")
		fmt.Println("  --brief               返回精简结果")
		fmt.Println("  --output, -o text|json  输出格式")
	case "balance":
		fmt.Println("Options:")
		fmt.Println("  --file, -f <path>     要检查的文件")
		fmt.Println("  --verbose, -v         输出完整匹配对")
		fmt.Println("  --output, -o text|json  输出格式")
	case "func-range":
		fmt.Println("Options:")
		fmt.Println("  --file, -f <path>     目标文件")
		fmt.Println("  --line, -l <n>        行号")
		fmt.Println("  --output, -o text|json  输出格式")
	case "tag-range":
		fmt.Println("Options:")
		fmt.Println("  --file, -f <path>     目标文件")
		fmt.Println("  --line, -l <n>        行号")
		fmt.Println("  --output, -o text|json  输出格式")
	}
}
