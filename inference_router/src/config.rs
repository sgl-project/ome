use clap::{Parser, Subcommand};
use std::path::PathBuf;
use serde_json::Value;

/// Inference Router CLI
#[derive(Parser, Debug)]
#[command(author, version, about = "Inference router for managing graph-based inference workflows")]
pub struct Config {
    /// Path to the inference graph JSON file
    #[arg(short, long, group = "graph_source", global = true)]
    pub graph_json: Option<PathBuf>,
    
    /// Inference graph as a JSON string
    #[arg(short = 's', long, group = "graph_source", global = true)]
    pub graph_string: Option<String>,
    
    /// Inference graph as JSON value (not directly configurable via CLI)
    #[clap(skip)]
    pub graph_value: Option<Value>,
    
    /// Mode to run the application in
    #[command(subcommand)]
    pub command: Option<Commands>,
    
    /// Port to listen on
    #[arg(short, long, default_value = "8080")]
    pub port: u16,
    
    /// Host to bind to
    #[arg(short = 'H', long, default_value = "127.0.0.1")]
    pub host: String,
    
    /// Headers to propagate (comma-separated regex patterns)
    #[arg(short = 'd', long)]
    pub headers: Option<String>,
    
    /// Log level (trace, debug, info, warn, error)
    #[arg(short, long, default_value = "info")]
    pub log_level: String,
    
    /// Use JSON format for logs
    #[arg(short, long)]
    pub json_logs: bool,
    
    /// Grace period in seconds before forcing shutdown
    #[arg(short = 'G', long, default_value = "30")]
    pub grace_period: u64,
}

/// Available commands
#[derive(Subcommand, Debug)]
pub enum Commands {
    /// Run as a router server (default)
    Router,
    
    /// Only validate the inference graph without starting the server
    Validate {
        /// Verbose output with detailed graph information
        #[arg(short, long)]
        verbose: bool,
    },
}

impl Config {
    /// Load configuration from command-line arguments
    pub fn from_args() -> Self {
        Config::parse()
    }
    
    /// Get the binding address string in format "host:port"
    pub fn bind_address(&self) -> String {
        format!("{}:{}", self.host, self.port)
    }
    
    /// Parse headers to propagate from a comma-separated string
    pub fn parse_headers(&self) -> Option<Vec<String>> {
        self.headers.as_ref().map(|h| h.split(',').map(|s| s.trim().to_string()).collect())
    }
} 