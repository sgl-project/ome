use actix_web::{web, HttpRequest, HttpResponse, Responder};
use log::{info, warn, error};
use serde_json::json;
use std::sync::atomic::Ordering;
use uuid::Uuid;
use actix_web::http::header::{HeaderName, HeaderValue};
use crate::state::AppState;
use crate::router;

/// Main router handler for all graph routing requests
pub async fn graph_handler(
    req: HttpRequest,
    body: web::Bytes,
    data: web::Data<AppState>,
) -> impl Responder {
    // Get or generate a request ID
    let request_id = req
        .headers()
        .get("X-Request-ID")
        .and_then(|h| h.to_str().ok())
        .map(|s| s.to_string())
        .unwrap_or_else(|| Uuid::new_v4().to_string());
    
    info!("[{}] Received request: {} {}", request_id, req.method(), req.uri());
    
    // Check if the server is shutting down
    if data.is_shutting_down.load(Ordering::SeqCst) {
        warn!("[{}] Request received during shutdown: {} {}", request_id, req.method(), req.uri());
        let response = HttpResponse::ServiceUnavailable().json(json!({
            "error": "Service is shutting down",
            "message": "New requests are not being accepted",
            "request_id": request_id
        }));
        info!("[{}] Responding with status: {}", request_id, response.status());
        return response;
    }
    
    // Increment active requests counter
    {
        let mut active_requests = data.active_requests.lock().unwrap();
        *active_requests += 1;
    }
    
    // Use a guard to ensure the counter is decremented when the function returns
    let _guard = scopeguard::guard((), |_| {
        let mut active_requests = data.active_requests.lock().unwrap();
        *active_requests -= 1;
    });
    
    // Extract important headers to propagate
    let mut headers_to_propagate = Vec::new();
    if let Some(header_patterns) = &data.headers_to_propagate {
        for pattern in header_patterns {
            for (name, value) in req.headers().iter() {
                if name.as_str().contains(pattern) {
                    info!("[{}] Will propagate header: {}: {:?}", request_id, name, value);
                    headers_to_propagate.push((name.clone(), value.clone()));
                }
            }
        }
    }
    
    // Always propagate or create X-Request-ID header
    let request_id_header = HeaderName::from_static("x-request-id");
    if !headers_to_propagate.iter().any(|(name, _)| name == &request_id_header) {
        match HeaderValue::from_str(&request_id) {
            Ok(header_value) => {
                headers_to_propagate.push((request_id_header, header_value));
            },
            Err(e) => {
                warn!("[{}] Failed to create X-Request-ID header value: {}", request_id, e);
            }
        }
    }
    
    // Process the request through the graph root node
    match router::route_request(&request_id, &data.graph, &body, &headers_to_propagate).await {
        Ok((response_body, status_code)) => {
            let mut response = HttpResponse::build(actix_web::http::StatusCode::from_u16(status_code).unwrap_or(actix_web::http::StatusCode::OK));
            
            // Add request ID to the response headers
            response.insert_header(("X-Request-ID", request_id.clone()));
            
            // Set content type if the response is JSON
            if let Ok(value) = serde_json::from_slice::<serde_json::Value>(&response_body) {
                let json_response = response.content_type("application/json").json(value);
                info!("[{}] Responding with status: {}", request_id, json_response.status());
                return json_response;
            }
            
            // Return raw response if not JSON
            let raw_response = response.body(response_body);
            info!("[{}] Responding with status: {}", request_id, raw_response.status());
            raw_response
        },
        Err(err) => {
            error!("[{}] Error routing request: {}", request_id, err);
            let error_response = router::prepare_error_response(&err, "Failed to process request");
            
            let response = HttpResponse::InternalServerError()
                .content_type("application/json")
                .insert_header(("X-Request-ID", request_id.clone()))
                .body(error_response);
                
            info!("[{}] Responding with status: {}", request_id, response.status());
            response
        }
    }
} 