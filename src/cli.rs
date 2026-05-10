pub(crate) fn parse_or_exit() -> bool {
    let mut version = false;
    let mut help = false;

    for arg in std::env::args().skip(1) {
        match arg.as_str() {
            "--version" | "-V" => version = true,
            "--help" | "-h" => help = true,
            _ => {}
        }
    }

    if help {
        println!(
            "{} {}\n\nUsage:\n  {} [--version] [--help]\n\nRuns the MCP server over stdio by default.",
            env!("CARGO_PKG_NAME"),
            env!("CARGO_PKG_VERSION"),
            env!("CARGO_PKG_NAME")
        );
        return false;
    }

    if version {
        println!("{}", env!("CARGO_PKG_VERSION"));
        return false;
    }

    true
}
