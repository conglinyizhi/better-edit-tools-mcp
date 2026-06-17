package betools

import (
	"testing"
)

func TestParseFileRange_WindowsPath(t *testing.T) {
	cases := []struct {
		input     string
		wantFile  string
		wantStart int
		wantEnd   int
	}{
		{"C:\\path\\file.go", "C:\\path\\file.go", 0, 0},
		{"C:\\path\\file.go:10", "C:\\path\\file.go", 10, 10},
		{"C:\\path\\file.go:5-15", "C:\\path\\file.go", 5, 15},
		{"D:\\dir\\file.txt:ALL", "D:\\dir\\file.txt", -1, -1},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			file, start, end, err := ParseFileRange(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if file != tc.wantFile || start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("ParseFileRange(%q) = (%q, %d, %d), want (%q, %d, %d)",
					tc.input, file, start, end, tc.wantFile, tc.wantStart, tc.wantEnd)
			}
		})
	}
}

func TestParseFileRange_URLWithPort(t *testing.T) {
	cases := []struct {
		input     string
		wantFile  string
		wantStart int
		wantEnd   int
	}{
		{"file://host:8080/path/to/file.go", "file://host:8080/path/to/file.go", 0, 0},
		{"file://host:8080/path/to/file.go:23", "file://host:8080/path/to/file.go", 23, 23},
		{"file://host:8080/path/to/file.go:1-3", "file://host:8080/path/to/file.go", 1, 3},
		{"http://localhost:3000/src/main.rs:10", "http://localhost:3000/src/main.rs", 10, 10},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			file, start, end, err := ParseFileRange(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if file != tc.wantFile || start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("ParseFileRange(%q) = (%q, %d, %d), want (%q, %d, %d)",
					tc.input, file, start, end, tc.wantFile, tc.wantStart, tc.wantEnd)
			}
		})
	}
}

func TestParseFileRange_RegularPaths(t *testing.T) {
	cases := []struct {
		input     string
		wantFile  string
		wantStart int
		wantEnd   int
	}{
		{"path/to/file.go", "path/to/file.go", 0, 0},
		{"path/to/file.go:10", "path/to/file.go", 10, 10},
		{"path/to/file.go:5-15", "path/to/file.go", 5, 15},
		{"/abs/path/file.go:ALL", "/abs/path/file.go", -1, -1},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			file, start, end, err := ParseFileRange(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if file != tc.wantFile || start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("ParseFileRange(%q) = (%q, %d, %d), want (%q, %d, %d)",
					tc.input, file, start, end, tc.wantFile, tc.wantStart, tc.wantEnd)
			}
		})
	}
}
