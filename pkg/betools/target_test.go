package betools

import (
	"strings"
	"testing"
)

func TestResolveTargetSpan_FunctionPrefersDefinitionOverCall(t *testing.T) {
	content := strings.Join([]string{
		"package main",
		"",
		"func helper() {}",
		"",
		"func main() {",
		"\thelper()",
		"}",
		"",
	}, "\n")

	m, opt := withFS(map[string]string{"main.go": content})
	span, err := ResolveTargetSpan("main.go", ContentTarget{Kind: "function", Value: "helper"}, opt)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if span.Start != 3 || span.End != 3 {
		t.Fatalf("expected helper definition at line 3, got span %+v", span)
	}
	_ = m
}

func TestResolveTargetSpan_FunctionFallbackToCall(t *testing.T) {
	content := strings.Join([]string{
		"package main",
		"",
		"func main() {",
		"\thelper()",
		"}",
		"",
	}, "\n")

	m, opt := withFS(map[string]string{"main.go": content})
	span, err := ResolveTargetSpan("main.go", ContentTarget{Kind: "function", Value: "helper"}, opt)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	// When falling back to a call site, functionRangeRaw returns the enclosing
	// function's range (main spans lines 3-5).
	if span.Start != 3 || span.End != 5 {
		t.Fatalf("expected enclosing function range 3-5 for call site, got span %+v", span)
	}
	_ = m
}

func TestResolveTargetSpan_RustDefinitionOverCall(t *testing.T) {
	content := strings.Join([]string{
		"fn helper() {}",
		"",
		"fn main() {",
		"    helper();",
		"}",
		"",
	}, "\n")

	m, opt := withFS(map[string]string{"main.rs": content})
	span, err := ResolveTargetSpan("main.rs", ContentTarget{Kind: "function", Value: "helper"}, opt)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if span.Start != 1 || span.End != 1 {
		t.Fatalf("expected helper definition at line 1, got span %+v", span)
	}
	_ = m
}

func TestResolveTargetSpan_GoMethodReceiver(t *testing.T) {
	content := strings.Join([]string{
		"package main",
		"",
		"type T struct{}",
		"",
		"func (t *T) helper() {}",
		"",
		"func main() {",
		"\tvar t T",
		"\tt.helper()",
		"}",
		"",
	}, "\n")

	m, opt := withFS(map[string]string{"main.go": content})
	span, err := ResolveTargetSpan("main.go", ContentTarget{Kind: "function", Value: "helper"}, opt)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if span.Start != 5 || span.End != 5 {
		t.Fatalf("expected method definition at line 5, got span %+v", span)
	}
	_ = m
}
