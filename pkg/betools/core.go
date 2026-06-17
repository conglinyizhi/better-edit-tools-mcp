package betools

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"runtime"
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

type syncer interface {
	Sync(name string) error
}

// filePlan describes a single target file in a multi-file atomic write.
type filePlan struct {
	file     string
	tmp      string
	backup   string
	existed  bool
	original []byte
	mode     fs.FileMode
}

// restorePlan describes a single target file during crash-safe rollback.
type restorePlan struct {
	file       string
	restoreTmp string
	deleteTmp  string
	existed    bool
}

func syncFile(path string, fsys FileSystem) error {
	if s, ok := fsys.(syncer); ok {
		return s.Sync(path)
	}
	return nil
}

func syncParentDir(path string, fsys FileSystem) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	parent := filepath.Dir(path)
	if parent == path {
		return nil
	}
	return syncFile(parent, fsys)
}

func writeAndSyncFile(path string, data []byte, perm fs.FileMode, fsys FileSystem) error {
	if err := fsys.WriteFile(path, data, perm); err != nil {
		return err
	}
	return syncFile(path, fsys)
}

func writeFileAtomic(path, content string, opts ...Option) error {
	return writeFileAtomicWithMode(path, content, 0o644, opts...)
}

func writeFileAtomicWithMode(path, content string, mode fs.FileMode, opts ...Option) error {
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
	if err := writeAndSyncFile(tmpPath, []byte(content), mode, cfg.fs); err != nil {
		return err
	}
	if err := cfg.fs.Rename(tmpPath, abs); err != nil {
		_ = cfg.fs.Remove(tmpPath)
		return err
	}
	return syncParentDir(abs, cfg.fs)
}

func writeFilesAtomic(writes []WriteSpecItem, opts ...Option) error {
	if len(writes) == 0 {
		return nil
	}
	cfg := withCallConfig(opts...)

	plans := make([]filePlan, 0, len(writes))

	// Phase 1: write all content temp files and backup temp files, fsync each.
	// Existing files keep their original mode through the atomic rename.
	for _, w := range writes {
		info, err := cfg.fs.Stat(w.File)
		var existed bool
		var mode fs.FileMode = 0o644
		var original []byte
		if err == nil {
			existed = true
			mode = info.Mode().Perm()
			data, readErr := cfg.fs.ReadFile(w.File)
			if readErr != nil {
				cleanupPlans(plans, cfg.fs)
				return readErr
			}
			original = data
		} else if errors.Is(err, fs.ErrNotExist) {
			existed = false
		} else {
			cleanupPlans(plans, cfg.fs)
			return err
		}

		tmpPath := makeTempPath(w.File, "tmp")
		if err := writeAndSyncFile(tmpPath, []byte(w.Content), mode, cfg.fs); err != nil {
			cleanupPlans(plans, cfg.fs)
			return err
		}

		if existed {
			backupPath := makeTempPath(w.File, "bak")
			if err := writeAndSyncFile(backupPath, original, mode, cfg.fs); err != nil {
				cleanupPlans(plans, cfg.fs)
				return err
			}
			plans = append(plans, filePlan{
				file:     w.File,
				tmp:      tmpPath,
				backup:   backupPath,
				existed:  true,
				original: original,
				mode:     mode,
			})
		} else {
			plans = append(plans, filePlan{
				file:    w.File,
				tmp:     tmpPath,
				existed: false,
			})
		}
	}

	// Phase 2: commit all temp files to their targets with atomic renames.
	for i, p := range plans {
		if err := cfg.fs.Rename(p.tmp, p.file); err != nil {
			if rbErr := rollbackCommitted(plans[:i], cfg.fs); rbErr != nil {
				cleanupPlans(plans, cfg.fs)
				return fmt.Errorf("commit %s failed: %v; rollback failed: %v", p.file, err, rbErr)
			}
			cleanupPlans(plans, cfg.fs)
			return err
		}
	}

	// Phase 3: fsync parent directories and clean up backup files.
	syncParentDirs(plans, cfg.fs)
	cleanupPlans(plans, cfg.fs)
	return nil
}

func makeTempPath(file, suffix string) string {
	parent := filepath.Dir(file)
	stem := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	if stem == "" {
		stem = "tmp"
	}
	counter := atomic.AddUint64(&tmpCounter, 1) - 1
	return filepath.Join(parent, fmt.Sprintf(".fe-%s-%d.%s.tmp", stem, counter, suffix))
}

// rollbackCommitted rolls back files that have already been renamed to their
// targets. To stay crash-safe, it writes original contents to restore temp
// files (or prepares delete temps for newly-created files) and performs a
// unified rename wave before syncing parent directories and cleaning up.
func rollbackCommitted(plans []filePlan, fsys FileSystem) error {
	if len(plans) == 0 {
		return nil
	}

	var rps []restorePlan
	for _, p := range plans {
		if p.existed {
			restoreTmp := makeTempPath(p.file, "restore")
			if err := writeAndSyncFile(restoreTmp, p.original, p.mode, fsys); err != nil {
				cleanupRestorePlans(rps, fsys)
				return err
			}
			rps = append(rps, restorePlan{file: p.file, restoreTmp: restoreTmp, existed: true})
		} else {
			deleteTmp := makeTempPath(p.file, "del")
			rps = append(rps, restorePlan{file: p.file, deleteTmp: deleteTmp, existed: false})
		}
	}

	// Unified rename wave: restore originals and remove newly-created files.
	for _, rp := range rps {
		if rp.existed {
			if err := fsys.Rename(rp.restoreTmp, rp.file); err != nil {
				cleanupRestorePlans(rps, fsys)
				return err
			}
		} else {
			if err := fsys.Rename(rp.file, rp.deleteTmp); err != nil {
				cleanupRestorePlans(rps, fsys)
				return err
			}
		}
	}

	syncParentDirsOfRestore(rps, fsys)
	cleanupRestorePlans(rps, fsys)
	return nil
}

func cleanupPlans(plans []filePlan, fsys FileSystem) {
	for _, p := range plans {
		_ = fsys.Remove(p.tmp)
		if p.backup != "" {
			_ = fsys.Remove(p.backup)
		}
	}
}

func cleanupRestorePlans(rps []restorePlan, fsys FileSystem) {
	for _, rp := range rps {
		if rp.existed {
			_ = fsys.Remove(rp.restoreTmp)
		} else {
			_ = fsys.Remove(rp.deleteTmp)
		}
	}
}

func syncParentDirs(plans []filePlan, fsys FileSystem) {
	seen := make(map[string]struct{})
	for _, p := range plans {
		parent := filepath.Dir(p.file)
		if _, ok := seen[parent]; ok {
			continue
		}
		seen[parent] = struct{}{}
		_ = syncParentDir(p.file, fsys)
	}
}

func syncParentDirsOfRestore(rps []restorePlan, fsys FileSystem) {
	seen := make(map[string]struct{})
	for _, rp := range rps {
		parent := filepath.Dir(rp.file)
		if _, ok := seen[parent]; ok {
			continue
		}
		seen[parent] = struct{}{}
		_ = syncParentDir(rp.file, fsys)
	}
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
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			default:
				// For unrecognized escapes, emit the escaped char only
				b.WriteByte(text[i])
			}
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func prepareContentLines(content, lineEnding string) []string {
	if content == "" {
		return nil
	}
	parts := strings.Split(content, "\n")
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
//
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

	tabCount := strings.Count(content, "\t")
	if tabCount > 0 && !isTabDominant(content) {
		warnings = append(warnings, fmt.Sprintf("content contains %d tab characters — verify indentation was preserved", tabCount))
	}

	lines := strings.Split(content, "\n")
	hasTrailing := false
	for _, line := range lines {
		if len(line) > 0 {
			// Strip line number prefix (e.g., "12\t") before checking content
			contentPart := line
			if tabIdx := strings.Index(line, "\t"); tabIdx >= 0 {
				contentPart = line[tabIdx+1:]
			}
			if contentPart == "" {
				continue
			}
			last := contentPart[len(contentPart)-1]
			if last == ' ' || last == '\t' {
				hasTrailing = true
				break
			}
		}
	}
	if hasTrailing {
		warnings = append(warnings, "content contains trailing whitespace on one or more lines")
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		warnings = append(warnings, "file does not end with a newline")
	}

	return warnings
}

// isTabDominant returns true when >50% of non-empty lines start with a tab.
// For Go, Python, Makefile, and shell scripts, tab indentation is conventional,
// so the tab warning would be noise.
func isTabDominant(content string) bool {
	lines := strings.Split(content, "\n")
	var total, tabStart int
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		total++
		if line[0] == '\t' {
			tabStart++
		}
	}
	if total == 0 {
		return false
	}
	return tabStart > total/2
}
