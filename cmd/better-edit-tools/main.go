package main

import (
	"fmt"
	"os"

	"github.com/conglinyizhi/better-edit-tools-mcp/internal/app"
	"github.com/conglinyizhi/better-edit-tools-mcp/internal/server"
)

func main() {
	args := os.Args[1:]

	// CLI tool mode: first argument is a known subcommand.
	if len(args) > 0 && app.KnownToolCommands[args[0]] {
		toolCfg, ok := app.ParseToolArgs(args)
		if !ok {
			return
		}
		if err := app.RunTool(toolCfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	cfg, ok := app.ParseArgs(args)
	if !ok {
		return
	}

	if err := server.Run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
