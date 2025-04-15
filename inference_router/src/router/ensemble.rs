use std::collections::HashMap;
use futures::future::{BoxFuture, FutureExt, join_all};
use log::{info, error, warn};
use serde_json::{json, Value};

use crate::models::{
    InferenceNode,
    InferenceGraphSpec,
    RouterError,
    DependencyType,
};
use crate::router::common::{RouterResult, PropagatedHeaders, is_successful};
use super::utils::execute_step;

/// Handle an ensemble node which executes steps in parallel
pub fn handle_ensemble_node<'a>(
    request_id: &'a str,
    node: &'a InferenceNode,
    graph: &'a InferenceGraphSpec,
    input: &'a [u8],
    headers: &'a PropagatedHeaders
) -> BoxFuture<'a, RouterResult> {
    async move {
        let mut futures = Vec::new();
        
        // Create a future for each step
        for (i, step) in node.steps.iter().enumerate() {
            let step_clone = step.clone();
            let input_clone = input.to_vec();
            let headers_clone = headers.clone();
            let graph_ref = graph;
            let request_id_clone = request_id.to_string();
            
            let future = async move {
                let step_name = if !step_clone.step_name.is_empty() {
                    step_clone.step_name.clone()
                } else {
                    i.to_string()
                };
                
                let result = execute_step(&request_id_clone, &step_clone, graph_ref, &input_clone, &headers_clone).await;
                (step_name, step_clone, result)
            };
            
            futures.push(future);
        }
        
        // Execute futures in parallel
        let results = join_all(futures).await;
        
        // Process results
        let mut response = HashMap::new();
        
        for (step_name, step, result) in results {
            match result {
                Ok((output, status)) => {
                    // For hard dependencies, if unsuccessful, return the error immediately
                    if step.dependency == Some(DependencyType::Hard) && !is_successful(status) {
                        info!("[{}] Step {} is a hard dependency and it failed with status {}", 
                            request_id, step_name, status);
                        return Ok((output, status));
                    }
                    
                    // Otherwise, add to the combined response
                    if let Ok(value) = serde_json::from_slice::<Value>(&output) {
                        response.insert(step_name, value);
                    } else {
                        warn!("[{}] Could not parse JSON response from step {}", request_id, step_name);
                        // Insert the raw output as a string
                        response.insert(step_name, json!(String::from_utf8_lossy(&output).to_string()));
                    }
                },
                Err(err) => {
                    error!("[{}] Error executing step {}: {}", request_id, step_name, err);
                    if step.dependency == Some(DependencyType::Hard) {
                        return Err(err);
                    }
                    // For soft dependencies, add error information to the response
                    response.insert(step_name, json!({
                        "error": err.to_string()
                    }));
                }
            }
        }
        
        // Create the combined response
        match serde_json::to_vec(&response) {
            Ok(bytes) => Ok((bytes, 200)),
            Err(err) => Err(RouterError::JsonError(err)),
        }
    }.boxed()
} 