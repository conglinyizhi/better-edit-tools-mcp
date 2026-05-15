package betools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type scanResult struct {
	SymbolLines   map[string][]int
	Matched       []MatchedPair
	Unbalanced    []UnbalancedItem
	QuoteWarnings []QuoteWarning
	TagMatched    []MatchedPair
}

func CheckStructureBalance(path string, verbose bool, opts ...Option) (string, error) {
	res, err := scanFile(path, opts...)
	if err != nil {
		return "", err
	}
	var out map[string]any

	if verbose {
		tagInfo := "(none)"
		if len(res.TagMatched) > 0 {
			parts := make([]string, 0, len(res.TagMatched))
			for _, t := range res.TagMatched {
				parts = append(parts, fmt.Sprintf("%s  %d:%d", t.Symbol, t.OpenLine, t.CloseLine))
			}
			tagInfo = strings.Join(parts, "\n")
		}
		var tagTree any = nil
		if len(res.TagMatched) > 0 {
			tagTree = formatTagTree(res.TagMatched)
		}
		out = map[string]any{
			"mode":           "verbose",
			"symbols":        formatAggregate(res.SymbolLines),
			"tags":           tagInfo,
			"tag_tree":       tagTree,
			"matched":        res.Matched,
			"unbalanced":     res.Unbalanced,
			"quote_warnings": res.QuoteWarnings,
		}
	} else {
		if len(res.Unbalanced) > 0 || len(res.QuoteWarnings) > 0 {
			out = map[string]any{
				"mode":           "unbalanced",
				"unbalanced":     res.Unbalanced,
				"quote_warnings": res.QuoteWarnings,
			}
		} else {
			out = map[string]any{
				"mode":   "unbalanced",
				"status": "all balanced",
			}
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	return string(data), nil
}

func scanFile(path string, opts ...Option) (scanResult, error) {
	data, err := readText(path, opts...)
	if err != nil {
		return scanResult{}, readPath(path, err)
	}
	content := data
	lines := strings.Split(content, "\n")

	symbolLines := map[string][]int{"{": {}, "}": {}, "[": {}, "]": {}, "(": {}, ")": {}}
	stack := make([]tagItem, 0)
	var matched []MatchedPair
	var unbalanced []UnbalancedItem
	quoteLines := map[string][]int{"\"": {}, "'": {}}
	var tagStack []tagItem
	var tagMatched []MatchedPair
	voidElts := map[string]struct{}{
		"area": {}, "base": {}, "br": {}, "col": {}, "embed": {}, "hr": {}, "img": {},
		"input": {}, "link": {}, "meta": {}, "param": {}, "source": {}, "track": {}, "wbr": {},
	}

	type mode int
	const (
		modeCode mode = iota
		modeLineComment
		modeBlockComment
		modeString
		modeTemplate
		modeTemplateExpr
		modeRegex
	)
	type frame struct {
		mode       mode
		strChar    byte
		braceDepth int
		inClass    bool
	}
	modes := []frame{{mode: modeCode}}
	escapeNext := false
	regexCanStart := true
	pendingIdent := strings.Builder{}

	isIdentStart := func(ch byte) bool {
		return ch == '_' || ch == '$' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
	}
	isIdentContinue := func(ch byte) bool {
		return isIdentStart(ch) || (ch >= '0' && ch <= '9')
	}
	regexKeyword := func(word string) bool {
		switch word {
		case "return", "throw", "case", "else", "do", "typeof", "instanceof", "in", "new", "void", "delete", "yield", "await":
			return true
		default:
			return false
		}
	}
	regexCanStartAfter := func(ch byte) bool {
		return strings.ContainsRune("([{,;:=!?+-%&|^~<>", rune(ch))
	}

lineLoop:
	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		bytes := []byte(line)
		for col := 0; col < len(bytes); col++ {
			ch := bytes[col]
			var next byte
			if col+1 < len(bytes) {
				next = bytes[col+1]
			}
			if escapeNext {
				escapeNext = false
				continue
			}
			top := &modes[len(modes)-1]
			switch top.mode {
			case modeLineComment:
				continue lineLoop
			case modeBlockComment:
				if ch == '*' && next == '/' {
					modes = modes[:len(modes)-1]
					col++
				}
				continue
			case modeString:
				if ch == '\\' {
					escapeNext = true
					continue
				}
				if ch == top.strChar {
					modes = modes[:len(modes)-1]
					regexCanStart = false
				}
				continue
			case modeTemplate:
				if ch == '\\' {
					escapeNext = true
					continue
				}
				if ch == '`' {
					modes = modes[:len(modes)-1]
					regexCanStart = false
					continue
				}
				if ch == '$' && next == '{' {
					symbolLines["{"] = append(symbolLines["{"], lineNum)
					stack = append(stack, tagItem{name: "{", line: lineNum, col: col + 1})
					modes = append(modes, frame{mode: modeTemplateExpr, braceDepth: 1})
					regexCanStart = true
					col++
					continue
				}
				continue
			case modeRegex:
				if ch == '\\' {
					escapeNext = true
					continue
				}
				if ch == '[' {
					top.inClass = true
					continue
				}
				if ch == ']' {
					top.inClass = false
					continue
				}
				if ch == '/' && !top.inClass {
					modes = modes[:len(modes)-1]
					regexCanStart = false
				}
				continue
			case modeCode, modeTemplateExpr:
			}

			if isIdentContinue(ch) {
				if pendingIdent.Len() == 0 && isIdentStart(ch) {
					pendingIdent.WriteByte(ch)
					continue
				}
				if pendingIdent.Len() > 0 {
					pendingIdent.WriteByte(ch)
					continue
				}
			}
			if pendingIdent.Len() > 0 {
				regexCanStart = regexKeyword(pendingIdent.String())
				pendingIdent.Reset()
			}

			if ch == '/' && next == '/' {
				modes = append(modes, frame{mode: modeLineComment})
				col++
				continue
			}
			if ch == '/' && next == '*' {
				modes = append(modes, frame{mode: modeBlockComment})
				col++
				continue
			}
			if ch == '`' {
				modes = append(modes, frame{mode: modeTemplate})
				regexCanStart = true
				continue
			}
			if ch == '"' || ch == '\'' {
				modes = append(modes, frame{mode: modeString, strChar: ch})
				quoteLines[string(ch)] = append(quoteLines[string(ch)], lineNum)
				regexCanStart = false
				continue
			}
			if ch == '/' && regexCanStart && next != '/' && next != '*' && next != '\n' && next != '\r' {
				modes = append(modes, frame{mode: modeRegex})
				regexCanStart = false
				continue
			}

			if ch == '<' && (top.mode == modeCode || top.mode == modeTemplateExpr) {
				fullTag, nextIndex, ok := readFullTag(bytes, col)
				if !ok {
					continue
				}
				col = nextIndex
				if strings.HasPrefix(fullTag, "<!") || strings.HasPrefix(fullTag, "<?") {
					continue
				}
				if strings.HasPrefix(fullTag, "</") {
					tagName := extractTagName(fullTag, true)
					if tagName == "" {
						continue
					}
					tagName = strings.ToLower(tagName)
					if len(tagStack) > 0 && tagStack[len(tagStack)-1].name == tagName {
						last := tagStack[len(tagStack)-1]
						tagStack = tagStack[:len(tagStack)-1]
						tagMatched = append(tagMatched, MatchedPair{Symbol: "<" + tagName + ">", OpenLine: last.line, CloseLine: lineNum, Depth: len(tagStack) + 1})
					} else if len(tagStack) == 0 {
						unbalanced = append(unbalanced, UnbalancedItem{Symbol: "</" + tagName + ">", Line: lineNum, Expected: "(无对应开标签)"})
					} else {
						unbalanced = append(unbalanced, UnbalancedItem{Symbol: "</" + tagName + ">", Line: lineNum, Expected: "</" + tagStack[len(tagStack)-1].name + ">"})
					}
					continue
				}
				tagName := extractTagName(fullTag, false)
				if tagName == "" {
					continue
				}
				trimmed := strings.TrimSpace(fullTag)
				if strings.HasSuffix(trimmed, "/>") {
					continue
				}
				if _, ok := voidElts[strings.ToLower(tagName)]; ok {
					continue
				}
				tagStack = append(tagStack, tagItem{name: strings.ToLower(tagName), line: lineNum, col: col + 1})
				continue
			}

			inTemplateExpr := top.mode == modeTemplateExpr
			closeTemplateExpr := false
			chStr := string(ch)
			if _, ok := symbolLines[chStr]; ok {
				symbolLines[chStr] = append(symbolLines[chStr], lineNum)
			}
			if ch == '{' || ch == '[' || ch == '(' {
				stack = append(stack, tagItem{name: chStr, line: lineNum, col: col + 1})
				if inTemplateExpr && ch == '{' {
					top.braceDepth++
				}
			} else if ch == '}' || ch == ']' || ch == ')' {
				expectedOpen := map[byte]string{'}': "{", ']': "[", ')': "("}[ch]
				if len(stack) > 0 && stack[len(stack)-1].name == expectedOpen {
					last := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					matched = append(matched, MatchedPair{Symbol: expectedOpen + chStr, OpenLine: last.line, CloseLine: lineNum, Depth: len(stack) + 1})
				} else {
					unbalanced = append(unbalanced, UnbalancedItem{Symbol: chStr, Line: lineNum, Col: col + 1, Expected: map[byte]string{'}': "{", ']': "[", ')': "("}[ch]})
				}
				if inTemplateExpr && ch == '}' {
					if top.braceDepth > 1 {
						top.braceDepth--
					} else {
						closeTemplateExpr = true
					}
				}
			}

			if closeTemplateExpr {
				modes = modes[:len(modes)-1]
				regexCanStart = true
			}
			if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
				regexCanStart = regexCanStartAfter(ch)
			}
		}
		if pendingIdent.Len() > 0 {
			regexCanStart = regexKeyword(pendingIdent.String())
			pendingIdent.Reset()
		}
		if len(modes) > 0 && modes[len(modes)-1].mode == modeLineComment {
			modes = modes[:len(modes)-1]
		}
	}

	for _, item := range stack {
		unbalanced = append(unbalanced, UnbalancedItem{Symbol: item.name, Line: item.line, Col: item.col, Expected: map[string]string{"{": "}", "[": "]", "(": ")"}[item.name]})
	}
	for _, item := range tagStack {
		unbalanced = append(unbalanced, UnbalancedItem{Symbol: "<" + item.name + ">", Line: item.line, Col: item.col, Expected: "</" + item.name + ">"})
	}

	var quoteWarnings []QuoteWarning
	for sym, list := range quoteLines {
		if len(list)%2 != 0 {
			uniq := append([]int(nil), list...)
			sort.Ints(uniq)
			uniq = dedupeInts(uniq)
			quoteWarnings = append(quoteWarnings, QuoteWarning{Symbol: sym, Count: len(list), Lines: uniq})
		}
	}
	return scanResult{SymbolLines: symbolLines, Matched: matched, Unbalanced: unbalanced, QuoteWarnings: quoteWarnings, TagMatched: tagMatched}, nil
}

type tagItem struct {
	name string
	line int
	col  int
}

func formatAggregate(symbolLines map[string][]int) string {
	order := []string{"{", "}", "(", ")", "[", "]"}
	var lines []string
	for _, sym := range order {
		list := symbolLines[sym]
		if len(list) == 0 {
			lines = append(lines, fmt.Sprintf("%s\t(无)", sym))
			continue
		}
		parts := make([]string, len(list))
		for i, n := range list {
			parts[i] = fmt.Sprint(n)
		}
		lines = append(lines, fmt.Sprintf("%s\t%s", sym, strings.Join(parts, " ")))
	}
	return strings.Join(lines, "\n")
}

func formatTreeInner(matched []MatchedPair, emptyMsg, header string) string {
	if len(matched) == 0 {
		return emptyMsg
	}
	var rows []string
	for _, m := range matched {
		indent := strings.Repeat("  ", min(m.Depth-1, 10))
		rows = append(rows, fmt.Sprintf("%d\t%s%s\t%d\t%d", m.Depth, indent, m.Symbol, m.OpenLine, m.CloseLine))
	}
	return header + "\n" + strings.Join(rows, "\n")
}

func formatTree(matched []MatchedPair) string {
	return formatTreeInner(matched, "(无匹配的括号对)", "depth\tsymbol\tline\tpair_line")
}

func formatTagTree(matched []MatchedPair) string {
	return formatTreeInner(matched, "(无匹配的标签)", "depth\ttag\topen\tclose")
}

func dedupeInts(in []int) []int {
	if len(in) == 0 {
		return in
	}
	out := []int{in[0]}
	for _, n := range in[1:] {
		if n != out[len(out)-1] {
			out = append(out, n)
		}
	}
	return out
}

func readFullTag(chars []byte, start int) (string, int, bool) {
	inQ := false
	var qChar byte
	for i := start + 1; i < len(chars); i++ {
		sc := chars[i]
		if !inQ {
			if sc == '"' || sc == '\'' {
				inQ = true
				qChar = sc
			} else if sc == '>' {
				return string(chars[start : i+1]), i, true
			} else if sc == '<' {
				return "", start, false
			}
		} else if sc == qChar {
			inQ = false
		}
	}
	return "", start, false
}

func extractTagName(fullTag string, isClose bool) string {
	s := fullTag
	if isClose {
		if !strings.HasPrefix(s, "</") {
			return ""
		}
		s = strings.TrimPrefix(s, "</")
	} else {
		if !strings.HasPrefix(s, "<") {
			return ""
		}
		s = strings.TrimPrefix(s, "<")
	}
	end := 0
	for end < len(s) {
		ch := s[end]
		if ch == '>' || ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			break
		}
		end++
	}
	return s[:end]
}
