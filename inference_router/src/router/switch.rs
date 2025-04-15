use futures::future::{BoxFuture, FutureExt};
use log::error;

use crate::models::{
    InferenceNode,
    InferenceGraphSpec,
    RouterError,
};
use crate::router::common::{RouterResult, PropagatedHeaders, pickup_route_by_condition};
use super::utils::execute_step;

/// Handle a switch node which selects a route based on condition
pub fn handle_switch_node<'a>(
    request_id: &'a str,
    node: &'a InferenceNode,
    graph: &'a InferenceGraphSpec,
    input: &'a [u8],
    headers: &'a PropagatedHeaders
) -> BoxFuture<'a, RouterResult> {
    async move {
        // Pick a route based on condition in input
        let route = match pickup_route_by_condition(input, &node.steps) {
            Some(r) => r,
            None => {
                error!("[{}] No route matched the condition in switch node", request_id);
                return Err(RouterError::NoMatchingConditionError);
            }
        };
        
        // Execute the selected route
        execute_step(request_id, &route, graph, input, headers).await
    }.boxed()
} 