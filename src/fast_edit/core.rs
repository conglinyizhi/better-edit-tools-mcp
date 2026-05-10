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
    if crlf > content.lines().count() / 2 {
        "\r\n"
    } else {
        "\n"
    }
}

/// 原子写入：先写临时文件再 rename
pub(crate) fn write_file_atomic(filepath: &str, content: &str) -> io::Result<()> {
    let abs = Path::new(filepath);
    let parent = abs.parent().unwrap_or(Path::new("."));
    let stem = abs.file_stem().and_then(|s| s.to_str()).unwrap_or("tmp");
    let counter = TMP_COUNTER.fetch_add(1, Ordering::Relaxed);
    let tmp_name = format!(".fe-{}-{}.tmp", stem, counter);
    let tmp_path = parent.join(&tmp_name);

    fs::write(&tmp_path, content)?;
    fs::rename(&tmp_path, abs)?;
    Ok(())
}

/// 多文件原子写入：尽量保证要么全部成功，要么回滚到原始状态
pub(crate) fn write_files_atomic(writes: &[(String, String)]) -> io::Result<()> {
    if writes.is_empty() {
        return Ok(());
    }

    let mut temp_paths: Vec<(String, std::path::PathBuf)> = Vec::new();
    let mut backup_paths: Vec<(String, Option<std::path::PathBuf>)> = Vec::new();

    for (filepath, content) in writes {
        let abs = Path::new(filepath);
        let parent = abs.parent().unwrap_or(Path::new("."));
        let stem = abs.file_stem().and_then(|s| s.to_str()).unwrap_or("tmp");
        let counter = TMP_COUNTER.fetch_add(1, Ordering::Relaxed);
        let tmp_name = format!(".fe-{}-{}.tmp", stem, counter);
        let tmp_path = parent.join(&tmp_name);
        fs::write(&tmp_path, content)?;
        temp_paths.push((filepath.clone(), tmp_path));

        if abs.exists() {
            let backup_name = format!(".fe-{}-{}.bak", stem, counter);
            let backup_path = parent.join(&backup_name);
            fs::copy(abs, &backup_path)?;
            backup_paths.push((filepath.clone(), Some(backup_path)));
        } else {
            backup_paths.push((filepath.clone(), None));
        }
    }

    let mut committed: Vec<String> = Vec::new();
    for (filepath, tmp_path) in &temp_paths {
        let dest = Path::new(filepath);
        if let Err(err) = fs::rename(tmp_path, dest) {
            // 尝试回滚已提交的文件
            for committed_file in committed.iter().rev() {
                if let Some((_, backup)) =
                    backup_paths.iter().find(|(path, _)| path == committed_file)
                {
                    match backup {
                        Some(backup_path) => {
                            let _ = fs::rename(backup_path, Path::new(committed_file));
                        }
                        None => {
                            let _ = fs::remove_file(Path::new(committed_file));
                        }
                    }
                }
            }
            // 清理剩余临时文件和备份
            for (_, pending_tmp) in &temp_paths {
                let _ = fs::remove_file(pending_tmp);
            }
            for (_, backup) in &backup_paths {
                if let Some(backup_path) = backup {
                    let _ = fs::remove_file(backup_path);
                }
            }
            return Err(err);
        }
        committed.push(filepath.clone());
    }

    for (_, backup) in &backup_paths {
        if let Some(backup_path) = backup {
            let _ = fs::remove_file(backup_path);
        }
    }
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
                Some(c) => {
                    result.push('\\');
                    result.push(c);
                }
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
pub(crate) fn prepare_content_lines(content: &str, line_ending: &str, raw: bool) -> Vec<String> {
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
    while lines.last().is_some_and(|l| l.trim().is_empty()) {
        lines.pop();
    }

    lines
}

/// 快速符号闭合检查（支持字符串/注释感知，避免误报）
pub(crate) fn quick_balance_check(content: &str) -> String {
    let mut curly: i32 = 0;
    let mut square: i32 = 0;
    let mut paren: i32 = 0;
    let mut in_string = false;
    let mut string_char = ' ';
    let mut escape = false;
    let mut in_line_comment = false;
    let mut in_block_comment = false;
    let chars: Vec<char> = content.chars().collect();
    let mut i = 0;
    while i < chars.len() {
        let ch = chars[i];
        let next = chars.get(i + 1).copied();

        // 转义：跳过下一个字符
        if escape {
            escape = false;
            i += 1;
            continue;
        }

        if ch == '\\' && in_string {
            escape = true;
            i += 1;
            continue;
        }

        // 行注释 //
        if !in_string && !in_block_comment && ch == '/' && next == Some('/') {
            in_line_comment = true;
            i += 1;
            continue;
        }
        // 换行结束行注释
        if in_line_comment && ch == '\n' {
            in_line_comment = false;
            i += 1;
            continue;
        }
        if in_line_comment {
            i += 1;
            continue;
        }

        // 块注释 /* */
        if !in_string && !in_block_comment && ch == '/' && next == Some('*') {
            in_block_comment = true;
            i += 2;
            continue;
        }
        if in_block_comment && ch == '*' && next == Some('/') {
            in_block_comment = false;
            i += 2;
            continue;
        }
        if in_block_comment {
            i += 1;
            continue;
        }

        // 字符串开始/结束
        if (ch == '"' || ch == '\'' || ch == '`') && !in_string {
            in_string = true;
            string_char = ch;
            i += 1;
            continue;
        }
        if in_string && ch == string_char {
            in_string = false;
            i += 1;
            continue;
        }
        if in_string {
            i += 1;
            continue;
        }

        // 花括号/方括号/圆括号计数
        match ch {
            '{' => curly += 1,
            '}' => curly -= 1,
            '[' => square += 1,
            ']' => square -= 1,
            '(' => paren += 1,
            ')' => paren -= 1,
            _ => {}
        }
        i += 1;
    }

    let mut errors: Vec<String> = Vec::new();
    if curly != 0 {
        errors.push(format!("{{}} 差 {} 个", curly.abs()));
    }
    if square != 0 {
        errors.push(format!("[] 差 {} 个", square.abs()));
    }
    if paren != 0 {
        errors.push(format!("() 差 {} 个", paren.abs()));
    }
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
            base_line,
            before.len(),
            base_line,
            after.len()
        ));
        let max_len = std::cmp::max(before.len(), after.len());
        for i in 0..max_len {
            let b = before
                .get(i)
                .map(|s| s.trim_end_matches('\n').trim_end_matches('\r'));
            let a = after
                .get(i)
                .map(|s| s.trim_end_matches('\n').trim_end_matches('\r'));
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
            out.push_str(&format!(
                "{}\t{}\n",
                base_line + i,
                l.trim_end_matches('\n').trim_end_matches('\r')
            ));
        }
        out.push_str(&format!(
            "\n+++ 修改后（行 {}-{}）+++\n",
            base_line, after_end
        ));
        for (i, l) in after.iter().enumerate() {
            out.push_str(&format!(
                "{}\t{}\n",
                base_line + i,
                l.trim_end_matches('\n').trim_end_matches('\r')
            ));
        }
        if out.ends_with('\n') {
            out.pop();
        }
        out
    }
}
