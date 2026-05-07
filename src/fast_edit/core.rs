use std::fs;
use std::io;
use std::path::Path;
use std::sync::atomic::{AtomicU64, Ordering};

static TMP_COUNTER: AtomicU64 = AtomicU64::new(0);

/// 读取文件内容并按行分割，保留行尾换行符
pub(crate) fn read_lines(filepath: &str) -> io::Result<(Vec<String>, String)> {
    let content = fs::read_to_string(Path::new(filepath))?;
    let le = detect_line_ending(&content);
    let lines: Vec<String> = content.lines().map(|l| format!("{}{}", l, le)).collect();
    Ok((lines, le.to_string()))
}

pub(crate) fn detect_line_ending(content: &str) -> &str {
    let crlf = content.matches("\r\n").count();
    if crlf >= content.lines().count() / 2 { "\r\n" } else { "\n" }
}

/// 原子写入：先写临时文件再 rename
pub(crate) fn write_file_atomic(filepath: &str, content: &str) -> io::Result<()> {
    let abs = Path::new(filepath);
    let parent = abs.parent().unwrap_or(Path::new("."));
    let stem = abs
        .file_stem()
        .and_then(|s| s.to_str())
        .unwrap_or("tmp");
    let counter = TMP_COUNTER.fetch_add(1, Ordering::Relaxed);
    let tmp_name = format!(".fe-{}-{}.tmp", stem, counter);
    let tmp_path = parent.join(&tmp_name);

    fs::write(&tmp_path, content)?;
    fs::rename(&tmp_path, abs)?;
    Ok(())
}

/// 将 \n \t 转义字符串还原为实际字符
pub(crate) fn parse_content(text: &str) -> String {
    let mut result = String::with_capacity(text.len());
    let mut chars = text.chars();
    while let Some(ch) = chars.next() {
        if ch == '\\' {
            match chars.next() {
                Some('n') => result.push('\n'),
                Some('t') => result.push('\t'),
                Some('r') => result.push('\r'),
                Some(c) => { result.push('\\'); result.push(c); }
                None => result.push('\\'),
            }
        } else {
            result.push(ch);
        }
    }
    result
}

/// 将内容解析并转换为带行尾符的行列表（共享管道）
/// 步骤：parse(如果 raw=false) → split → trim \r → 追加行尾符 → 去除尾部空行
pub(crate) fn prepare_content_lines(
    content: &str,
    line_ending: &str,
    raw: bool,
) -> Vec<String> {
    let parsed = if raw {
        content.to_string()
    } else {
        parse_content(content)
    };

    if parsed.is_empty() {
        return Vec::new();
    }

    let mut lines: Vec<String> = parsed
        .split('\n')
        .map(|l| format!("{}{}", l.trim_end_matches('\r'), line_ending))
        .collect();

    // 去掉末尾多余的空行
    while lines.last().map_or(false, |l| l.trim().is_empty()) {
        lines.pop();
    }

    lines
}

/// 快速符号闭合检查
pub(crate) fn quick_balance_check(content: &str) -> String {
    let mut curly: i32 = 0;
    let mut square: i32 = 0;
    let mut paren: i32 = 0;
    for ch in content.chars() {
        match ch {
            '{' => curly += 1,
            '}' => curly -= 1,
            '[' => square += 1,
            ']' => square -= 1,
            '(' => paren += 1,
            ')' => paren -= 1,
            _ => {}
        }
    }
    let mut errors: Vec<String> = Vec::new();
    if curly != 0 { errors.push(format!("{{}} 差 {} 个", curly.abs())); }
    if square != 0 { errors.push(format!("[] 差 {} 个", square.abs())); }
    if paren != 0 { errors.push(format!("() 差 {} 个", paren.abs())); }
    if errors.is_empty() {
        "符号闭合快速检查：OK".to_string()
    } else {
        format!("符号闭合快速检查：Error ({})", errors.join("; "))
    }
}

/// 构建修改前后对比文本
pub(crate) fn build_diff(
    before: &[String],
    after: &[String],
    base_line: usize,
    format: &str,
) -> String {
    if format == "diff" {
        let mut out = String::new();
        out.push_str(&format!(
            "@@ -{},{} +{},{} @@\n",
            base_line, before.len(), base_line, after.len()
        ));
        let max_len = std::cmp::max(before.len(), after.len());
        for i in 0..max_len {
            let b = before.get(i).map(|s| s.trim_end_matches('\n').trim_end_matches('\r'));
            let a = after.get(i).map(|s| s.trim_end_matches('\n').trim_end_matches('\r'));
            match (b, a) {
                (Some(b), Some(a)) if b == a => {
                    out.push_str(&format!(" {}\n", b));
                }
                (Some(b), _) => {
                    out.push_str(&format!("-{}\n", b));
                    if let Some(a) = a {
                        out.push_str(&format!("+{}\n", a));
                    }
                }
                (None, Some(a)) => {
                    out.push_str(&format!("+{}\n", a));
                }
                _ => {}
            }
        }
        if out.ends_with('\n') {
            out.pop();
        }
        out
    } else {
        let before_end = base_line + before.len() - 1;
        let after_end = base_line + after.len() - 1;
        let mut out = format!("--- 修改前（行 {}-{}）---\n", base_line, before_end);
        for (i, l) in before.iter().enumerate() {
            out.push_str(&format!("{}\t{}\n", base_line + i, l.trim_end_matches('\n').trim_end_matches('\r')));
        }
        out.push_str(&format!("\n+++ 修改后（行 {}-{}）+++\n", base_line, after_end));
        for (i, l) in after.iter().enumerate() {
            out.push_str(&format!("{}\t{}\n", base_line + i, l.trim_end_matches('\n').trim_end_matches('\r')));
        }
        if out.ends_with('\n') {
            out.pop();
        }
        out
    }
}
