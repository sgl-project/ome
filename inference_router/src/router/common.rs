use std::time::Instant;
use log::{info, debug, error};
use reqwest::Client;
use actix_web::http::header::{HeaderName, HeaderValue};
use serde_json::{Value};
use rand::Rng;

use crate::models::{
    InferenceStep,
    RouterError,
    InferenceGraphRoutingError,
};

pub type RouterResult = Result<(Vec<u8>, u16), RouterError>;
pub type PropagatedHeaders = Vec<(HeaderName, HeaderValue)>;

/// Helper function to create a standard error response
pub fn prepare_error_response(err: &RouterError, message: &str) -> Vec<u8> {
    let error = InferenceGraphRoutingError {
        message: message.to_string(),
        error: err.to_string(),
    };
    
    match serde_json::to_vec(&error) {
        Ok(bytes) => bytes,
        Err(e) => {
            error!("Failed to serialize error response: {}", e);
            format!("{{\"message\":\"{}\",\"error\":\"{}\"}}", message, err).into_bytes()
        }
    }
}

/// Time tracking helper function
pub fn time_track(start: Instant, node_or_step: &str, name: &str) {
    let elapsed = start.elapsed();
    info!("elapsed time {}={} time={:?}", node_or_step, name, elapsed);
}

/// Check if HTTP status code indicates success (2xx)
pub fn is_successful(status_code: u16) -> bool {
    status_code >= 200 && status_code < 300
}

/// Call an external service with the given input
pub async fn call_service(service_url: &str, input: &[u8], headers: &PropagatedHeaders) -> RouterResult {
    let start = Instant::now();
    let service_name = service_url.to_string();
    debug!("Entering call_service for {}", service_url);
    
    // Create request client
    let client = Client::new();
    let mut request_builder = client.post(service_url)
        .body(input.to_vec())
        .header("Content-Type", "application/json");
    
    // Add propagated headers
    for (name, value) in headers {
        request_builder = request_builder.header(name.clone(), value.clone());
    }
    
    // Execute the request
    let response = match request_builder.send().await {
        Ok(resp) => resp,
        Err(err) => {
            error!("Error calling service {}: {}", service_url, err);
            return Err(RouterError::ServiceError(err.to_string()));
        }
    };
    
    // Read status code
    let status = response.status().as_u16();
    
    // Read response body
    let body = match response.bytes().await {
        Ok(bytes) => bytes.to_vec(),
        Err(err) => {
            error!("Error reading response from {}: {}", service_url, err);
            return Err(RouterError::ServiceError(err.to_string()));
        }
    };
    
    // Track timing
    time_track(start, "step", &service_name);
    
    Ok((body, status))
}

/// Pick a route based on weights
pub fn pickup_route(routes: &[InferenceStep]) -> Option<InferenceStep> {
    if routes.is_empty() {
        return None;
    }

    // Check if any weights are specified
    let has_weights = routes.iter().any(|route| route.weight.is_some());
    
    if !has_weights {
        // If no weights are specified, use equal distribution
        let mut rng = rand::thread_rng();
        let index = rng.gen_range(0..routes.len());
        return Some(routes[index].clone());
    }

    let mut rng = rand::thread_rng();
    let point: i32 = rng.gen_range(0..101); // [0, 100]
    
    let mut end = 0;
    for route in routes {
        if let Some(weight) = route.weight {
            end += weight;
            if point < end {
                return Some(route.clone());
            }
        }
    }
    
    // If we get here and no route was selected but we have routes,
    // select the last route to ensure we always return something
    if !routes.is_empty() {
        return Some(routes.last().unwrap().clone());
    }
    
    None
}

/// Pick a route based on condition
pub fn pickup_route_by_condition(input: &[u8], routes: &[InferenceStep]) -> Option<InferenceStep> {
    if let Ok(value) = serde_json::from_slice::<Value>(input) {
        log::debug!("Input for condition evaluation: {:?}", value);
        
        for route in routes {
            if let Some(condition) = &route.condition {
                log::debug!("Evaluating condition: {:?}", condition);
                
                // Handle boolean conditions (e.g. "type.text")
                if !condition.contains("==") {
                    let path = condition.replace(".", "/");
                    log::debug!("Boolean condition path: /{}", path);
                    
                    if let Some(actual_value) = value.pointer(&format!("/{}", path)) {
                        log::debug!("Found value at path: {:?}", actual_value);
                        
                        // Check if the value is a boolean or if it's a nested object with a boolean field
                        if actual_value.as_bool() == Some(true) || 
                           (actual_value.is_object() && actual_value.get("value").and_then(|v| v.as_bool()) == Some(true)) {
                            log::debug!("Boolean condition matched");
                            return Some(route.clone());
                        }
                    } else {
                        log::debug!("No value found at path: /{}", path);
                    }
                    continue;
                }
                
                // Handle equality conditions (e.g. "type == text")
                let parts: Vec<&str> = condition.split(" == ").collect();
                if parts.len() != 2 {
                    log::debug!("Invalid condition format: {:?}", condition);
                    continue;
                }
                
                let path = parts[0].replace(".", "/");
                let expected_value = parts[1].trim_matches('"').replace("\\\"", "\"");
                
                log::debug!("Equality condition path: /{}, expected value: {:?}", path, expected_value);
                
                // Get the actual value from the input
                if let Some(actual_value) = value.pointer(&format!("/{}", path)) {
                    log::debug!("Found value at path: {:?}", actual_value);
                    
                    // Compare the values
                    if let Some(actual_str) = actual_value.as_str() {
                        log::debug!("Comparing {} with {}", actual_str, expected_value);
                        if actual_str == expected_value {
                            log::debug!("Equality condition matched");
                            return Some(route.clone());
                        }
                    } else {
                        log::debug!("Value is not a string: {:?}", actual_value);
                    }
                } else {
                    log::debug!("No value found at path: /{}", path);
                }
            } else {
                // No condition means default route
                log::debug!("Using default route (no condition)");
                return Some(route.clone());
            }
        }
    } else {
        log::debug!("Failed to parse input as JSON for condition evaluation");
    }
    
    None
} 