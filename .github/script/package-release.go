package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func runPackageZip(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: package-zip <archive> <binary>")
	}

	return packageZip(args[0], args[1])
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
