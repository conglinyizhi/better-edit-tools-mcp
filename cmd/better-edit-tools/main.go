package main

import (
	"fmt"
	"os"

	"github.com/conglinyizhi/better-edit-tools-mcp/internal/app"
	"github.com/conglinyizhi/better-edit-tools-mcp/internal/server"
)

func main() {
	cfg, ok := app.ParseArgs(os.Args[1:])
	if !ok {
		return
	}

	if err := server.Run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
