use std::path::Path;
use crate::fast_edit::core::{read_lines, write_file_atomic, parse_content, quick_balance_check, build_diff};
use crate::fast_edit::func_range::op_function_range_raw;

// ── Result structs ──

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

// ── Operations ──

/// 显示文件内容
pub fn op_show(filepath: &str, start: usize, end: Option<&str>) -> Result<ShowResult, String> {
    let (lines, _) = read_lines(filepath).map_err(|e| format!("读取文件失败: {}", e))?;
    let total = lines.len();
    let mut s = start.max(1);
    let e = match end {
        Some("auto") | None => {
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

    let insert_pos = after;
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
pub fn op_batch(spec: &str) -> Result<BatchResult, String> {
    let spec_val: serde_json::Value =
        serde_json::from_str(spec).map_err(|e| format!("batch spec JSON 解析失败: {}", e))?;

    let file_specs = match &spec_val {
        serde_json::Value::Array(arr) => {
            arr.iter().collect()
        }
        serde_json::Value::Object(map) => {
            if let Some(files) = map.get("files").and_then(|v| v.as_array()) {
                files.iter().collect()
            } else {
                vec![&spec_val]
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
            let a_key = a.get("start").or_else(|| a.get("line")).and_then(|v| v.as_u64()).unwrap_or(0);
            let b_key = b.get("start").or_else(|| b.get("line")).and_then(|v| v.as_u64()).unwrap_or(0);
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
                        let mut nl: Vec<String> = nc.split('\n')
                            .map(|l| format!("{}{}", l.trim_end_matches('\r'), le))
                            .collect();
                        if e < lines.len() && !nl.is_empty() && !nl.last().expect("已检查非空").ends_with('\n') {
                            if let Some(last) = nl.last_mut() {
                                last.push_str(&le);
                            }
                        }
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
                        return Err(format!("batch/delete: end ({}) 超出范围 ({}..{})", e, s, lines.len()));
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
