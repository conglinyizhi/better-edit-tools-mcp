package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: %s <package-zip|release-notes> [args...]", filepath.Base(os.Args[0]))
	}

	var err error
	switch os.Args[1] {
	case "package-zip":
		err = runPackageZip(os.Args[2:])
	case "release-notes":
		err = runReleaseNotes(os.Args[2:])
	default:
		err = fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
	if err != nil {
		fatalf("%v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
