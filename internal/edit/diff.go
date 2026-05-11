package edit

import (
	"fmt"
	"strings"
)

func QuickBalanceCheck(content string) string {
	var curly, square, paren int
	inString := false
	var stringChar byte
	escape := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(content); i++ {
		ch := content[i]
		var next byte
		if i+1 < len(content) {
			next = content[i+1]
		}

		if escape {
			escape = false
			continue
		}
		if ch == '\\' && inString {
			escape = true
			continue
		}
		if !inString && !inBlockComment && ch == '/' && next == '/' {
			inLineComment = true
			i++
			continue
		}
		if inLineComment && ch == '\n' {
			inLineComment = false
			continue
		}
		if inLineComment {
			continue
		}
		if !inString && !inBlockComment && ch == '/' && next == '*' {
			inBlockComment = true
			i++
			continue
		}
		if inBlockComment && ch == '*' && next == '/' {
			inBlockComment = false
			i++
			continue
		}
		if inBlockComment {
			continue
		}
		if (ch == '"' || ch == '\'' || ch == '`') && !inString {
			inString = true
			stringChar = ch
			continue
		}
		if inString && ch == stringChar {
			inString = false
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			curly++
		case '}':
			curly--
		case '[':
			square++
		case ']':
			square--
		case '(':
			paren++
		case ')':
			paren--
		}
	}

	var errs []string
	if curly != 0 {
		errs = append(errs, fmt.Sprintf("{} 差 %d 个", abs(curly)))
	}
	if square != 0 {
		errs = append(errs, fmt.Sprintf("[] 差 %d 个", abs(square)))
	}
	if paren != 0 {
		errs = append(errs, fmt.Sprintf("() 差 %d 个", abs(paren)))
	}
	if len(errs) == 0 {
		return "符号闭合快速检查：OK"
	}
	return "符号闭合快速检查：Error (" + strings.Join(errs, "; ") + ")"
}

func BuildDiff(before, after []string, baseLine int, format string) string {
	if format == "diff" {
		var b strings.Builder
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", baseLine, len(before), baseLine, len(after))
		maxLen := len(before)
		if len(after) > maxLen {
			maxLen = len(after)
		}
		for i := 0; i < maxLen; i++ {
			var bl, al string
			if i < len(before) {
				bl = strings.TrimRight(before[i], "\n\r")
			}
			if i < len(after) {
				al = strings.TrimRight(after[i], "\n\r")
			}
			switch {
			case i < len(before) && i < len(after) && bl == al:
				fmt.Fprintf(&b, " %s\n", bl)
			case i < len(before):
				fmt.Fprintf(&b, "-%s\n", bl)
				if i < len(after) {
					fmt.Fprintf(&b, "+%s\n", al)
				}
			case i < len(after):
				fmt.Fprintf(&b, "+%s\n", al)
			}
		}
		return strings.TrimRight(b.String(), "\n")
	}

	beforeEnd := baseLine + len(before) - 1
	afterEnd := baseLine + len(after) - 1
	var b strings.Builder
	fmt.Fprintf(&b, "--- 修改前（行 %d-%d）---\n", baseLine, beforeEnd)
	for i, line := range before {
		fmt.Fprintf(&b, "%d\t%s\n", baseLine+i, strings.TrimRight(line, "\n\r"))
	}
	fmt.Fprintf(&b, "\n+++ 修改后（行 %d-%d）+++\n", baseLine, afterEnd)
	for i, line := range after {
		fmt.Fprintf(&b, "%d\t%s\n", baseLine+i, strings.TrimRight(line, "\n\r"))
	}
	return strings.TrimRight(b.String(), "\n")
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
