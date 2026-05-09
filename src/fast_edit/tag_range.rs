use std::fs;
use std::path::Path;

use crate::error::{EditError, EditResult};

#[derive(serde::Serialize)]
pub struct TagRangeResult {
    pub start: usize,
    pub end: usize,
    pub kind: String,
}

pub fn op_tag_range(filepath: &str, line: usize) -> EditResult<TagRangeResult> {
    let content = fs::read_to_string(Path::new(filepath))
        .map_err(|e| EditError::read_path(filepath, e))?;
    let lines: Vec<&str> = content.split('\n').collect();
    if line < 1 || line > lines.len() {
        return Err(EditError::invalid_arg(format!("目标行 {} 超出文件范围 (1..{})", line, lines.len())));
    }

    let mut stack: Vec<(String, usize)> = Vec::new();
    let mut open_ranges: Vec<(String, usize, usize)> = Vec::new();

    for (idx, raw_line) in lines.iter().enumerate() {
        let line_no = idx + 1;
        let mut cursor = 0usize;
        let chars: Vec<char> = raw_line.chars().collect();
        while cursor < chars.len() {
            if chars[cursor] == '<' {
                let mut j = cursor + 1;
                if j < chars.len() && chars[j] == '!' {
                    while j < chars.len() && chars[j] != '>' {
                        j += 1;
                    }
                    cursor = j.saturating_add(1);
                    continue;
                }
                if j < chars.len() && chars[j] == '/' {
                    j += 1;
                    let mut tag = String::new();
                    while j < chars.len() {
                        let ch = chars[j];
                        if ch.is_ascii_alphanumeric() || ch == '-' || ch == '_' || ch == ':' {
                            tag.push(ch);
                            j += 1;
                        } else {
                            break;
                        }
                    }
                    while j < chars.len() && chars[j] != '>' {
                        j += 1;
                    }
                    if let Some((open_tag, open_line)) = stack.pop()
                        && open_tag == tag
                    {
                        open_ranges.push((open_tag, open_line, line_no));
                    }
                    cursor = j.saturating_add(1);
                    continue;
                }
                let mut tag = String::new();
                while j < chars.len() {
                    let ch = chars[j];
                    if ch.is_ascii_alphanumeric() || ch == '-' || ch == '_' || ch == ':' {
                        tag.push(ch);
                        j += 1;
                    } else {
                        break;
                    }
                }
                if !tag.is_empty() {
                    stack.push((tag, line_no));
                }
                while j < chars.len() && chars[j] != '>' {
                    j += 1;
                }
                cursor = j.saturating_add(1);
                continue;
            }
            cursor += 1;
        }
    }

    for (kind, start, end) in open_ranges {
        if start <= line && line <= end {
            return Ok(TagRangeResult { start, end, kind });
        }
    }

    Err(EditError::invalid_arg(format!(
        "第 {} 行不在任何可配对 tag 范围内",
        line
    )))
}
