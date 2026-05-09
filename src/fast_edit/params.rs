use rmcp::schemars;

use crate::error::{EditError, EditResult};

use super::func_range::op_function_range_raw;
use super::tag_range::op_tag_range;

#[derive(Debug, Clone, serde::Deserialize, schemars::JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum ContentTarget {
    Line(u32),
    Function(String),
    Marker(String),
    Tag(String),
}

#[derive(Debug, Clone, serde::Deserialize, schemars::JsonSchema, Default)]
pub struct CommonEditParams {
    #[serde(default)]
    pub preview: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub target: Option<ContentTarget>,
}

#[derive(Debug, Clone, Copy)]
pub struct TargetSpan {
    pub start: usize,
    pub end: usize,
}

pub fn resolve_target_span(filepath: &str, target: &ContentTarget) -> EditResult<TargetSpan> {
    let content = std::fs::read_to_string(filepath)
        .map_err(|e| EditError::read_path(filepath, e))?;
    let lines: Vec<&str> = content.lines().collect();
    if lines.is_empty() {
        return Err(EditError::invalid_arg("target: 文件为空"));
    }

    match target {
        ContentTarget::Line(line) => {
            let line = *line as usize;
            if line < 1 || line > lines.len() {
                return Err(EditError::invalid_arg(format!("target line {} 超出文件范围 (1..{})", line, lines.len())));
            }
            Ok(TargetSpan { start: line, end: line })
        }
        ContentTarget::Marker(marker) => {
            let needle = marker.trim();
            if needle.is_empty() {
                return Err(EditError::invalid_arg("marker 不能为空"));
            }
            let found = lines
                .iter()
                .position(|line| line.contains(needle))
                .map(|idx| idx + 1)
                .ok_or_else(|| EditError::invalid_arg(format!("未找到 marker: {}", needle)))?;
            Ok(TargetSpan { start: found, end: found })
        }
        ContentTarget::Function(name) => {
            let needle = name.trim();
            if needle.is_empty() {
                return Err(EditError::invalid_arg("function 不能为空"));
            }
            let found = lines
                .iter()
                .position(|line| line.contains(&format!("fn {}", needle)) || line.contains(&format!("{}(", needle)))
                .map(|idx| idx + 1)
                .ok_or_else(|| EditError::invalid_arg(format!("未找到 function: {}", needle)))?;
            let (start, end) = op_function_range_raw(filepath, found)?;
            Ok(TargetSpan { start, end })
        }
        ContentTarget::Tag(name) => {
            let needle = name.trim();
            if needle.is_empty() {
                return Err(EditError::invalid_arg("tag 不能为空"));
            }
            let found = lines
                .iter()
                .position(|line| line.contains(&format!("<{}", needle)) || line.contains(&format!("</{}", needle)))
                .map(|idx| idx + 1)
                .ok_or_else(|| EditError::invalid_arg(format!("未找到 tag: {}", needle)))?;
            let tag = op_tag_range(filepath, found)?;
            Ok(TargetSpan { start: tag.start, end: tag.end })
        }
    }
}
