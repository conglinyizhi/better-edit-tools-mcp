use rmcp::handler::server::wrapper::Parameters;
use rmcp::{schemars, tool, tool_router, ServiceExt, transport::stdio};

use crate::fast_edit::{resolve_target_span, CommonEditParams};

fn validate_format(fmt: &str) -> Result<(), String> {
    if !matches!(fmt, "plain" | "diff") {
        return Err(format!("format 参数仅支持 'plain' 或 'diff', 收到: '{}'", fmt));
    }
    Ok(())
}

#[derive(Debug, Clone, serde::Deserialize, schemars::JsonSchema)]
#[serde(untagged)]
enum ShowEndParam {
    Auto(String),
    Line(u32),
}

#[derive(Debug, serde::Deserialize, schemars::JsonSchema)]
struct StructureBalanceParams {
    file: String,
    mode: Option<String>,
}

#[derive(Debug, serde::Deserialize, schemars::JsonSchema)]
struct ShowParams {
    file: String,
    start: u32,
    end: Option<ShowEndParam>,
    #[serde(default, flatten)]
    common: CommonEditParams,
}

#[derive(Debug, serde::Deserialize, schemars::JsonSchema)]
struct ReplaceParams {
    file: String,
    start: u32,
    end: u32,
    content: String,
    raw: Option<bool>,
    format: Option<String>,
    #[serde(default, flatten)]
    common: CommonEditParams,
}

#[derive(Debug, serde::Deserialize, schemars::JsonSchema)]
struct InsertParams {
    file: String,
    line: u32,
    content: String,
    raw: Option<bool>,
    format: Option<String>,
    #[serde(default, flatten)]
    common: CommonEditParams,
}

#[derive(Debug, serde::Deserialize, schemars::JsonSchema)]
struct DeleteParams {
    file: String,
    start: Option<u32>,
    end: Option<u32>,
    line: Option<u32>,
    lines: Option<String>,
    format: Option<String>,
    #[serde(default, flatten)]
    common: CommonEditParams,
}

#[derive(Debug, serde::Deserialize, schemars::JsonSchema)]
struct BatchParams {
    spec: String,
    #[serde(default, flatten)]
    common: CommonEditParams,
}

#[derive(Debug, serde::Deserialize, schemars::JsonSchema)]
struct FuncRangeParams {
    file: String,
    line: u32,
}

#[derive(Debug, serde::Deserialize, schemars::JsonSchema)]
struct TagRangeParams {
    file: String,
    line: u32,
}

#[derive(Debug, serde::Deserialize, schemars::JsonSchema)]
struct WriteParams {
    spec: String,
    #[serde(default, flatten)]
    common: CommonEditParams,
}

#[derive(Clone)]
struct OpenCodeTools;

#[tool_router(server_handler)]
impl OpenCodeTools {
    #[tool(name = "be-show", description = "显示文件指定行范围的内容（带行号）。end 可省略、传数字，或传 'auto' 自动扩展到包含 start 行的完整函数范围。")]
    fn be_show(&self, Parameters(params): Parameters<ShowParams>) -> Result<String, String> {
        let (start, end) = if let Some(target) = params.common.target.as_ref() {
            let span = resolve_target_span(&params.file, target).map_err(|e| e.to_string())?;
            (span.start, Some(crate::fast_edit::ShowEnd::Line(span.end)))
        } else {
            let end = match params.end {
                None => None,
                Some(ShowEndParam::Auto(s)) if s == "auto" => Some(crate::fast_edit::ShowEnd::Auto),
                Some(ShowEndParam::Auto(s)) => {
                    return Err(format!("end 参数仅支持数字或 'auto', 收到: '{}'", s));
                }
                Some(ShowEndParam::Line(v)) => Some(crate::fast_edit::ShowEnd::Line(v as usize)),
            };
            (params.start as usize, end)
        };
        let r = crate::fast_edit::op_show(&params.file, start, end).map_err(|e| e.to_string())?;
        serde_json::to_string_pretty(&r).map_err(|e| format!("JSON 序列化失败: {}", e))
    }

    #[tool(name = "be-replace", description = "替换文件中指定行范围的内容。start/end 传数字。")]
    fn be_replace(&self, Parameters(params): Parameters<ReplaceParams>) -> Result<String, String> {
        let raw = params.raw.unwrap_or(false);
        let fmt = params.format.as_deref().unwrap_or("plain");
        validate_format(fmt)?;
        let (start, end) = if let Some(target) = params.common.target.as_ref() {
            let span = resolve_target_span(&params.file, target).map_err(|e| e.to_string())?;
            (span.start, span.end)
        } else {
            (params.start as usize, params.end as usize)
        };
        let r = crate::fast_edit::op_replace(
            &params.file, start, end,
            &params.content, raw, fmt, params.common.preview,
        ).map_err(|e| e.to_string())?;
        serde_json::to_string_pretty(&r).map_err(|e| format!("JSON 序列化失败: {}", e))
    }

    #[tool(name = "be-insert", description = "在文件指定行后插入内容。line=0 表示插入到文件开头。")]
    fn be_insert(&self, Parameters(params): Parameters<InsertParams>) -> Result<String, String> {
        let raw = params.raw.unwrap_or(false);
        let fmt = params.format.as_deref().unwrap_or("plain");
        validate_format(fmt)?;
        let after = if let Some(target) = params.common.target.as_ref() {
            let span = resolve_target_span(&params.file, target).map_err(|e| e.to_string())?;
            span.end
        } else {
            params.line as usize
        };
        let r = crate::fast_edit::op_insert(&params.file, after, &params.content, raw, fmt, params.common.preview).map_err(|e| e.to_string())?;
        serde_json::to_string_pretty(&r).map_err(|e| format!("JSON 序列化失败: {}", e))
    }

    #[tool(name = "be-delete", description = "删除文件中指定行范围。start/end 传数字（省略时删除单行 line）；或传入 lines JSON 数组字符串批量删除多行。")]
    fn be_delete(&self, Parameters(params): Parameters<DeleteParams>) -> Result<String, String> {
        let fmt = params.format.as_deref().unwrap_or("plain");
        validate_format(fmt)?;
        let (start, end, line) = if let Some(target) = params.common.target.as_ref() {
            let span = resolve_target_span(&params.file, target).map_err(|e| e.to_string())?;
            (Some(span.start), Some(span.end), None)
        } else {
            (
                params.start.map(|v| v as usize),
                params.end.map(|v| v as usize),
                params.line.map(|v| v as usize),
            )
        };
        let r = crate::fast_edit::op_delete(
            &params.file,
            start,
            end,
            line,
            params.lines.as_deref(),
            fmt,
            params.common.preview,
        ).map_err(|e| e.to_string())?;
        serde_json::to_string_pretty(&r).map_err(|e| format!("JSON 序列化失败: {}", e))
    }

    #[tool(name = "be-batch", description = "批量编辑文件（单次调用完成多处修改）。性能最优，推荐用于 3+ 处修改。支持单文件或多文件。所有行号均基于原始文件，工具内部自动从下往上执行，无需手动排序。spec JSON 格式：单文件 {\"file\":\"/path\",\"edits\":[{\"action\":\"replace-lines\",\"start\":10,\"end\":12,\"content\":\"new\"}]} 或多文件 {\"files\":[...]}")]
    fn be_batch(&self, Parameters(params): Parameters<BatchParams>) -> Result<String, String> {
        let r = crate::fast_edit::op_batch(&params.spec, params.common.preview).map_err(|e| e.to_string())?;
        serde_json::to_string_pretty(&r).map_err(|e| format!("JSON 序列化失败: {}", e))
    }

    #[tool(name = "be-write", description = "批量写入文件内容。JSON 格式：{\"file\":\"/path\",\"content\":\"...\"} 或 {\"files\":[...]}。当 JSON 因特殊字符解析失败时自动启用状态机降级提取。")]
    fn be_write(&self, Parameters(params): Parameters<WriteParams>) -> Result<String, String> {
        let r = crate::fast_edit::op_write(&params.spec, params.common.preview).map_err(|e| e.to_string())?;
        serde_json::to_string_pretty(&r).map_err(|e| format!("JSON 序列化失败: {}", e))
    }

    #[tool(name = "be-func-range", description = "传入文件路径和行号，返回该行所在 {} 块/函数的起止行号（基于花括号计数，支持字符串/注释感知）。")]
    fn be_func_range(
        &self,
        Parameters(params): Parameters<FuncRangeParams>,
    ) -> Result<String, String> {
        let r = crate::fast_edit::op_func_range(&params.file, params.line as usize).map_err(|e| e.to_string())?;
        serde_json::to_string_pretty(&r).map_err(|e| format!("JSON 序列化失败: {}", e))
    }

    #[tool(name = "be-tag-range", description = "传入文件路径和行号，返回该行所在 XML/HTML/Vue tag 的起止行号。")]
    fn be_tag_range(
        &self,
        Parameters(params): Parameters<TagRangeParams>,
    ) -> Result<String, String> {
        let r = crate::fast_edit::op_tag_range(&params.file, params.line as usize).map_err(|e| e.to_string())?;
        serde_json::to_string_pretty(&r).map_err(|e| format!("JSON 序列化失败: {}", e))
    }

    #[tool(name = "be-balance", description = "检查文件中括号/花括号/方括号的成对情况、HTML/XML 标签闭合，以及引号的奇偶警告。三种模式：aggregate（聚合）、unbalanced（失衡，默认）、tree（树状嵌套）")]
    fn be_balance(
        &self,
        Parameters(params): Parameters<StructureBalanceParams>,
    ) -> Result<String, String> {
        let mode = params.mode.as_deref().unwrap_or("unbalanced");
        crate::structure_balance::check_structure_balance(&params.file, mode)
    }
}

pub(crate) async fn run() -> Result<(), Box<dyn std::error::Error>> {
    let service = OpenCodeTools.serve(stdio()).await?;
    service.waiting().await?;
    Ok(())
}
