use futures::future::{BoxFuture, FutureExt};
use log::{info, error};
use serde_json::Value;

use crate::models::{
    InferenceNode,
    InferenceGraphSpec,
    DependencyType,
};
use crate::router::common::{RouterResult, PropagatedHeaders, is_successful};
use super::utils::{execute_step};

/// Handle a sequence node which executes steps in order
pub fn handle_sequence_node<'a>(
    request_id: &'a str,
    node: &'a InferenceNode,
    graph: &'a InferenceGraphSpec,
    input: &'a [u8],
    headers: &'a PropagatedHeaders
) -> BoxFuture<'a, RouterResult> {
    async move {
        let mut current_input = input.to_vec();
        let mut status_code = 200;
        
        for step in &node.steps {
            let step_type = if step.node_name.is_some() { "node" } else { "serviceUrl" };
            info!("[{}] Starting execution of step type={} stepName={}", 
                request_id, step_type, step.step_name);
            
            // Determine the request for this step
            let request = if step.data.as_deref() == Some("$response") && !current_input.is_empty() {
                current_input.clone()
            } else {
                input.to_vec()
            };
            
            // Check if there's a condition on the previous output
            if let Some(condition) = &step.condition {
                if !serde_json::from_slice::<Value>(&current_input).map_or(false, |v| {
                    v.pointer(&format!("/{}", condition.replace(".", "/"))).is_some()
                }) {
                    info!("[{}] Condition '{}' not met, skipping step {}", 
                        request_id, condition, step.step_name);
                    return Ok((current_input, status_code));
                }
            }
            
            // Execute the step
            match execute_step(request_id, step, graph, &request, headers).await {
                Ok((output, code)) => {
                    current_input = output;
                    status_code = code;
                    
                    // Check if hard dependency failed
                    if step.dependency == Some(DependencyType::Hard) && !is_successful(code) {
                        info!("[{}] Step {} is a hard dependency and it failed with status {}", 
                            request_id, step.step_name, code);
                        return Ok((current_input, status_code));
                    }
                },
                Err(err) => {
                    error!("[{}] Error executing step {}: {}", request_id, step.step_name, err);
                    return Err(err);
                }
            }
        }
        
        Ok((current_input, status_code))
    }.boxed()
} 