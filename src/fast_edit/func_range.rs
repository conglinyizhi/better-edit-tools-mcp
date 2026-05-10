use crate::error::{EditError, EditResult};
use std::fs;
use std::path::Path;

#[derive(serde::Serialize)]
pub struct FunctionRangeResult {
    pub start: usize,
    pub end: usize,
}

/// 查找函数范围（基于花括号计数）
pub(crate) fn op_function_range_raw(
    filepath: &str,
    target_line: usize,
) -> EditResult<(usize, usize)> {
    let content =
        fs::read_to_string(Path::new(filepath)).map_err(|e| EditError::read_path(filepath, e))?;
    let lines: Vec<&str> = content.split('\n').collect();
    if target_line < 1 || target_line > lines.len() {
        return Err(EditError::invalid_arg(format!(
            "目标行 {} 超出文件范围 (1..{})",
            target_line,
            lines.len()
        )));
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

            // 行注释
            if !in_string
                && !matches!(comment_state, CommentState::Block)
                && ch == '/'
                && next == Some('/')
            {
                comment_state = CommentState::Line;
                break 'col_loop;
            }

            // 块注释开始
            if !in_string
                && !matches!(comment_state, CommentState::Block)
                && ch == '/'
                && next == Some('*')
            {
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

            if matches!(comment_state, CommentState::Block)
                || matches!(comment_state, CommentState::Line)
            {
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
                if depth == 0
                    && let Some(start) = current_start
                {
                    ranges.push((start, line_idx + 1));
                    current_start = None;
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

    Err(EditError::invalid_arg(format!(
        "第 {} 行不在任何函数/块范围内（基于花括号检测）",
        target_line
    )))
}

pub fn op_function_range(filepath: &str, line: usize) -> EditResult<FunctionRangeResult> {
    let (start, end) = op_function_range_raw(filepath, line)?;
    Ok(FunctionRangeResult { start, end })
}

pub fn op_func_range(filepath: &str, line: usize) -> EditResult<FunctionRangeResult> {
    op_function_range(filepath, line)
}
