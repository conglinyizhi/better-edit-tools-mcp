use crate::error::{EditError, EditResult};
use crate::fast_edit::core::write_files_atomic;

// ── Internal types ──

struct WriteFileSpec {
    file: String,
    content: String,
}

enum WriteSpec {
    Single(WriteFileSpec),
    Multi(Vec<WriteFileSpec>),
}

// ── Public types ──

#[derive(serde::Serialize)]
pub struct WriteResult {
    pub status: String,
    pub files: usize,
    pub results: Vec<WriteFileResult>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub degraded: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub warning: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub preview: Option<bool>,
}

#[derive(serde::Serialize)]
pub struct WriteFileResult {
    pub file: String,
    pub lines: usize,
    pub bytes: usize,
}

// ── 状态机降级解析（当 JSON 解析失败时使用） ──

/// 从 JSON-like 字符串中直接提取 content 字段值，不依赖 JSON parser
fn extract_content_raw(spec: &str) -> Option<String> {
    let key = "\"content\"";
    let idx = spec.find(key)?;
    let after_key = &spec[idx + key.len()..];
    let colon = after_key.find(':')?;
    let mut pos = idx + key.len() + colon + 1;

    let bytes = spec.as_bytes();
    while pos < bytes.len() && matches!(bytes[pos], b' ' | b'\t' | b'\n' | b'\r') {
        pos += 1;
    }

    if pos >= bytes.len() || bytes[pos] != b'"' {
        return None;
    }
    pos += 1;

    let mut result = String::new();
    while pos < bytes.len() {
        match bytes[pos] {
            b'\\' if pos + 1 < bytes.len() => {
                match bytes[pos + 1] {
                    b'n' => result.push('\n'),
                    b't' => result.push('\t'),
                    b'r' => result.push('\r'),
                    b'"' => result.push('"'),
                    b'\\' => result.push('\\'),
                    c => result.push(c as char),
                }
                pos += 2;
                continue;
            }
            b'"' => break,
            c => result.push(c as char),
        }
        pos += 1;
    }
    Some(result)
}

/// 从 JSON-like 字符串中直接提取 file 字段值
fn extract_file_raw(spec: &str) -> Option<String> {
    let key = "\"file\"";
    let idx = spec.find(key)?;
    let after_key = &spec[idx + key.len()..];
    let colon = after_key.find(':')?;
    let mut pos = idx + key.len() + colon + 1;

    let bytes = spec.as_bytes();
    while pos < bytes.len() && matches!(bytes[pos], b' ' | b'\t' | b'\n' | b'\r') {
        pos += 1;
    }

    if pos >= bytes.len() || bytes[pos] != b'"' {
        return None;
    }
    pos += 1;

    let mut result = String::new();
    while pos < bytes.len() {
        match bytes[pos] {
            b'\\' if pos + 1 < bytes.len() => {
                result.push(bytes[pos + 1] as char);
                pos += 2;
                continue;
            }
            b'"' => break,
            c => result.push(c as char),
        }
        pos += 1;
    }
    Some(result)
}

/// 降级方案：手动从 JSON-like 字符串中提取 file 和 content 字段
fn parse_spec_raw(spec: &str) -> EditResult<WriteSpec> {
    // 检测多文件模式
    if let Some(files_idx) = spec.find("\"files\"") {
        let after_files = &spec[files_idx + 8..];
        let bracket = after_files
            .find('[')
            .ok_or_else(|| EditError::invalid_arg("files 字段后找不到 ["))?;
        let array_start = files_idx + 8 + bracket;

        let mut depth = 0i32;
        let mut in_str = false;
        let mut array_end = None;
        for (i, &b) in spec.as_bytes()[array_start..].iter().enumerate() {
            if b == b'\\' && in_str {
                continue;
            }
            if b == b'"' {
                in_str = !in_str;
                continue;
            }
            if in_str {
                continue;
            }
            if b == b'[' {
                depth += 1;
            } else if b == b']' {
                depth -= 1;
                if depth == 0 {
                    array_end = Some(array_start + i);
                    break;
                }
            }
        }
        let array_end = array_end.ok_or_else(|| EditError::invalid_arg("找不到数组结束的 ]"))?;
        let array_body = &spec[array_start + 1..array_end];

        let mut results = Vec::new();
        let mut search_pos = 0;
        while let Some(elem_start) = array_body[search_pos..].find("{\"file\"") {
            let abs_start = search_pos + elem_start;

            let mut depth = 0i32;
            let mut in_str = false;
            let mut elem_end = None;
            let elem_bytes = array_body.as_bytes();
            for (i, &b) in elem_bytes[abs_start..].iter().enumerate() {
                if b == b'\\' && in_str {
                    continue;
                }
                if b == b'"' {
                    in_str = !in_str;
                    continue;
                }
                if in_str {
                    continue;
                }
                if b == b'{' {
                    depth += 1;
                } else if b == b'}' {
                    depth -= 1;
                    if depth == 0 {
                        elem_end = Some(abs_start + i + 1);
                        break;
                    }
                }
            }
            let elem_end = match elem_end {
                Some(e) => e,
                None => break,
            };

            let elem = &array_body[abs_start..elem_end];
            if let Some(fp) = extract_file_raw(elem) {
                let ct = extract_content_raw(elem).unwrap_or_default();
                results.push(WriteFileSpec {
                    file: fp,
                    content: ct,
                });
            }
            search_pos = elem_end;
        }

        if results.is_empty() {
            return Err(EditError::invalid_arg("从 files 数组中解析出 0 个有效元素"));
        }
        return Ok(WriteSpec::Multi(results));
    }

    // 单文件模式
    let fp = extract_file_raw(spec);
    let ct =
        extract_content_raw(spec).ok_or_else(|| EditError::invalid_arg("找不到 content 字段"))?;
    Ok(WriteSpec::Single(WriteFileSpec {
        file: fp.unwrap_or_default(),
        content: ct,
    }))
}

/// 从标准 JSON Value 中解析文件规格
fn parse_write_value(val: &serde_json::Value) -> EditResult<WriteSpec> {
    let parse_one = |v: &serde_json::Value| -> EditResult<WriteFileSpec> {
        let file = v
            .get("file")
            .and_then(|v| v.as_str())
            .ok_or_else(|| EditError::invalid_arg("缺少 file 字段"))?;
        let mut content = v
            .get("content")
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .to_string();
        // 支持 extract: true，自动提取 ``` 代码块内容
        if v.get("extract").and_then(|v| v.as_bool()).unwrap_or(false) {
            content = extract_code_blocks(&content);
        }
        Ok(WriteFileSpec {
            file: file.to_string(),
            content,
        })
    };

    if let Some(files) = val.get("files").and_then(|v| v.as_array()) {
        let mut specs = Vec::new();
        for f in files {
            specs.push(parse_one(f)?);
        }
        Ok(WriteSpec::Multi(specs))
    } else {
        parse_one(val).map(WriteSpec::Single)
    }
}

/// 提取 markdown 代码块内容，无代码块则返回原文
fn extract_code_blocks(text: &str) -> String {
    let mut result = String::new();
    let mut in_block = false;
    let mut capture = false;

    for line in text.lines() {
        if line.trim_start().starts_with("```") {
            if in_block {
                in_block = false;
                capture = false;
            } else {
                in_block = true;
                capture = true;
            }
            continue;
        }
        if capture {
            result.push_str(line);
            result.push('\n');
        }
    }

    if result.is_empty() {
        text.to_string()
    } else {
        result.trim_end().to_string()
    }
}

// ── Public API ──

/// 写入文件内容，支持 JSON 降级解析
pub fn op_write(spec: &str, preview: bool) -> EditResult<WriteResult> {
    let (write_spec, degraded) = match serde_json::from_str::<serde_json::Value>(spec) {
        Ok(val) => {
            let specs = parse_write_value(&val)?;
            (specs, false)
        }
        Err(_) => {
            let parsed = parse_spec_raw(spec)?;
            (parsed, true)
        }
    };

    let file_specs: Vec<WriteFileSpec> = match write_spec {
        WriteSpec::Single(s) => vec![s],
        WriteSpec::Multi(v) => v,
    };

    let mut results = Vec::new();
    let writes: Vec<(String, String)> = file_specs
        .iter()
        .map(|fs| (fs.file.clone(), fs.content.clone()))
        .collect();
    if !preview {
        write_files_atomic(&writes).map_err(|e| {
            let path = writes.first().map(|(p, _)| p.as_str()).unwrap_or(spec);
            EditError::write_path(path, e)
        })?;
    }

    for fs in &file_specs {
        let content = fs.content.clone();
        let line_count = content.lines().count();
        let byte_count = content.len();
        results.push(WriteFileResult {
            file: fs.file.clone(),
            lines: line_count,
            bytes: byte_count,
        });
    }

    let mut result = WriteResult {
        status: "ok".to_string(),
        files: results.len(),
        results,
        degraded: None,
        warning: None,
        preview: preview.then_some(true),
    };

    if degraded {
        result.degraded = Some(true);
        result.warning = Some(
            "JSON 格式有误（如未转义的引号/换行符等），已启用状态机降级方案提取内容，写入内容可能不完整或不准确，请立即重新读取源文件确认后继续修改"
                .to_string(),
        );
    }

    Ok(result)
}
