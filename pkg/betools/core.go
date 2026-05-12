package betools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

var tmpCounter uint64

func readLines(path string) ([]string, string, error) {
	data, err := os.ReadFile(path)
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

func writeFileAtomic(path, content string) error {
	abs := path
	parent := filepath.Dir(abs)
	stem := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	if stem == "" {
		stem = "tmp"
	}
	counter := atomic.AddUint64(&tmpCounter, 1) - 1
	tmpName := fmt.Sprintf(".fe-%s-%d.tmp", stem, counter)
	tmpPath := filepath.Join(parent, tmpName)
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, abs); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func writeFilesAtomic(writes []WriteSpecItem) error {
	if len(writes) == 0 {
		return nil
	}
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
		if err := os.WriteFile(tmpPath, []byte(w.Content), 0o644); err != nil {
			return err
		}
		tmps = append(tmps, tmpItem{file: w.File, tmp: tmpPath})

		if _, err := os.Stat(w.File); err == nil {
			backupPath := filepath.Join(parent, fmt.Sprintf(".fe-%s-%d.bak", stem, counter))
			if err := copyFile(w.File, backupPath); err != nil {
				return err
			}
			backups = append(backups, backupItem{file: w.File, backup: backupPath, exists: true})
		} else if errors.Is(err, os.ErrNotExist) {
			backups = append(backups, backupItem{file: w.File, exists: false})
		} else {
			return err
		}
	}

	var committed []string
	for _, item := range tmps {
		if err := os.Rename(item.tmp, item.file); err != nil {
			for i := len(committed) - 1; i >= 0; i-- {
				target := committed[i]
				for _, bk := range backups {
					if bk.file == target {
						if bk.exists {
							_ = os.Rename(bk.backup, bk.file)
						} else {
							_ = os.Remove(bk.file)
						}
					}
				}
			}
			for _, it := range tmps {
				_ = os.Remove(it.tmp)
			}
			for _, bk := range backups {
				if bk.exists {
					_ = os.Remove(bk.backup)
				}
			}
			return err
		}
		committed = append(committed, item.file)
	}

	for _, bk := range backups {
		if bk.exists {
			_ = os.Remove(bk.backup)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return out.Close()
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
	if !raw {
		parsed = parseContent(content)
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
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
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
