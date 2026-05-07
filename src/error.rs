
#[derive(Debug, thiserror::Error)]
pub enum EditError {
    #[error("文件 {path} 读取失败: {source}")]
    ReadFile { path: String, source: std::io::Error },

    #[error("文件 {path} 写入失败: {source}")]
    WriteFile { path: String, source: std::io::Error },

    #[error("文件 {path} 重新读取失败: {source}")]
    ReReadFile { path: String, source: std::io::Error },

    #[error("JSON 解析失败: {0}")]
    JsonParse(#[from] serde_json::Error),

    #[error("{0}")]
    InvalidArgument(String),
}

impl EditError {
    pub fn invalid_arg(msg: impl Into<String>) -> Self {
        EditError::InvalidArgument(msg.into())
    }


    pub fn read_path(path: &str, source: std::io::Error) -> Self {
        EditError::ReadFile {
            path: path.to_string(),
            source,
        }
    }

    pub fn write_path(path: &str, source: std::io::Error) -> Self {
        EditError::WriteFile {
            path: path.to_string(),
            source,
        }
    }

    pub fn reread_path(path: &str, source: std::io::Error) -> Self {
        EditError::ReReadFile {
            path: path.to_string(),
            source,
        }
    }
}

pub type EditResult<T> = Result<T, EditError>;
