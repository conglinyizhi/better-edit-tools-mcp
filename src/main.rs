mod cli;
mod error;
mod fast_edit;
mod lang;
mod server;
mod structure_balance;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    if let Some(config) = cli::parse_or_exit() {
        server::run(config.lang).await?;
    }
    Ok(())
}
