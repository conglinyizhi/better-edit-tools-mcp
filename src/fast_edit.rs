use std::fs;
use std::io;
use std::path::Path;
use std::sync::atomic::{AtomicU64, Ordering};

// ── Helpers ──

static TMP_COUNTER: AtomicU64 = AtomicU64::new(0);

/// 读取文件内容并按行分割，保留行尾换行符
fn read_lines(filepath: &str) -> io::Result<(Vec<String>, String)> {
    let content = fs::read_to_string(Path::new(filepath))?;
    let le = detect_line_ending(&content);
    let lines: Vec<String> = content.lines().map(|l| format!("{}{}", l, le)).collect();
    Ok((lines, le.to_string()))
}

fn detect_line_ending(content: &str) -> &str {
    let crlf = content.matches("\r\n").count();
    if crlf > content.lines().count() / 2 { "\r\n" } else { "\n" }
}

/// 原子写入：先写临时文件再 rename
fn write_file_atomic(filepath: &str, content: &str) -> io::Result<()> {
    let abs = Path::new(filepath);
    let parent = abs.parent().unwrap_or(Path::new("."));
    let stem = abs
        .file_stem()
        .and_then(|s| s.to_str())
        .unwrap_or("tmp");
    let counter = TMP_COUNTER.fetch_add(1, Ordering::Relaxed);
    let tmp_name = format!(".fe-{}-{}.tmp", stem, counter);
    let tmp_path = parent.join(&tmp_name);

    // 写临时文件
    fs::write(&tmp_path, content)?;
    // rename 到目标
    fs::rename(&tmp_path, abs)?;
    Ok(())
}

/// 将 \n \t 转义字符串还原为实际字符
fn parse_content(text: &str) -> String {
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

/// 快速符号闭合检查
fn quick_balance_check(content: &str) -> String {
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
fn build_diff(
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
        // plain
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

// ── Operations ──

#[derive(serde::Serialize)]
pub struct ShowResult {
    status: String,
    file: String,
    start: usize,
    end: usize,
    total: usize,
    content: String,
}

#[derive(serde::Serialize)]
pub struct EditResult {
    status: String,
    file: String,
    removed: usize,
    added: usize,
    total: usize,
    diff: String,
    balance: String,
    affected: String,
}

#[derive(serde::Serialize)]
pub struct InsertResult {
    status: String,
    file: String,
    after: usize,
    added: usize,
    total: usize,
    diff: String,
    balance: String,
    affected: String,
}

#[derive(serde::Serialize)]
pub struct DeleteResult {
    status: String,
    file: String,
    total: usize,
    diff: String,
    balance: String,
    affected: String,
}

#[derive(serde::Serialize)]
pub struct BatchResult {
    status: String,
    files: usize,
    results: Vec<BatchFileResult>,
}

#[derive(serde::Serialize)]
pub struct BatchFileResult {
    file: String,
    edits: usize,
    total: usize,
}

#[derive(serde::Serialize)]
pub struct FunctionRangeResult {
    start: usize,
    end: usize,
}

/// 显示文件内容
pub fn op_show(filepath: &str, start: usize, end: Option<&str>) -> Result<ShowResult, String> {
    let (lines, _) = read_lines(filepath).map_err(|e| format!("读取文件失败: {}", e))?;
    let total = lines.len();
    let mut s = start.max(1);
    let e = match end {
        Some("auto") | None => {
            // 尝试找函数范围
            match op_function_range_raw(filepath, s) {
                Ok(r) => {
                    s = r.0;
                    r.1
                }
                Err(_) => {
                    let ctx_before = 5usize;
                    let ctx_after = 5usize;
                    let min_lines = 20usize;
                    let ctx_start = s.saturating_sub(ctx_before).max(1);
                    let mut ctx_end = (s + ctx_after).min(total);
                    if ctx_end - ctx_start + 1 < min_lines {
                        let extra = (min_lines - (ctx_end - ctx_start + 1) + 1) / 2;
                        ctx_end = total.min(ctx_end + extra);
                    }
                    s = ctx_start;
                    ctx_end
                }
            }
        }
        Some(v) => v.parse::<usize>().map_err(|_| "end 参数必须是数字或 'auto'".to_string())?,
    };
    let e = e.min(total);

    let content: String = lines[s - 1..e]
        .iter()
        .enumerate()
        .map(|(i, l)| format!("{}\t{}", s + i, l.trim_end_matches('\n').trim_end_matches('\r')))
        .collect::<Vec<_>>()
        .join("\n");

    Ok(ShowResult {
        status: "ok".to_string(),
        file: Path::new(filepath).to_string_lossy().to_string(),
        start: s,
        end: e,
        total,
        content,
    })
}

/// 替换文件内容
pub fn op_replace(filepath: &str, start: usize, end: usize, content: &str, raw: bool, format: &str) -> Result<EditResult, String> {
    let (mut lines, le) = read_lines(filepath).map_err(|e| format!("读取文件失败: {}", e))?;
    let total = lines.len();

    if start < 1 || start > total {
        return Err(format!("replace: start 超出范围 (1..{})", total));
    }
    if end < start || end > total {
        return Err(format!("replace: end 超出范围 ({}..{})", start, total));
    }

    const CTX: usize = 5;
    let before_start = start.saturating_sub(CTX).max(1);
    let before_end = total.min(end + CTX);
    let before_content: Vec<String> = lines[before_start - 1..before_end].to_vec();

    let nc = if raw { content.to_string() } else { parse_content(content) };
    let mut new_lines: Vec<String> = if nc.is_empty() {
        Vec::new()
    } else {
        nc.split('\n')
            .map(|l| format!("{}{}", l.trim_end_matches('\r'), le))
            .collect()
    };
    // 去掉末尾多余的空行（ts 版本的行为）
    while new_lines.last().map_or(false, |l| l.trim().is_empty()) {
        new_lines.pop();
    }
    // 确保最后一行后有换行
    if !new_lines.is_empty() && !new_lines.last().expect("已检查非空").ends_with('\n') {
        if let Some(last) = new_lines.last_mut() {
            last.push_str(&le);
        }
    }

    lines.splice(start - 1..end, new_lines.clone());
    let new_content = lines.concat();
    write_file_atomic(filepath, &new_content).map_err(|e| format!("写入文件失败: {}", e))?;

    // 修改后上下文
    let (after_lines, _) = read_lines(filepath).map_err(|e| format!("重新读取失败: {}", e))?;
    let delta = new_lines.len() as isize - (end as isize - start as isize + 1);
    let after_end = (before_end as isize + delta).max(0) as usize;
    let after_end = after_end.min(after_lines.len());
    let after_content: Vec<String> = after_lines[before_start - 1..after_end].to_vec();

    let diff = build_diff(&before_content, &after_content, before_start, format);
    let balance = quick_balance_check(&after_lines.concat());

    Ok(EditResult {
        status: "ok".to_string(),
        file: Path::new(filepath).to_string_lossy().to_string(),
        removed: end - start + 1,
        added: new_lines.len(),
        total: after_lines.len(),
        diff,
        balance,
        affected: format!("行 {}-{}（当前共 {} 行）", before_start, after_end, after_lines.len()),
    })
}

/// 在指定行后插入内容，after=0 表示文件开头
pub fn op_insert(filepath: &str, after: usize, content: &str, raw: bool, format: &str) -> Result<InsertResult, String> {
    let (mut lines, le) = read_lines(filepath).map_err(|e| format!("读取文件失败: {}", e))?;
    let total = lines.len();

    if after > total {
        return Err(format!("insert: line ({}) 超出范围 (0..{})", after, total));
    }

    const CTX: usize = 5;
    let before_start = (after as isize - CTX as isize + 1).max(1) as usize;
    let before_end = total.min(after + CTX);
    let before_content: Vec<String> = lines[before_start - 1..before_end].to_vec();

    let nc = if raw { content.to_string() } else { parse_content(content) };
    let mut new_lines: Vec<String> = if nc.is_empty() {
        Vec::new()
    } else {
        nc.split('\n')
            .map(|l| format!("{}{}", l.trim_end_matches('\r'), le))
            .collect()
    };
    // 清理空的末尾行
    while new_lines.last().map_or(false, |l| l.trim().is_empty()) {
        new_lines.pop();
    }
    if !new_lines.is_empty() && !new_lines.last().expect("已检查非空").ends_with('\n') {
        if let Some(last) = new_lines.last_mut() {
            last.push_str(&le);
        }
    }

    let insert_pos = after; // after=0 表示插到开头
    let after_line = insert_pos;
    // 确保前一行有换行
    if after_line > 0 && after_line <= lines.len() {
        if !lines[after_line - 1].ends_with('\n') {
            lines[after_line - 1].push_str(&le);
        }
    }

    let mut result = Vec::with_capacity(lines.len() + new_lines.len());
    result.extend_from_slice(&lines[..after_line]);
    result.extend(new_lines.clone());
    result.extend_from_slice(&lines[after_line..]);

    let new_content = result.concat();
    write_file_atomic(filepath, &new_content).map_err(|e| format!("写入文件失败: {}", e))?;

    let (after_lines, _) = read_lines(filepath).map_err(|e| format!("重新读取失败: {}", e))?;
    let after_end = (before_end + new_lines.len()).min(after_lines.len());
    let after_content: Vec<String> = after_lines[before_start - 1..after_end].to_vec();

    let diff = build_diff(&before_content, &after_content, before_start, format);
    let balance = quick_balance_check(&after_lines.concat());

    Ok(InsertResult {
        status: "ok".to_string(),
        file: Path::new(filepath).to_string_lossy().to_string(),
        after: after_line,
        added: new_lines.len(),
        total: after_lines.len(),
        diff,
        balance,
        affected: format!("行 {}-{}（当前共 {} 行）", before_start, after_end, after_lines.len()),
    })
}

/// 删除行，支持单行、范围和批量行号
pub fn op_delete(
    filepath: &str,
    start: Option<usize>,
    end: Option<usize>,
    line: Option<usize>,
    lines_json: Option<&str>,
    format: &str,
) -> Result<DeleteResult, String> {
    let (mut file_lines, _) = read_lines(filepath).map_err(|e| format!("读取文件失败: {}", e))?;
    let total = file_lines.len();
    const CTX: usize = 5;

    if let Some(json) = lines_json {
        let nums: Vec<usize> = serde_json::from_str::<Vec<usize>>(json)
            .map_err(|e| format!("lines JSON 解析失败: {}", e))?;
        let valid: Vec<usize> = nums.into_iter().filter(|&n| n >= 1 && n <= total).collect();
        if valid.is_empty() {
            return Err(format!("delete: 所有行号均超出文件范围 (1..{})", total));
        }
        let min_del = *valid.iter().min().expect("valid 已保证非空");
        let max_del = *valid.iter().max().expect("valid 已保证非空");
        let before_start = min_del.saturating_sub(CTX).max(1);
        let before_end = total.min(max_del + CTX);
        let before_content: Vec<String> = file_lines[before_start - 1..before_end].to_vec();
        let to_delete: std::collections::HashSet<usize> = valid.iter().copied().collect();
        // 重新读取文件并过滤掉要删除的行
        let (orig_lines, _le) = read_lines(filepath).map_err(|e| format!("重新读取文件失败: {}", e))?;
        let filtered: Vec<String> = orig_lines
            .into_iter()
            .enumerate()
            .filter(|(i, _)| !to_delete.contains(&(i + 1)))
            .map(|(_, l)| l)
            .collect();
        let new_content = filtered.concat();
        write_file_atomic(filepath, &new_content).map_err(|e| format!("写入文件失败: {}", e))?;

        let (after_lines, _) = read_lines(filepath).map_err(|e| format!("重新读取失败: {}", e))?;
        let after_end = (before_end as isize - valid.len() as isize).max(0) as usize;
        let after_end = after_end.min(after_lines.len());
        let after_content: Vec<String> = after_lines[before_start - 1..after_end].to_vec();
        let diff = build_diff(&before_content, &after_content, before_start, format);
        let balance = quick_balance_check(&after_lines.concat());
        let tip = "注意：该工具修改方式激进，若不确定请及时重新读取源码文件".to_string();

        return Ok(DeleteResult {
            status: "ok".to_string(),
            file: Path::new(filepath).to_string_lossy().to_string(),
            total: after_lines.len(),
            diff,
            balance: format!("{}\n{}", balance, tip),
            affected: format!("行 {}-{}（当前共 {} 行）", before_start, after_end, after_lines.len()),
        });
    }

    let s = start.or(line).unwrap_or(1);
    let e = end.or(line).unwrap_or(s);

    if s < 1 || s > total {
        return Err(format!("delete: start ({}) 超出范围 (1..{})", s, total));
    }
    if e < s || e > total {
        return Err(format!("delete: end ({}) 超出范围 ({}..{})", e, s, total));
    }

    let before_start = s.saturating_sub(CTX).max(1);
    let before_end = total.min(e + CTX);
    let before_content: Vec<String> = file_lines[before_start - 1..before_end].to_vec();
    let deleted = e - s + 1;

    file_lines.splice(s - 1..e, std::iter::empty());
    let new_content = file_lines.concat();
    write_file_atomic(filepath, &new_content).map_err(|e| format!("写入文件失败: {}", e))?;

    let (after_lines, _) = read_lines(filepath).map_err(|e| format!("重新读取失败: {}", e))?;
    let after_end = (before_end as isize - deleted as isize).max(0) as usize;
    let after_end = after_end.min(after_lines.len());
    let after_content: Vec<String> = after_lines[before_start - 1..after_end].to_vec();
    let diff = build_diff(&before_content, &after_content, before_start, format);
    let balance = quick_balance_check(&after_lines.concat());
    let tip = "注意：该工具修改方式激进，若不确定请及时重新读取源码文件".to_string();

    Ok(DeleteResult {
        status: "ok".to_string(),
        file: Path::new(filepath).to_string_lossy().to_string(),
        total: after_lines.len(),
        diff,
        balance: format!("{}\n{}", balance, tip),
        affected: format!("行 {}-{}（当前共 {} 行）", before_start, after_end, after_lines.len()),
    })
}

/// 批量编辑
pub fn op_batch(spec: &str, _format: &str) -> Result<BatchResult, String> {
    let spec_val: serde_json::Value =
        serde_json::from_str(spec).map_err(|e| format!("batch spec JSON 解析失败: {}", e))?;

    let file_specs = match &spec_val {
        serde_json::Value::Array(arr) => {
            // 裸数组: 每个元素是 {file, edits}
            arr.iter().collect()
        }
        serde_json::Value::Object(map) => {
            if let Some(files) = map.get("files").and_then(|v| v.as_array()) {
                files.iter().collect()
            } else {
                vec![&spec_val] // 单文件格式 {file, edits}
            }
        }
        _ => return Err("batch: 不支持的 JSON 格式，需要数组或对象".to_string()),
    };

    let mut results = Vec::new();
    for fs in &file_specs {
        let filepath = fs
            .get("file")
            .and_then(|v| v.as_str())
            .ok_or_else(|| "batch: 缺少 \"file\" 字段".to_string())?;
        let edits = fs
            .get("edits")
            .and_then(|v| v.as_array())
            .ok_or_else(|| format!("batch: 缺少 \"edits\" 数组字段 (file: {})", filepath))?;
        if edits.is_empty() {
            return Err("batch: edits 数组为空".to_string());
        }

        let (mut lines, le) = read_lines(filepath).map_err(|e| format!("读取 {} 失败: {}", filepath, e))?;

        // 从后往前排序，避免行号偏移
        let mut sorted_edits: Vec<&serde_json::Value> = edits.iter().collect();
        sorted_edits.sort_by(|a, b| {
            let a_key = a
                .get("start")
                .or_else(|| a.get("line"))
                .and_then(|v| v.as_u64())
                .unwrap_or(0);
            let b_key = b
                .get("start")
                .or_else(|| b.get("line"))
                .and_then(|v| v.as_u64())
                .unwrap_or(0);
            b_key.cmp(&a_key)
        });

        for edit in &sorted_edits {
            let action = edit
                .get("action")
                .and_then(|v| v.as_str())
                .ok_or("batch: 缺少 action 字段".to_string())?;
            match action {
                "replace-lines" => {
                    let s = edit["start"].as_u64().ok_or("replace-lines: 缺少 start")? as usize;
                    let e = edit["end"].as_u64().ok_or("replace-lines: 缺少 end")? as usize;
                    if s < 1 || s > lines.len() {
                        return Err(format!("batch/replace: start ({}) 超出范围 (1..{})", s, lines.len()));
                    }
                    if e < s || e > lines.len() {
                        return Err(format!("batch/replace: end ({}) 超出范围 ({}..{})", e, s, lines.len()));
                    }
                    let raw_content = edit["content"].as_str().unwrap_or("");
                    let nc = raw_content
                        .split('\n')
                        .map(|l| format!("{}{}", l.trim_end_matches('\r'), le))
                        .collect::<Vec<_>>()
                        .join("");
                    let new_lines: Vec<String> = if nc.is_empty() {
                        Vec::new()
                    } else {
                        let mut nl: Vec<String> = nc
.split('\n')
.map(|l| format!("{}{}", l.trim_end_matches('\r'), le))
.collect();
                        // 确保最后一行有换行
                        if e < lines.len() && !nl.is_empty() && !nl.last().expect("已检查非空").ends_with('\n') {
                            if let Some(last) = nl.last_mut() {
                                last.push_str(&le);
                            }
                        }
                        // 去掉末尾多余空行
                        while nl.len() > 1 && nl.last().map_or(false, |l| l.trim().is_empty()) {
                            nl.pop();
                        }
                        nl
                    };
                    lines.splice(s - 1..e, new_lines);
                }
                "insert-after" => {
                    let ln = edit["line"].as_u64().ok_or("insert-after: 缺少 line")? as usize;
                    if ln > lines.len() {
                        return Err(format!("batch/insert: line ({}) 超出范围 (0..{})", ln, lines.len()));
                    }
                    let raw_content = edit["content"].as_str().unwrap_or("");
                    let mut new_lines: Vec<String> = raw_content
                        .split('\n')
                        .map(|l| format!("{}{}", l.trim_end_matches('\r'), le))
                        .collect();
                    while new_lines.last().map_or(false, |l| l.trim().is_empty()) {
                        new_lines.pop();
                    }
                    if ln > 0 && ln <= lines.len() && !lines[ln - 1].ends_with('\n') {
                        lines[ln - 1].push_str(&le);
                    }
                    let mut result = Vec::with_capacity(lines.len() + new_lines.len());
                    result.extend_from_slice(&lines[..ln]);
                    result.extend(new_lines);
                    result.extend_from_slice(&lines[ln..]);
                    lines = result;
                }
                "delete-lines" => {
                    let s = edit["start"].as_u64().ok_or("delete-lines: 缺少 start")? as usize;
                    let e = edit["end"].as_u64().ok_or("delete-lines: 缺少 end")? as usize;
                    if s < 1 || s > lines.len() {
                        return Err(format!("batch/delete: start ({}) 超出范围 (1..{})", s, lines.len()));
                    }
                    if e < s || e > lines.len() {
                        return Err(format!(
                            "batch/delete: end ({}) 超出范围 ({}..{})",
                            e, s, lines.len()
                        ));
                    }
                    lines.splice(s - 1..e, std::iter::empty());
                }
                _ => {
                    return Err(format!("batch: 未知操作 \"{}\"，支持: replace-lines, insert-after, delete-lines", action));
                }
            }
        }

        let new_content = lines.concat();
        write_file_atomic(filepath, &new_content)
            .map_err(|e| format!("写入 {} 失败: {}", filepath, e))?;

        results.push(BatchFileResult {
            file: Path::new(filepath).to_string_lossy().to_string(),
            edits: edits.len(),
            total: lines.len(),
        });
    }

    Ok(BatchResult {
        status: "ok".to_string(),
        files: results.len(),
        results,
    })
}

/// 查找函数范围（基于花括号计数）
fn op_function_range_raw(filepath: &str, target_line: usize) -> Result<(usize, usize), String> {
    let content = fs::read_to_string(Path::new(filepath))
        .map_err(|e| format!("读取文件失败: {}", e))?;
    let lines: Vec<&str> = content.split('\n').collect();
    if target_line < 1 || target_line > lines.len() {
        return Err(format!("目标行 {} 超出文件范围 (1..{})", target_line, lines.len()));
    }

    #[derive(Clone, Copy)]
    enum CommentState {
        None,
        Line,
        Block,
    }

    let mut depth: i32 = 0;
    let mut in_string = false;
    let mut string_char = ' ';
    let mut escape_next = false;
    let mut comment_state = CommentState::None;
    let mut current_start: Option<usize> = None;
    let mut ranges: Vec<(usize, usize)> = Vec::new();

    for (line_idx, line) in lines.iter().enumerate() {
        let chars: Vec<char> = line.chars().collect();
        let mut col = 0;
        comment_state = if matches!(comment_state, CommentState::Block) {
            CommentState::Block
        } else {
            CommentState::None
        };

        'col_loop: while col < chars.len() {
            let ch = chars[col];
            let next = if col + 1 < chars.len() { Some(chars[col + 1]) } else { None };

            if escape_next {
                escape_next = false;
                col += 1;
                continue;
            }

            // 行注释
            if !in_string && !matches!(comment_state, CommentState::Block) && ch == '/' && next == Some('/') {
                comment_state = CommentState::Line;
                break 'col_loop;
            }

            // 块注释开始
            if !in_string && !matches!(comment_state, CommentState::Block) && ch == '/' && next == Some('*') {
                comment_state = CommentState::Block;
                col += 2;
                continue;
            }

            // 块注释结束
            if matches!(comment_state, CommentState::Block) && ch == '*' && next == Some('/') {
                comment_state = CommentState::None;
                col += 2;
                continue;
            }

            if matches!(comment_state, CommentState::Block) || matches!(comment_state, CommentState::Line) {
                col += 1;
                continue;
            }

            // 字符串开始/结束
            if (ch == '"' || ch == '\'' || ch == '`') && !in_string {
                in_string = true;
                string_char = ch;
                col += 1;
                continue;
            } else if in_string && ch == string_char {
                in_string = false;
                col += 1;
                continue;
            }

            // 转义
            if in_string && ch == '\\' {
                escape_next = true;
                col += 1;
                continue;
            }

            if in_string {
                col += 1;
                continue;
            }

            // 花括号计数
            if ch == '{' {
                if depth == 0 {
                    current_start = Some(line_idx + 1);
                }
                depth += 1;
            } else if ch == '}' {
                depth -= 1;
                if depth == 0 {
                    if let Some(start) = current_start {
                        ranges.push((start, line_idx + 1));
                        current_start = None;
                    }
                }
                if depth < 0 {
                    depth = 0;
                }
            }

            col += 1;
        }
    }

    for &(rs, re) in &ranges {
        if rs <= target_line && target_line <= re {
            return Ok((rs, re));
        }
    }

    Err(format!(
        "第 {} 行不在任何函数/块范围内（基于花括号检测）",
        target_line
    ))
}

pub fn op_function_range(filepath: &str, line: usize) -> Result<FunctionRangeResult, String> {
    let (start, end) = op_function_range_raw(filepath, line)?;
    Ok(FunctionRangeResult { start, end })
}
