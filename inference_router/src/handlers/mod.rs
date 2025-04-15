// Handlers module - re-exports from submodules
mod health_handler;
mod graph_handler;

// Re-export handler functions
pub use health_handler::health_check;
pub use graph_handler::graph_handler;


