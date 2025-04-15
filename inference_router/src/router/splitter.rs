use futures::future::{BoxFuture, FutureExt};
use log::error;

use crate::models::{
    InferenceNode,
    InferenceGraphSpec,
};
use crate::router::common::{RouterResult, PropagatedHeaders, pickup_route};
use super::utils::execute_step;

/// Handle a splitter node which selects a route based on weights
pub fn handle_splitter_node<'a>(
    request_id: &'a str,
    node: &'a InferenceNode,
    graph: &'a InferenceGraphSpec,
    input: &'a [u8],
    headers: &'a PropagatedHeaders
) -> BoxFuture<'a, RouterResult> {
    async move {
        // Pick a route based on weights
        let route = match pickup_route(&node.steps) {
            Some(r) => r,
            None => {
                error!("[{}] Failed to pick a route in splitter node", request_id);
                return Err(crate::models::RouterError::ServiceError(
                    "Failed to pick a route in splitter node".to_string()
                ));
            }
        };
        
        // Execute the selected route
        execute_step(request_id, &route, graph, input, headers).await
    }.boxed()
} 