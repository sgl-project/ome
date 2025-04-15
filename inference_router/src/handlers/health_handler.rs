use actix_web::{web, HttpRequest, HttpResponse};
use log::{info, warn};
use uuid::Uuid;
use std::sync::atomic::Ordering;
use crate::state::AppState;

/// Health check handler
pub async fn health_check(req: HttpRequest, data: web::Data<AppState>) -> HttpResponse {
    // Get or generate a request ID
    let request_id = req
        .headers()
        .get("X-Request-ID")
        .and_then(|h| h.to_str().ok())
        .map(|s| s.to_string())
        .unwrap_or_else(|| Uuid::new_v4().to_string());
    
    info!("[{}] Health check received", request_id);
    
    // Check if the server is shutting down
    if data.is_shutting_down.load(Ordering::SeqCst) {
        warn!("[{}] Health check received during shutdown", request_id);
        let mut response = HttpResponse::ServiceUnavailable().body("Service is shutting down");
        response.headers_mut().insert(
            actix_web::http::header::HeaderName::from_static("x-request-id"),
            match actix_web::http::header::HeaderValue::from_str(&request_id) {
                Ok(v) => v,
                Err(_) => return response, // Return without header if invalid
            }
        );
        info!("[{}] Responding with status: {}", request_id, response.status());
        return response;
    }
    
    let mut response = HttpResponse::Ok().body("OK");
    response.headers_mut().insert(
        actix_web::http::header::HeaderName::from_static("x-request-id"),
        match actix_web::http::header::HeaderValue::from_str(&request_id) {
            Ok(v) => v,
            Err(_) => return response, // Return without header if invalid
        }
    );
    info!("[{}] Responding with status: {}", request_id, response.status());
    response
} 