package betools

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

var tmpCounter uint64

func readLines(path string, opts ...Option) ([]string, string, error) {
	cfg := withCallConfig(opts...)
	data, err := cfg.fs.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	content := string(data)
	le := detectLineEnding(content)
	lines := splitKeepLineEnding(content, le)
	return lines, le, nil
}

func detectLineEnding(content string) string {
	lineCount := rustLineCount(content)
	if lineCount == 0 {
		return "\n"
	}
	crlf := strings.Count(content, "\r\n")
	if crlf > lineCount/2 {
		return "\r\n"
	}
	return "\n"
}

func splitKeepLineEnding(content, le string) []string {
	if content == "" {
		return nil
	}
	parts := strings.Split(content, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		lines = append(lines, strings.TrimSuffix(part, "\r")+le)
	}
	return lines
}

func writeFileAtomic(path, content string, opts ...Option) error {
	cfg := withCallConfig(opts...)
	abs := path
	parent := filepath.Dir(abs)
	stem := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	if stem == "" {
		stem = "tmp"
	}
	counter := atomic.AddUint64(&tmpCounter, 1) - 1
	tmpName := fmt.Sprintf(".fe-%s-%d.tmp", stem, counter)
	tmpPath := filepath.Join(parent, tmpName)
	if err := cfg.fs.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return err
	}
	if err := cfg.fs.Rename(tmpPath, abs); err != nil {
		_ = cfg.fs.Remove(tmpPath)
		return err
	}
	return nil
}

func writeFilesAtomic(writes []WriteSpecItem, opts ...Option) error {
	if len(writes) == 0 {
		return nil
	}
	cfg := withCallConfig(opts...)
	type tmpItem struct {
		file string
		tmp  string
	}
	type backupItem struct {
		file   string
		backup string
		exists bool
	}
	var tmps []tmpItem
	var backups []backupItem

	for _, w := range writes {
		parent := filepath.Dir(w.File)
		stem := strings.TrimSuffix(filepath.Base(w.File), filepath.Ext(w.File))
		if stem == "" {
			stem = "tmp"
		}
		counter := atomic.AddUint64(&tmpCounter, 1) - 1
		tmpName := fmt.Sprintf(".fe-%s-%d.tmp", stem, counter)
		tmpPath := filepath.Join(parent, tmpName)
		if err := cfg.fs.WriteFile(tmpPath, []byte(w.Content), 0o644); err != nil {
			return err
		}
		tmps = append(tmps, tmpItem{file: w.File, tmp: tmpPath})

		if _, err := cfg.fs.Stat(w.File); err == nil {
			backupPath := filepath.Join(parent, fmt.Sprintf(".fe-%s-%d.bak", stem, counter))
			if err := copyFile(w.File, backupPath, WithFileSystem(cfg.fs)); err != nil {
				return err
			}
			backups = append(backups, backupItem{file: w.File, backup: backupPath, exists: true})
		} else if errors.Is(err, fs.ErrNotExist) {
			backups = append(backups, backupItem{file: w.File, exists: false})
		} else {
			return err
		}
	}

	var committed []string
	for _, item := range tmps {
		if err := cfg.fs.Rename(item.tmp, item.file); err != nil {
			for i := len(committed) - 1; i >= 0; i-- {
				target := committed[i]
				for _, bk := range backups {
					if bk.file == target {
						if bk.exists {
							_ = cfg.fs.Rename(bk.backup, bk.file)
						} else {
							_ = cfg.fs.Remove(bk.file)
						}
					}
				}
			}
			for _, it := range tmps {
				_ = cfg.fs.Remove(it.tmp)
			}
			for _, bk := range backups {
				if bk.exists {
					_ = cfg.fs.Remove(bk.backup)
				}
			}
			return err
		}
		committed = append(committed, item.file)
	}

	for _, bk := range backups {
		if bk.exists {
			_ = cfg.fs.Remove(bk.backup)
		}
	}
	return nil
}

func copyFile(src, dst string, opts ...Option) error {
	cfg := withCallConfig(opts...)
	in, err := cfg.fs.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := cfg.fs.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func rejectBinary(path string, fsys FileSystem) error {
	const sampleSize = 512
	f, err := fsys.Open(path)
	if err != nil {
		return readPath(path, err)
	}
	defer f.Close()

	buf := make([]byte, sampleSize)
	n, readErr := io.ReadFull(f, buf)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return readPath(path, readErr)
	}
	buf = buf[:n]
	if len(buf) == 0 {
		return nil
	}
	if isBinarySample(buf) {
		return invalidArg(fmt.Sprintf("show: %s appears to be a binary file; use a text file or a dedicated binary inspection tool", filepath.Clean(path)))
	}
	return nil
}

func isBinarySample(data []byte) bool {
	if isBinary(data) {
		return true
	}
	if !utf8.Valid(data) {
		return true
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	var control int
	for _, b := range sample {
		switch b {
		case 0, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x0b, 0x0c, 0x0e, 0x0f:
			control++
		default:
			if b < 0x20 && b != '\n' && b != '\r' && b != '\t' {
				control++
			}
		}
	}
	return control > len(sample)/6
}

func parseContent(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '\\' && i+1 < len(text) {
			i++
			switch text[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte('\\')
				b.WriteByte(text[i])
			}
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func prepareContentLines(content, lineEnding string, raw bool) []string {
	parsed := content
	// Always normalize line breaks first (handles JSON degradation of \\n)
	parsed = normalizeLineBreaks(parsed)
	if !raw {
		parsed = parseContent(parsed)
	}
	if parsed == "" {
		return nil
	}
	parts := strings.Split(parsed, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		lines = append(lines, strings.TrimSuffix(part, "\r")+lineEnding)
	}
	for len(lines) > 0 {
		trimmed := strings.TrimRight(lines[len(lines)-1], "\n\r")
		if len(trimmed) == 0 {
			lines = lines[:len(lines)-1]
		} else {
			break
		}
	}
	return lines
}

// normalizeLineBreaks detects when JSON serialization degraded real newlines
// into literal \\n (two characters: backslash + n) and fixes it.
// The heuristic:
//   - If content already has real newlines → correctly transmitted, no-op.
//   - If no real newlines but contains literal \\n → semantic degradation, fix.
//   - Otherwise → no-op.
// This is idempotent: running it on already-correct content produces no change.
func normalizeLineBreaks(s string) string {
	if strings.Contains(s, "\n") {
		return s
	}
	if strings.Contains(s, `\n`) {
		return strings.ReplaceAll(s, `\n`, "\n")
	}
	return s
}

func rustLineCount(content string) int {
	if content == "" {
		return 0
	}
	parts := strings.Split(content, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return len(parts)
}

func scanContentWarnings(content string) []string {
	var warnings []string

	tabCount := strings.Count(content, "	")
	if tabCount > 0 {
		warnings = append(warnings, fmt.Sprintf("content contains %d tab characters — verify indentation was preserved", tabCount))
	}

		lines := strings.Split(content, "\n")
	hasTrailing := false
	for _, line := range lines {
		if len(line) > 0 {
			last := line[len(line)-1]
			if last == ' ' || last == '	' {
				hasTrailing = true
				break
			}
		}
	}
	if hasTrailing {
		warnings = append(warnings, "content contains trailing whitespace on one or more lines")
	}

		if !strings.HasSuffix(content, "\n") {
		warnings = append(warnings, "file does not end with a newline")
	}

	return warnings
}
