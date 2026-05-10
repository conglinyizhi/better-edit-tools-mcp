#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum Lang {
    En,
    Zh,
}

impl Lang {
    pub(crate) fn parse(tag: &str) -> Option<Self> {
        let normalized = tag.trim().replace('_', "-").to_ascii_lowercase();
        let primary = normalized.split('-').next().unwrap_or("");
        match primary {
            "en" => Some(Self::En),
            "zh" => Some(Self::Zh),
            _ => None,
        }
    }

    pub(crate) fn from_env() -> Self {
        std::env::var("LANG")
            .ok()
            .and_then(|value| Self::parse(&value))
            .unwrap_or(Self::En)
    }
}
