package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fatalf("usage: %s <archive> <binary>", filepath.Base(os.Args[0]))
	}

	archivePath := os.Args[1]
	binaryPath := os.Args[2]

	if err := packageZip(archivePath, binaryPath); err != nil {
		fatalf("%v", err)
	}
}

func packageZip(archivePath, binaryPath string) error {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}

	archive, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer archive.Close()

	zw := zip.NewWriter(archive)
	defer zw.Close()

	binary, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("open binary: %w", err)
	}
	defer binary.Close()

	info, err := binary.Stat()
	if err != nil {
		return fmt.Errorf("stat binary: %w", err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("create zip header: %w", err)
	}
	header.Name = filepath.Base(binaryPath)
	header.Method = zip.Deflate

	entry, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create zip entry: %w", err)
	}

	if _, err := io.Copy(entry, binary); err != nil {
		return fmt.Errorf("write zip entry: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close archive: %w", err)
	}
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
