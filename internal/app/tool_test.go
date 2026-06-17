package app

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestParseToolArgs_ReadRangeFlagsTracked(t *testing.T) {
	cases := []struct {
		name         string
		args         []string
		wantStart    int
		wantStartSet bool
		wantEnd      int
		wantEndSet   bool
		wantEndAuto  bool
	}{
		{
			name:         "no range flags",
			args:         []string{"read", "--file", "a.txt"},
			wantStart:    0,
			wantStartSet: false,
			wantEnd:      0,
			wantEndSet:   false,
			wantEndAuto:  false,
		},
		{
			name:         "explicit start only",
			args:         []string{"read", "--file", "a.txt", "--start", "5"},
			wantStart:    5,
			wantStartSet: true,
			wantEnd:      0,
			wantEndSet:   false,
			wantEndAuto:  false,
		},
		{
			name:         "explicit end only",
			args:         []string{"read", "--file", "a.txt", "--end", "10"},
			wantStart:    0,
			wantStartSet: false,
			wantEnd:      10,
			wantEndSet:   true,
			wantEndAuto:  false,
		},
		{
			name:         "explicit range",
			args:         []string{"read", "--file", "a.txt", "-s", "2", "-e", "4"},
			wantStart:    2,
			wantStartSet: true,
			wantEnd:      4,
			wantEndSet:   true,
			wantEndAuto:  false,
		},
		{
			name:         "end auto",
			args:         []string{"read", "--file", "a.txt", "--end", "auto"},
			wantStart:    0,
			wantStartSet: false,
			wantEnd:      0,
			wantEndSet:   false,
			wantEndAuto:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, ok := ParseToolArgs(tc.args)
			if !ok {
				t.Fatalf("ParseToolArgs(%v) returned false", tc.args)
			}
			if cfg.Start != tc.wantStart || cfg.StartSet != tc.wantStartSet {
				t.Fatalf("start mismatch: got (%d, %v), want (%d, %v)", cfg.Start, cfg.StartSet, tc.wantStart, tc.wantStartSet)
			}
			if cfg.End != tc.wantEnd || cfg.EndSet != tc.wantEndSet || cfg.EndAuto != tc.wantEndAuto {
				t.Fatalf("end mismatch: got (%d, %v, auto=%v), want (%d, %v, auto=%v)", cfg.End, cfg.EndSet, cfg.EndAuto, tc.wantEnd, tc.wantEndSet, tc.wantEndAuto)
			}
		})
	}
}

func TestRunRead_DefaultsToWholeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	content := "package main\n\nfunc main() {\n\tprintln(1)\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cfg := ToolConfig{Name: "read", File: path, Output: "json", Start: 0, StartSet: false, End: 0, EndSet: false}
	out := captureOutput(t, func() {
		if err := RunTool(cfg); err != nil {
			t.Fatalf("RunTool: %v", err)
		}
	})

	if !bytes.Contains(out, []byte(`"start": 1`)) {
		t.Fatalf("expected start=1, got: %s", out)
	}
	if !bytes.Contains(out, []byte(`"end": 5`)) {
		t.Fatalf("expected end=5, got: %s", out)
	}
}

func TestRunRead_OnlyEndDefaultsStartToOne(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	content := "a\nb\nc\nd\ne\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cfg := ToolConfig{Name: "read", File: path, Output: "json", Start: 0, StartSet: false, End: 3, EndSet: true}
	out := captureOutput(t, func() {
		if err := RunTool(cfg); err != nil {
			t.Fatalf("RunTool: %v", err)
		}
	})

	if !bytes.Contains(out, []byte(`"start": 1`)) {
		t.Fatalf("expected start=1, got: %s", out)
	}
	if !bytes.Contains(out, []byte(`"end": 3`)) {
		t.Fatalf("expected end=3, got: %s", out)
	}
}

func captureOutput(t *testing.T, fn func()) []byte {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured output: %v", err)
	}
	return out
}
