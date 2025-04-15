use actix_web::{web, App, HttpResponse, HttpServer};
use log::info;
use serde_json::{json, Value};
use std::net::TcpListener;
use std::sync::{Once, Mutex};
use tokio::task;
use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use thiserror::Error;

// Maximum number of concurrent mock servers to avoid "too many open files" error
static MAX_MOCK_SERVERS: usize = 20;
static MOCK_SERVER_MUTEX: Mutex<()> = Mutex::new(());
static ACTIVE_SERVERS: Mutex<Vec<tokio::task::JoinHandle<()>>> = Mutex::new(Vec::new());
static SERVER_COUNTER: AtomicUsize = AtomicUsize::new(0);

// Define a local RouterError type rather than importing
#[derive(Debug, Error)]
pub enum RouterError {
    #[error("Failed to start mock server: {0}")]
    MockServerError(String),
    
    #[error("IO error: {0}")]
    IoError(#[from] std::io::Error),
    
    #[error("Limit reached: {0}")]
    LimitError(String),
}

// Initialize logger once for tests
static INIT: Once = Once::new();

pub fn init_test_logger() {
    INIT.call_once(|| {
        env_logger::builder()
            .filter_level(log::LevelFilter::Debug)
            .is_test(true)
            .init();
    });
}

// Counter for tracking requests to mock servers
#[derive(Clone)]
pub struct RequestCounter {
    count: Arc<AtomicUsize>,
}

impl RequestCounter {
    pub fn new() -> Self {
        RequestCounter {
            count: Arc::new(AtomicUsize::new(0)),
        }
    }

    pub fn increment(&self) -> usize {
        self.count.fetch_add(1, Ordering::SeqCst) + 1
    }

    pub fn get_count(&self) -> usize {
        self.count.load(Ordering::SeqCst)
    }
    
    pub fn reset(&self) {
        self.count.store(0, Ordering::SeqCst);
    }
}

// Start a mock server that responds with configurable JSON
pub async fn start_mock_server(
    name: &str,
    response: Value,
    counter: RequestCounter,
) -> Result<String, RouterError> {
    // Acquire lock to ensure we don't exceed max servers
    let _guard = MOCK_SERVER_MUTEX.lock().unwrap();
    
    // Check if we've reached the limit
    {
        let server_count = ACTIVE_SERVERS.lock().unwrap().len();
        if server_count >= MAX_MOCK_SERVERS {
            return Err(RouterError::LimitError(format!(
                "Maximum number of mock servers ({}) reached. Consider cleaning up existing servers.", 
                MAX_MOCK_SERVERS
            )));
        }
    }
    
    // Generate a unique ID for this server
    let server_id = SERVER_COUNTER.fetch_add(1, Ordering::SeqCst);
    let server_name = format!("{}-{}", name, server_id);
    
    // Find an available port
    let listener = TcpListener::bind("127.0.0.1:0").map_err(|e| {
        RouterError::MockServerError(format!("Failed to bind to random port: {}", e))
    })?;
    let port = listener.local_addr().unwrap().port();
    let server_url = format!("http://127.0.0.1:{}", port);
    let server_url_clone = server_url.clone();
    
    // Clone data for the closure
    let response_data = response.clone();
    let counter_clone = counter.clone();
    let server_name_clone = server_name.clone();  // Clone for use outside the async block
    
    // Start server in background task
    let server_handle = task::spawn(async move {
        info!("Starting mock server '{}' on {}", server_name, server_url);
        
        let server = HttpServer::new(move || {
            let counter = counter_clone.clone();
            let response = response_data.clone();
            
            App::new()
                .route(
                    "/{tail:.*}",
                    web::post().to(mock_handler),
                )
                .app_data(web::Data::new(counter))
                .app_data(web::Data::new(response))
                .route(
                    "/health",
                    web::get().to(health_handler),
                )
        })
        .listen(listener)
        .unwrap()
        .run();
        
        // Run the server until completion
        server.await.unwrap();
    });
    
    // Store the handle for cleanup
    {
        let mut servers = ACTIVE_SERVERS.lock().unwrap();
        servers.push(server_handle);
    }
    
    // Give the server a moment to start
    tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;
    
    info!("Mock server '{}' started at {}", server_name_clone, server_url_clone);
    Ok(server_url_clone)
}

// Cleanup all active mock servers (async version)
pub async fn cleanup_mock_servers() {
    let servers = {
        // Use a safer approach that won't panic if mutex is poisoned
        match ACTIVE_SERVERS.lock() {
            Ok(mut servers) => std::mem::take(&mut *servers),
            Err(poisoned) => {
                // Even if poisoned, we can still access the data
                let mut servers = poisoned.into_inner();
                std::mem::take(&mut *servers)
            }
        }
    };
    
    let count = servers.len();
    if count > 0 {
        info!("Cleaning up {} mock servers", count);
        
        // Abort all server handles
        for handle in servers {
            handle.abort();
        }
        
        // Allow more time for servers to shut down properly
        tokio::time::sleep(tokio::time::Duration::from_millis(500)).await;
        
        info!("All mock servers have been cleaned up");
    }
}

// Sync version for use in destructors
pub fn cleanup_mock_servers_sync() {
    let servers = {
        // Use a safer approach that won't panic if mutex is poisoned
        match ACTIVE_SERVERS.lock() {
            Ok(mut servers) => std::mem::take(&mut *servers),
            Err(poisoned) => {
                // Even if poisoned, we can still access the data
                let mut servers = poisoned.into_inner();
                std::mem::take(&mut *servers)
            }
        }
    };
    
    // Abort all server handles
    for handle in servers {
        handle.abort();
    }
    
    // No sleep in sync version - the process is likely terminating anyway
}

// Handler for mock server requests
async fn mock_handler(
    body: web::Json<Value>, 
    counter: web::Data<RequestCounter>,
    response: web::Data<Value>
) -> HttpResponse {
    let request_num = counter.increment();
    info!(
        "Mock server received request #{}: {:?}",
        request_num, body
    );
    
    // Process response to replace $input placeholders with actual values
    let processed_response = if let Some(obj) = response.as_object() {
        let mut result = serde_json::Map::new();
        for (key, value) in obj {
            if let Value::String(s) = value {
                if s.starts_with("$input") {
                    // Handle path expressions like $input.type
                    if s.contains('.') {
                        let path = s.split('.').skip(1).collect::<Vec<_>>();
                        if let Some(field_value) = body.get(&path[0]) {
                            result.insert(key.clone(), field_value.clone());
                        } else {
                            result.insert(key.clone(), Value::String(s.clone()));
                        }
                    } else {
                        // $input by itself returns the whole body
                        result.insert(key.clone(), body.clone());
                    }
                } else {
                    result.insert(key.clone(), value.clone());
                }
            } else {
                result.insert(key.clone(), value.clone());
            }
        }
        Value::Object(result)
    } else {
        response.as_ref().clone()
    };
    
    // Return configured response
    info!(
        "Mock server responding with: {:?}",
        processed_response
    );
    HttpResponse::Ok().json(processed_response)
}

// Handler for health checks
async fn health_handler() -> HttpResponse {
    HttpResponse::Ok().body("OK")
}

// Helper to create a test graph with mock server URLs
pub fn create_test_graph(server_urls: &[(&str, &str)]) -> Value {
    let mut nodes = json!({});
    
    // Create a simple sequence node with steps for each server
    let mut steps = Vec::new();
    for (name, url) in server_urls {
        steps.push(json!({
            "stepName": name,
            "serviceUrl": url,
            "dependency": "hard"
        }));
    }
    
    nodes["root"] = json!({
        "routerType": "sequence",
        "steps": steps
    });
    
    json!({ "nodes": nodes })
}

// Helper to create a test graph for testing different router types
pub fn create_test_graph_with_types(
    sequence_urls: &[(&str, &str)],
    splitter_urls: &[(&str, &str, i64)],
    ensemble_urls: &[(&str, &str)],
    switch_urls: &[(&str, &str, &str)]
) -> Value {
    let mut nodes = json!({});
    
    // Create a sequence node in root
    let mut sequence_steps = Vec::new();
    for (name, url) in sequence_urls {
        sequence_steps.push(json!({
            "stepName": name,
            "serviceUrl": url,
            "dependency": "hard"
        }));
    }
    
    // Add references to other nodes if they have elements
    if !splitter_urls.is_empty() {
        sequence_steps.push(json!({
            "stepName": "splitter-step",
            "nodeName": "splitter-node",
            "dependency": "hard"
        }));
    }
    
    if !ensemble_urls.is_empty() {
        sequence_steps.push(json!({
            "stepName": "ensemble-step",
            "nodeName": "ensemble-node",
            "dependency": "hard"
        }));
    }
    
    if !switch_urls.is_empty() {
        sequence_steps.push(json!({
            "stepName": "switch-step",
            "nodeName": "switch-node",
            "dependency": "hard"
        }));
    }
    
    nodes["root"] = json!({
        "routerType": "sequence",
        "steps": sequence_steps
    });
    
    // Create a splitter node if we have splitter URLs
    if !splitter_urls.is_empty() {
        let mut splitter_steps = Vec::new();
        for (name, url, weight) in splitter_urls {
            splitter_steps.push(json!({
                "stepName": name,
                "serviceUrl": url,
                "weight": weight,
                "dependency": "hard"
            }));
        }
        
        nodes["splitter-node"] = json!({
            "routerType": "splitter",
            "steps": splitter_steps
        });
    }
    
    // Create an ensemble node if we have ensemble URLs
    if !ensemble_urls.is_empty() {
        let mut ensemble_steps = Vec::new();
        for (name, url) in ensemble_urls {
            ensemble_steps.push(json!({
                "stepName": name,
                "serviceUrl": url,
                "dependency": "hard"
            }));
        }
        
        nodes["ensemble-node"] = json!({
            "routerType": "ensemble",
            "steps": ensemble_steps
        });
    }
    
    // Create a switch node if we have switch URLs
    if !switch_urls.is_empty() {
        let mut switch_steps = Vec::new();
        for (name, url, condition) in switch_urls {
            switch_steps.push(json!({
                "stepName": name,
                "serviceUrl": url,
                "condition": condition,
                "dependency": "hard"
            }));
        }
        
        nodes["switch-node"] = json!({
            "routerType": "switch",
            "steps": switch_steps
        });
    }
    
    json!({ "nodes": nodes })
}

/// Helper function to run a test function and ensure cleanup
/// regardless of whether the test passes or fails
pub async fn run_test_with_cleanup<F, Fut, T>(test_func: F) -> Result<T, Box<dyn std::error::Error>>
where
    F: FnOnce() -> Fut,
    Fut: std::future::Future<Output = Result<T, Box<dyn std::error::Error>>>,
{
    // Run the test
    let result = test_func().await;
    
    // Always clean up, regardless of test result
    cleanup_mock_servers().await;
    
    // Return the original result
    result
} 