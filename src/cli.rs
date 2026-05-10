use crate::lang::Lang;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) struct CliConfig {
    pub(crate) lang: Lang,
}

pub(crate) fn parse_or_exit() -> Option<CliConfig> {
    let mut version = false;
    let mut help = false;
    let mut lang = None;
    let mut args = std::env::args().skip(1).peekable();

    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--version" | "-V" => version = true,
            "--help" | "-h" => help = true,
            "--lang" => {
                let value = match args.next() {
                    Some(value) => value,
                    None => {
                        eprintln!("--lang 需要一个语言标签值，例如 zh 或 en");
                        return None;
                    }
                };
                lang = Some(match Lang::parse(&value) {
                    Some(lang) => lang,
                    None => {
                        eprintln!("不支持的 --lang 值: {value}");
                        return None;
                    }
                });
            }
            _ => {
                if let Some(value) = arg.strip_prefix("--lang=") {
                    lang = Some(match Lang::parse(value) {
                        Some(lang) => lang,
                        None => {
                            eprintln!("不支持的 --lang 值: {value}");
                            return None;
                        }
                    });
                }
            }
        }
    }

    if help {
        println!(
            "{} {}\n\nUsage:\n  {} [--lang <zh|en>] [--version] [--help]\n\nRuns the MCP server over stdio by default.",
            env!("CARGO_PKG_NAME"),
            env!("CARGO_PKG_VERSION"),
            env!("CARGO_PKG_NAME")
        );
        return None;
    }

    if version {
        println!("{}", env!("CARGO_PKG_VERSION"));
        return None;
    }

    Some(CliConfig {
        lang: lang.unwrap_or_else(Lang::from_env),
    })
}
