use std::sync::{Arc, Mutex, atomic::{AtomicBool}};
use crate::models::InferenceGraphSpec;

/// Application state to share between handlers
pub struct AppState {
    pub graph: Arc<InferenceGraphSpec>,
    pub headers_to_propagate: Option<Vec<String>>,
    pub active_requests: Arc<Mutex<u32>>,
    pub is_shutting_down: Arc<AtomicBool>,
}

impl AppState {
    pub fn new(graph: InferenceGraphSpec, headers_to_propagate: Option<Vec<String>>) -> Self {
        AppState {
            graph: Arc::new(graph),
            headers_to_propagate,
            active_requests: Arc::new(Mutex::new(0)),
            is_shutting_down: Arc::new(AtomicBool::new(false)),
        }
    }
} 