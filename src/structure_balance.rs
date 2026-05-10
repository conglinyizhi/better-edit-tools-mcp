use crate::error::{EditError, EditResult};
use std::collections::{HashMap, HashSet};
use std::fs;
use std::path::Path;

// ── Data structures ──

#[derive(Debug, serde::Serialize)]
pub struct MatchedPair {
    symbol: String,
    open_line: usize,
    close_line: usize,
    depth: usize,
}

#[derive(Debug, serde::Serialize)]
pub struct UnbalancedItem {
    symbol: String,
    line: usize,
    expected: String,
}

#[derive(Debug, serde::Serialize)]
pub struct QuoteWarning {
    symbol: String,
    count: usize,
    lines: Vec<usize>,
}

struct ScanResult {
    symbol_lines: HashMap<String, Vec<usize>>,
    matched: Vec<MatchedPair>,
    unbalanced: Vec<UnbalancedItem>,
    quote_warnings: Vec<QuoteWarning>,
    tag_matched: Vec<MatchedPair>,
}

// ── HTML void elements ──

fn void_elements() -> HashSet<&'static str> {
    [
        "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param",
        "source", "track", "wbr",
    ]
    .iter()
    .copied()
    .collect()
}

fn is_ident_start(ch: char) -> bool {
    ch == '_' || ch == '$' || ch.is_ascii_alphabetic()
}

fn is_ident_continue(ch: char) -> bool {
    is_ident_start(ch) || ch.is_ascii_digit()
}

fn regex_keyword(word: &str) -> bool {
    matches!(
        word,
        "return"
            | "throw"
            | "case"
            | "else"
            | "do"
            | "typeof"
            | "instanceof"
            | "in"
            | "new"
            | "void"
            | "delete"
            | "yield"
            | "await"
    )
}

fn regex_can_start_after(ch: char) -> bool {
    matches!(
        ch,
        '(' | '['
            | '{'
            | ','
            | ';'
            | ':'
            | '='
            | '!'
            | '?'
            | '+'
            | '-'
            | '*'
            | '%'
            | '&'
            | '|'
            | '^'
            | '~'
            | '<'
            | '>'
    )
}

fn scan_file(filepath: &str) -> EditResult<ScanResult> {
    let content =
        fs::read_to_string(Path::new(filepath)).map_err(|e| EditError::read_path(filepath, e))?;

    let lines: Vec<&str> = content.split('\n').collect();

    let mut symbol_lines: HashMap<String, Vec<usize>> = HashMap::new();
    for ch in ["{", "}", "[", "]", "(", ")"] {
        symbol_lines.insert(ch.to_string(), Vec::new());
    }

    let mut stack: Vec<(String, usize)> = Vec::new();
    let mut matched: Vec<MatchedPair> = Vec::new();
    let mut unbalanced: Vec<UnbalancedItem> = Vec::new();
    let mut quote_lines: HashMap<String, Vec<usize>> = HashMap::new();
    quote_lines.insert("\"".to_string(), Vec::new());
    quote_lines.insert("'".to_string(), Vec::new());

    let mut tag_stack: Vec<(String, usize)> = Vec::new();
    let mut tag_matched: Vec<MatchedPair> = Vec::new();
    let void_elts = void_elements();

    #[derive(Clone, Copy, Debug, PartialEq, Eq)]
    enum Mode {
        Code,
        LineComment,
        BlockComment,
        String(char),
        Template,
        TemplateExpr { brace_depth: usize },
        Regex { in_class: bool },
    }

    let mut modes: Vec<Mode> = vec![Mode::Code];
    let mut escape_next = false;
    let mut regex_can_start = true;
    let mut pending_ident = String::new();

    let opens: HashSet<char> = ['{', '[', '('].iter().copied().collect();
    let closes: HashMap<char, &str> = [('}', "{"), (']', "["), (')', "(")]
        .iter()
        .copied()
        .collect();
    let pair_of: HashMap<&str, &str> = [
        ("{", "}"),
        ("}", "{"),
        ("[", "]"),
        ("]", "["),
        ("(", ")"),
        (")", "("),
    ]
    .iter()
    .copied()
    .collect();

    for (i, line) in lines.iter().enumerate() {
        let line_num = i + 1;
        let chars: Vec<char> = line.chars().collect();
        let mut col = 0;

        while col < chars.len() {
            let ch = chars[col];
            let next = if col + 1 < chars.len() {
                Some(chars[col + 1])
            } else {
                None
            };

            if escape_next {
                escape_next = false;
                col += 1;
                continue;
            }

            match *modes.last().unwrap() {
                Mode::LineComment => {
                    break;
                }
                Mode::BlockComment => {
                    if ch == '*' && next == Some('/') {
                        modes.pop();
                        col += 2;
                    } else {
                        col += 1;
                    }
                    continue;
                }
                Mode::String(quote) => {
                    if ch == '\\' {
                        escape_next = true;
                        col += 1;
                        continue;
                    }
                    if ch == quote {
                        modes.pop();
                        if quote == '"' {
                            quote_lines.get_mut("\"").unwrap().push(line_num);
                        } else {
                            quote_lines.get_mut("'").unwrap().push(line_num);
                        }
                        regex_can_start = false;
                    }
                    col += 1;
                    continue;
                }
                Mode::Template => {
                    if ch == '\\' {
                        escape_next = true;
                        col += 1;
                        continue;
                    }
                    if ch == '`' {
                        modes.pop();
                        regex_can_start = false;
                        col += 1;
                        continue;
                    }
                    if ch == '$' && next == Some('{') {
                        let ch_str = "{".to_string();
                        if let Some(list) = symbol_lines.get_mut(&ch_str) {
                            list.push(line_num);
                        }
                        stack.push((ch_str, line_num));
                        modes.push(Mode::TemplateExpr { brace_depth: 1 });
                        regex_can_start = true;
                        col += 2;
                        continue;
                    }
                    col += 1;
                    continue;
                }
                Mode::Regex { in_class } => {
                    if ch == '\\' {
                        escape_next = true;
                        col += 1;
                        continue;
                    }
                    if ch == '[' {
                        if let Some(Mode::Regex { in_class }) = modes.last_mut() {
                            *in_class = true;
                        }
                        col += 1;
                        continue;
                    }
                    if ch == ']' {
                        if let Some(Mode::Regex { in_class }) = modes.last_mut() {
                            *in_class = false;
                        }
                        col += 1;
                        continue;
                    }
                    if ch == '/' && !in_class {
                        modes.pop();
                        regex_can_start = false;
                    }
                    col += 1;
                    continue;
                }
                Mode::Code | Mode::TemplateExpr { .. } => {}
            }

            if is_ident_continue(ch) {
                if pending_ident.is_empty() && is_ident_start(ch) {
                    pending_ident.push(ch);
                    col += 1;
                    continue;
                }
                if !pending_ident.is_empty() {
                    pending_ident.push(ch);
                    col += 1;
                    continue;
                }
            }

            if !pending_ident.is_empty() {
                regex_can_start = regex_keyword(&pending_ident);
                pending_ident.clear();
            }

            if ch == '/' && next == Some('/') {
                modes.push(Mode::LineComment);
                col += 2;
                continue;
            }
            if ch == '/' && next == Some('*') {
                modes.push(Mode::BlockComment);
                col += 2;
                continue;
            }

            if ch == '`' {
                modes.push(Mode::Template);
                regex_can_start = true;
                col += 1;
                continue;
            }
            if ch == '"' || ch == '\'' {
                modes.push(Mode::String(ch));
                if ch == '"' {
                    quote_lines.get_mut("\"").unwrap().push(line_num);
                } else {
                    quote_lines.get_mut("'").unwrap().push(line_num);
                }
                regex_can_start = false;
                col += 1;
                continue;
            }

            if ch == '/'
                && regex_can_start
                && let Some(n) = next
                && !matches!(n, '/' | '*' | '\n' | '\r')
            {
                modes.push(Mode::Regex { in_class: false });
                regex_can_start = false;
                col += 1;
                continue;
            }

            if ch == '<' {
                let mut in_q = false;
                let mut q_char = ' ';
                let mut tag_end = None;
                let mut s = col + 1;
                while s < chars.len() {
                    let sc = chars[s];
                    if !in_q {
                        if sc == '"' || sc == '\'' {
                            in_q = true;
                            q_char = sc;
                        } else if sc == '>' {
                            tag_end = Some(s);
                            break;
                        } else if sc == '<' {
                            break;
                        }
                    } else if sc == q_char {
                        in_q = false;
                    }
                    s += 1;
                }

                let tag_end = match tag_end {
                    Some(e) => e,
                    None => {
                        col += 1;
                        continue;
                    }
                };

                let full_tag: String = chars[col..=tag_end].iter().collect();
                col = tag_end + 1;

                if full_tag.starts_with("<!") || full_tag.starts_with("<?") {
                    continue;
                }

                if full_tag.starts_with("</") {
                    if let Some(name_match) = extract_tag_name(&full_tag, true) {
                        let tag_name = name_match.to_lowercase();
                        if let Some((last_name, last_line)) = tag_stack.last() {
                            if *last_name == tag_name {
                                tag_matched.push(MatchedPair {
                                    symbol: format!("<{}>", tag_name),
                                    open_line: *last_line,
                                    close_line: line_num,
                                    depth: tag_stack.len(),
                                });
                                tag_stack.pop();
                            } else {
                                unbalanced.push(UnbalancedItem {
                                    symbol: format!("</{}>", tag_name),
                                    line: line_num,
                                    expected: format!("</{}>", last_name),
                                });
                            }
                        } else {
                            unbalanced.push(UnbalancedItem {
                                symbol: format!("</{}>", tag_name),
                                line: line_num,
                                expected: "(无对应开标签)".to_string(),
                            });
                        }
                    }
                    continue;
                }

                if let Some(tag_name) = extract_tag_name(&full_tag, false) {
                    let tag_name = tag_name.to_lowercase();
                    let trimmed = full_tag.trim_end().to_string();
                    let is_self_closing = trimmed.ends_with("/>");
                    if is_self_closing {
                        continue;
                    }
                    if void_elts.contains(tag_name.as_str()) {
                        continue;
                    }
                    tag_stack.push((tag_name.to_string(), line_num));
                }
                continue;
            }

            let in_template_expr = matches!(modes.last(), Some(Mode::TemplateExpr { .. }));
            let mut close_template_expr = false;

            let ch_str = ch.to_string();
            if symbol_lines.contains_key(&ch_str) {
                symbol_lines.get_mut(&ch_str).unwrap().push(line_num);
            }

            if opens.contains(&ch) {
                stack.push((ch_str.clone(), line_num));
                if in_template_expr
                    && let Some(Mode::TemplateExpr { brace_depth }) = modes.last_mut()
                    && ch == '{'
                {
                    *brace_depth += 1;
                }
            } else if let Some(&expected_open) = closes.get(&ch) {
                if let Some((last_sym, last_line)) = stack.last() {
                    if last_sym == expected_open {
                        matched.push(MatchedPair {
                            symbol: format!("{}{}", expected_open, ch),
                            open_line: *last_line,
                            close_line: line_num,
                            depth: stack.len(),
                        });
                        stack.pop();
                    } else {
                        unbalanced.push(UnbalancedItem {
                            symbol: ch_str.clone(),
                            line: line_num,
                            expected: pair_of.get(ch_str.as_str()).unwrap_or(&"").to_string(),
                        });
                    }
                } else {
                    unbalanced.push(UnbalancedItem {
                        symbol: ch_str.clone(),
                        line: line_num,
                        expected: pair_of.get(ch_str.as_str()).unwrap_or(&"").to_string(),
                    });
                }

                if in_template_expr
                    && let Some(Mode::TemplateExpr { brace_depth }) = modes.last_mut()
                    && ch == '}'
                {
                    if *brace_depth > 1 {
                        *brace_depth -= 1;
                    } else {
                        close_template_expr = true;
                    }
                }
            }

            if close_template_expr {
                modes.pop();
                regex_can_start = true;
            }

            if !ch.is_whitespace() {
                regex_can_start = regex_can_start_after(ch);
            }

            col += 1;
        }

        if !pending_ident.is_empty() {
            regex_can_start = regex_keyword(&pending_ident);
            pending_ident.clear();
        }

        if matches!(modes.last(), Some(Mode::LineComment)) {
            modes.pop();
        }
    }

    for (sym, ln) in &stack {
        unbalanced.push(UnbalancedItem {
            symbol: sym.clone(),
            line: *ln,
            expected: pair_of.get(sym.as_str()).unwrap_or(&"").to_string(),
        });
    }

    for (name, ln) in &tag_stack {
        unbalanced.push(UnbalancedItem {
            symbol: format!("<{}>", name),
            line: *ln,
            expected: format!("</{}>", name),
        });
    }

    let mut quote_warnings = Vec::new();
    for (q, list) in &quote_lines {
        if list.len() % 2 != 0 {
            let mut unique: Vec<usize> = {
                let mut v = list.clone();
                v.sort();
                v.dedup();
                v
            };
            unique.sort();
            quote_warnings.push(QuoteWarning {
                symbol: q.clone(),
                count: list.len(),
                lines: unique,
            });
        }
    }

    Ok(ScanResult {
        symbol_lines,
        matched,
        unbalanced,
        quote_warnings,
        tag_matched,
    })
}

fn extract_tag_name(full_tag: &str, is_close: bool) -> Option<&str> {
    if is_close {
        // </tag ...>
        let s = full_tag.strip_prefix("</")?;
        let end = s.find(|c: char| c.is_whitespace() || c == '>')?;
        Some(&s[..end])
    } else {
        // <tag ...>
        let s = full_tag.strip_prefix('<')?;
        let end = s.find(|c: char| c.is_whitespace() || c == '>')?;
        Some(&s[..end])
    }
}

// ── Format helpers ──

fn format_aggregate(symbol_lines: &HashMap<String, Vec<usize>>) -> String {
    let order = ["{", "}", "(", ")", "[", "]"];
    let mut lines = Vec::new();
    for sym in &order {
        if let Some(list) = symbol_lines.get(*sym) {
            if list.is_empty() {
                lines.push(format!("{}\t(无)", sym));
            } else {
                let nums: Vec<String> = list.iter().map(|n| n.to_string()).collect();
                lines.push(format!("{}\t{}", sym, nums.join(" ")));
            }
        }
    }
    lines.join("\n")
}

fn format_tree_inner(matched: &[MatchedPair], empty_msg: &str, header: &str) -> String {
    if matched.is_empty() {
        return empty_msg.to_string();
    }
    let rows: Vec<String> = matched
        .iter()
        .map(|m| {
            let indent = "  ".repeat(std::cmp::min(m.depth.saturating_sub(1), 10));
            format!(
                "{}\t{}{}\t{}\t{}",
                m.depth, indent, m.symbol, m.open_line, m.close_line
            )
        })
        .collect();
    let mut result = header.to_string();
    for r in rows {
        result.push('\n');
        result.push_str(&r);
    }
    result
}

fn format_tree(matched: &[MatchedPair]) -> String {
    format_tree_inner(
        matched,
        "(无匹配的括号对)",
        "depth\tsymbol\tline\tpair_line",
    )
}

fn format_tag_tree(tag_matched: &[MatchedPair]) -> String {
    format_tree_inner(tag_matched, "(无匹配的标签)", "depth\ttag\topen\tclose")
}

fn format_unbalanced(
    unbalanced: &[UnbalancedItem],
    quote_warnings: &[QuoteWarning],
) -> serde_json::Value {
    let mut result = serde_json::Map::new();
    if !unbalanced.is_empty() {
        result.insert(
            "unbalanced".to_string(),
            serde_json::to_value(unbalanced).unwrap_or(serde_json::Value::Null),
        );
    }
    if !quote_warnings.is_empty() {
        result.insert(
            "quote_warnings".to_string(),
            serde_json::to_value(quote_warnings).unwrap_or(serde_json::Value::Null),
        );
    }
    if unbalanced.is_empty() && quote_warnings.is_empty() {
        result.insert(
            "status".to_string(),
            serde_json::Value::String("all balanced".to_string()),
        );
    }
    serde_json::Value::Object(result)
}

// ── Public API ──

pub fn check_structure_balance(file: &str, mode: &str) -> Result<String, String> {
    if !["aggregate", "unbalanced", "tree"].contains(&mode) {
        return Err(format!(
            "未知 mode: \"{}\"，支持: aggregate, unbalanced, tree",
            mode
        ));
    }

    let result = scan_file(file).map_err(|e| e.to_string())?;

    let output = match mode {
        "aggregate" => {
            let tag_info = if result.tag_matched.is_empty() {
                "(无标签)".to_string()
            } else {
                result
                    .tag_matched
                    .iter()
                    .map(|t| format!("{}  {}:{}", t.symbol, t.open_line, t.close_line))
                    .collect::<Vec<_>>()
                    .join("\n")
            };
            let quote_info = if result.quote_warnings.is_empty() {
                "(引号成对)".to_string()
            } else {
                result
                    .quote_warnings
                    .iter()
                    .map(|q| {
                        format!(
                            "{} 奇数个（共 {} 个）位于行 {}",
                            q.symbol,
                            q.count,
                            q.lines
                                .iter()
                                .map(|n| n.to_string())
                                .collect::<Vec<_>>()
                                .join(", ")
                        )
                    })
                    .collect::<Vec<_>>()
                    .join("\n")
            };
            serde_json::json!({
                "mode": "aggregate",
                "symbols": format_aggregate(&result.symbol_lines),
                "tags": tag_info,
                "quote_warnings": quote_info,
            })
        }
        "tree" => {
            serde_json::json!({
                "mode": "tree",
                "tree": format_tree(&result.matched),
                "tag_tree": if result.tag_matched.is_empty() {
                    serde_json::Value::Null
                } else {
                    serde_json::Value::String(format_tag_tree(&result.tag_matched))
                },
                "unbalanced": if result.unbalanced.is_empty() {
                    serde_json::Value::Null
                } else {
                    serde_json::to_value(&result.unbalanced).unwrap_or(serde_json::Value::Null)
                },
                "quote_warnings": if result.quote_warnings.is_empty() {
                    serde_json::Value::Null
                } else {
                    serde_json::to_value(&result.quote_warnings).unwrap_or(serde_json::Value::Null)
                },
            })
        }
        _ => {
            // unbalanced (default)
            let mut base = match format_unbalanced(&result.unbalanced, &result.quote_warnings) {
                serde_json::Value::Object(map) => map,
                _ => serde_json::Map::new(),
            };
            base.insert(
                "mode".to_string(),
                serde_json::Value::String("unbalanced".to_string()),
            );
            serde_json::Value::Object(base)
        }
    };

    serde_json::to_string_pretty(&output).map_err(|e| format!("JSON 序列化失败: {}", e))
}
