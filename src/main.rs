mod cli;
mod error;
mod fast_edit;
mod server;
mod structure_balance;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    if cli::parse_or_exit() {
        server::run().await?;
    }
    Ok(())
}
