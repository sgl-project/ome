/// Testing utilities for the inference router
use std::sync::{Arc, Mutex};
use actix_web::{web, App, HttpRequest, HttpResponse, HttpServer};
use serde_json::{json, Value};
use log::{info, debug};
use std::collections::HashMap;
use std::net::{SocketAddr, IpAddr, Ipv4Addr};
use uuid::Uuid;

/// Counter for requests to a mock server
#[derive(Debug, Default)]
pub struct RequestCounter {
    pub total: usize,
    pub by_path: HashMap<String, usize>,
}

/// Mock server configuration and state
pub struct MockServer {
    pub name: String,
    pub counter: Arc<Mutex<RequestCounter>>,
    pub response_delay_ms: u64,
    pub response_status: u16,
    pub default_response: Value,
    pub fixed_port: Option<u16>,
}

impl MockServer {
    /// Create a new mock server with default settings
    pub fn new(name: &str) -> Self {
        Self {
            name: name.to_string(),
            counter: Arc::new(Mutex::new(RequestCounter::default())),
            response_delay_ms: 0,
            response_status: 200,
            default_response: json!({"status": "ok"}),
            fixed_port: None,
        }
    }

    /// Set a fixed port for the server
    pub fn with_port(mut self, port: u16) -> Self {
        self.fixed_port = Some(port);
        self
    }

    /// Set a delay for responses
    pub fn with_delay(mut self, delay_ms: u64) -> Self {
        self.response_delay_ms = delay_ms;
        self
    }

    /// Set the response status code
    pub fn with_status(mut self, status: u16) -> Self {
        self.response_status = status;
        self
    }

    /// Set the default response
    pub fn with_response(mut self, response: Value) -> Self {
        self.default_response = response;
        self
    }

    /// Start the mock server
    pub async fn start(self) -> (Arc<Mutex<RequestCounter>>, actix_web::dev::Server, String) {
        // Clone the counter to return and to use inside the closure
        let counter = self.counter.clone();
        let counter_return = counter.clone();

        let delay = self.response_delay_ms;
        let status = self.response_status;
        let response = self.default_response.clone();
        let name = self.name.clone();

        // Determine address to bind to
        let addr = if let Some(port) = self.fixed_port {
            SocketAddr::new(IpAddr::V4(Ipv4Addr::new(0, 0, 0, 0)), port)
        } else {
            SocketAddr::new(IpAddr::V4(Ipv4Addr::new(0, 0, 0, 0)), 0)
        };

        // Create the server
        let server = HttpServer::new(move || {
            let app_counter = counter.clone();
            App::new()
                .app_data(web::Data::new(app_counter))
                .app_data(web::Data::new(delay))
                .app_data(web::Data::new(status))
                .app_data(web::Data::new(response.clone()))
                .app_data(web::Data::new(name.clone()))
                .default_service(web::route().to(mock_handler))
        })
        .bind(addr)
        .expect("Failed to bind mock server");

        // Get the socket address before running the server
        let socket_addr = match server.addrs().get(0) {
            Some(addr) => addr.clone(),
            None => {
                panic!("Failed to get server address");
            }
        };
        
        // Get the URL for clients to connect to
        let url = format!("http://{}", socket_addr);
        
        // Start the server
        let server = server.run();
        
        info!("Started mock server '{}' on {}", self.name, socket_addr);

        // Return the counter, server, and URL
        (counter_return, server, url)
    }
}

/// Handler for all requests to the mock server
async fn mock_handler(
    req: HttpRequest,
    body: web::Bytes,
    counter: web::Data<Arc<Mutex<RequestCounter>>>,
    delay: web::Data<u64>,
    status: web::Data<u16>,
    response: web::Data<Value>,
    server_name: web::Data<String>,
) -> HttpResponse {
    // Debug log the incoming request
    println!("Mock server '{}' received request to {}", server_name.as_ref(), req.uri().path());
    
    // Try to extract the request body for debug
    let body_str = String::from_utf8_lossy(&body);
    println!("Request body: {}", body_str);
    
    // Increment counters
    let path = req.uri().path().to_string();
    {
        let mut counter = counter.lock().unwrap();
        counter.total += 1;
        *counter.by_path.entry(path.clone()).or_insert(0) += 1;
    }

    // Generate a request ID to simulate a real service
    let request_id = req
        .headers()
        .get("X-Request-ID")
        .and_then(|h| h.to_str().ok())
        .map(|s| s.to_string())
        .unwrap_or_else(|| Uuid::new_v4().to_string());

    info!(
        "[{}] Mock server '{}' received {} request to {}",
        request_id, server_name.as_str(), req.method(), path
    );

    debug!(
        "[{}] Request body: {:?}",
        request_id,
        String::from_utf8_lossy(&body)
    );

    // Simulate processing delay if configured
    if **delay > 0 {
        println!("Applying delay of {}ms", **delay);
        tokio::time::sleep(tokio::time::Duration::from_millis(**delay)).await;
    }

    // Create a modified response that includes the request info
    let mut resp_obj = match response.as_object() {
        Some(obj) => obj.clone(),
        None => {
            // Default to empty object if response is not an object
            println!("Warning: Response was not a JSON object");
            serde_json::Map::new()
        }
    };
    
    // Add request metadata to response
    resp_obj.insert("request_id".to_string(), json!(request_id));
    resp_obj.insert("server".to_string(), json!(server_name.as_str()));
    resp_obj.insert("path".to_string(), json!(path));
    
    // Add a timestamp
    let timestamp = chrono::Utc::now().to_rfc3339();
    resp_obj.insert("timestamp".to_string(), json!(timestamp));
    
    // Try to include the request body if it's valid JSON
    if let Ok(req_json) = serde_json::from_slice::<Value>(&body) {
        resp_obj.insert("request".to_string(), req_json);
    } else if !body.is_empty() {
        // Include as string if not valid JSON
        resp_obj.insert(
            "request".to_string(),
            json!(String::from_utf8_lossy(&body).to_string()),
        );
    }

    println!("Sending response for request {}", request_id);
    
    // Return response with configured status code
    HttpResponse::build(actix_web::http::StatusCode::from_u16(**status).unwrap())
        .insert_header(("X-Request-ID", request_id))
        .insert_header(("Content-Type", "application/json"))
        .json(resp_obj)
}

/// Start three mock servers for integration testing
pub async fn start_mock_servers() -> (
    (Arc<Mutex<RequestCounter>>, actix_web::dev::Server, String),
    (Arc<Mutex<RequestCounter>>, actix_web::dev::Server, String),
    (Arc<Mutex<RequestCounter>>, actix_web::dev::Server, String),
) {
    // Create classification service
    let classification_server = MockServer::new("classification")
        .with_response(json!({
            "classification": "passed",
            "confidence": 0.95,
        }));

    // Create text processing service
    let text_server = MockServer::new("text-processor")
        .with_response(json!({
            "processed": true,
            "type": "text",
            "result": "This is processed text",
        }));

    // Create image processing service
    let image_server = MockServer::new("image-processor")
        .with_response(json!({
            "processed": true,
            "type": "image",
            "dimensions": {
                "width": 800,
                "height": 600
            }
        }));

    // Start all servers
    let classification = classification_server.start().await;
    let text = text_server.start().await;
    let image = image_server.start().await;

    (classification, text, image)
} 