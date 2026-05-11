package app

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Lang string
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
	fmt.Printf("%s %s\n\nUsage:\n  %s [--lang <zh|en>] [--version] [--help]\n\nRuns the MCP server over stdio by default.\n", Name, Version, Name)
}
