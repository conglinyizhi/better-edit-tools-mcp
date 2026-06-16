package app

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Lang     string
	NoPrefix bool
}

func ParseArgs(args []string) (Config, bool) {
	var cfg Config
	var lang string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help" || arg == "-h":
			printHelp()
			return Config{}, false
		case arg == "--version" || arg == "-V":
			fmt.Println(Version)
			return Config{}, false
		case arg == "--no-prefix":
			cfg.NoPrefix = true
		case arg == "--lang":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--lang 需要一个语言标签值，例如 zh 或 en")
				return Config{}, false
			}
			lang = args[i+1]
			i++
		case strings.HasPrefix(arg, "--lang="):
			lang = strings.TrimPrefix(arg, "--lang=")
		}
	}

	if lang == "" {
		lang = LangFromEnv()
	}
	if NormalizeLang(lang) == "" {
		fmt.Fprintf(os.Stderr, "不支持的 --lang 值: %s\n", lang)
		return Config{}, false
	}

	cfg.Lang = NormalizeLang(lang)
	return cfg, true
}

func printHelp() {
	fmt.Printf("%s %s\n\nUsage:\n  %s [--lang <zh|en>] [--no-prefix] [--version] [--help]\n  %s <command> [options]\n\nRuns the MCP server over stdio by default.\n\nFlags:\n  --no-prefix    Hide the be- prefix from tool names\n\nCLI commands:\n  read, replace, insert, delete, write, balance, func-range, tag-range\n\nUse `%s <command> --help` for command-specific options.\n", Name, Version, Name, Name, Name)
}
