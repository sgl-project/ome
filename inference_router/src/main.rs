use actix_web::{web, App, HttpServer};
use log::{info, error, warn};
use std::time::Duration;
use tokio::signal;
use std::sync::atomic::{Ordering};
use std::process;

pub mod models;
pub mod handlers;
pub mod logging;
pub mod config;
pub mod router;
pub mod state;

use models::{
    InferenceGraphSpec, 
    parse_graph_from_file, 
    parse_graph_from_string, 
    parse_graph_from_value,
    RouterError,
    validator,
};
use handlers::{health_check, graph_handler};
use config::{Config, Commands};
use state::AppState;

/// Helper function to convert RouterError to std::io::Error
fn router_error_to_io_error(error: RouterError) -> std::io::Error {
    let kind = match error {
        RouterError::IoError(io_err) => return io_err,
        RouterError::ConfigError(_) => std::io::ErrorKind::InvalidInput,
        RouterError::GraphParseError(_) => std::io::ErrorKind::InvalidData,
        RouterError::JsonError(_) => std::io::ErrorKind::InvalidData,
        _ => std::io::ErrorKind::Other,
    };
    
    std::io::Error::new(kind, error.to_string())
}

/// Parse the inference graph from the provided source
fn parse_graph(config: &Config) -> Result<InferenceGraphSpec, RouterError> {
    match (&config.graph_json, &config.graph_string, &config.graph_value) {
        (Some(path), _, _) => {
            info!("Loading from file: {:?}", path);
            parse_graph_from_file(path)
        },
        (_, Some(graph_str), _) => {
            info!("Loading from string input");
            parse_graph_from_string(graph_str)
        },
        (_, _, Some(value)) => {
            info!("Loading from JSON value");
            parse_graph_from_value(value.clone())
        },
        _ => {
            error!("No inference graph source provided. Use either --graph-json or --graph-string");
            Err(RouterError::ConfigError("No inference graph source provided".to_string()))
        }
    }
}

/// Trait for command handlers
trait CommandHandler {
    async fn handle(&self, config: Config) -> std::io::Result<()>;
}

/// Handler for the Router command
struct RouterCommand;
impl CommandHandler for RouterCommand {
    async fn handle(&self, config: Config) -> std::io::Result<()> {
        // Parse the inference graph
        let graph = match parse_graph(&config) {
            Ok(g) => g,
            Err(e) => {
                error!("Failed to parse inference graph: {}", e);
                return Err(router_error_to_io_error(e));
            }
        };
        
        // Run as a router server
        run_router_server(config, graph).await
    }
}

/// Handler for the Validate command
struct ValidateCommand {
    verbose: bool,
}

impl CommandHandler for ValidateCommand {
    async fn handle(&self, config: Config) -> std::io::Result<()> {
        // Run in validation mode
        match parse_graph(&config) {
            Ok(graph) => {
                // Run additional validation checks
                info!("Performing additional validation checks...");
                
                // Use the validator module to validate the graph structure
                if let Err(e) = validator::validate_graph_structure(&graph) {
                    error!("Graph validation failed: {}", e);
                    return Err(router_error_to_io_error(e));
                }
                
                // Graph is already validated during parsing
                if self.verbose {
                    // Print graph details
                    validator::print_graph_details(&graph);
                }
                
                info!("Graph validation successful");
                
                // Exit successfully (using Ok since we're handling the process exit)
                Ok(())
            },
            Err(e) => {
                error!("Graph validation failed: {}", e);
                Err(router_error_to_io_error(e))
            }
        }
    }
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    // Get the configuration
    let config = Config::from_args();
    
    // Setup logging
    logging::init_logging(config.json_logs, &config.log_level);
    
    info!("Loading inference graph");
    
    // Execute the appropriate command handler based on the subcommand
    match &config.command {
        Some(Commands::Router) => {
            let handler = RouterCommand{};
            handler.handle(config).await
        },
        Some(Commands::Validate { verbose }) => {
            let handler = ValidateCommand { verbose: *verbose };
            let result = handler.handle(config).await;
            
            // For validate command, we want to handle process exit explicitly
            match result {
                Ok(_) => {
                    process::exit(0);
                },
                Err(_) => {
                    process::exit(1);
                }
            }
        },
        None => {
            let handler = RouterCommand{};
            handler.handle(config).await // Default to router command
        },
    }
}

/// Run the application in router server mode
async fn run_router_server(config: Config, graph: InferenceGraphSpec) -> std::io::Result<()> {
    // Create application state
    let app_state = web::Data::new(AppState::new(graph, config.parse_headers()));
    
    // Clone app_state for the shutdown handler
    let app_state_clone = app_state.clone();
    
    // Start the HTTP server
    let bind_address = config.bind_address();
    info!("Binding to {}", bind_address);
    
    // Create a server with graceful shutdown
    let server = HttpServer::new(move || {
        App::new()
            .app_data(app_state.clone())
            .route("/health", web::get().to(health_check))
            .route("/", web::post().to(graph_handler))
            .default_service(web::to(graph_handler)) // This will handle all other routes
    })
    .bind(bind_address)?
    .run();
    
    // Get the server handle for graceful shutdown
    let server_handle = server.handle();
    
    // Spawn a task to handle graceful shutdown
    let grace_period = config.grace_period;
    
    tokio::spawn(async move {
        // Wait for shutdown signal (SIGTERM or SIGINT)
        match signal::ctrl_c().await {
            Ok(()) => {
                info!("Received shutdown signal (SIGINT)");
                handle_shutdown(app_state_clone, server_handle, grace_period).await;
            }
            Err(err) => {
                error!("Error waiting for shutdown signal: {}", err);
            }
        }
    });
    
    // Run the server
    server.await
}

/// Handle graceful shutdown of the server
async fn handle_shutdown(
    app_state: web::Data<AppState>,
    server_handle: actix_web::dev::ServerHandle,
    grace_period: u64,
) {
    // Set the shutdown flag
    app_state.is_shutting_down.store(true, Ordering::SeqCst);
    
    info!("Starting graceful shutdown...");
    
    // Wait for active requests to complete or timeout
    let start_time = std::time::Instant::now();
    let grace_duration = Duration::from_secs(grace_period);
    
    loop {
        // Check if there are any active requests
        let active_count = {
            let active_requests = app_state.active_requests.lock().unwrap();
            *active_requests
        };
        
        if active_count == 0 {
            info!("All requests completed, proceeding with shutdown");
            break;
        }
        
        // Check if we've exceeded the grace period
        if start_time.elapsed() >= grace_duration {
            warn!(
                "Grace period of {} seconds exceeded, forcing shutdown with {} active requests",
                grace_period, active_count
            );
            break;
        }
        
        info!("Waiting for {} active requests to complete...", active_count);
        tokio::time::sleep(Duration::from_secs(1)).await;
    }
    
    // Stop accepting new connections
    info!("Stopping server...");
    server_handle.stop(true).await;
    info!("Server stopped");
}