use std::time::Instant;
use futures::future::{BoxFuture, FutureExt};
use log::{error};

use crate::models::{
    InferenceGraphSpec,
    InferenceStep,
    RouterType,
    RouterError,
    GRAPH_ROOT_NODE_NAME,
};
use crate::router::common::{RouterResult, PropagatedHeaders, time_track, call_service};

/// Execute a single step in the inference graph
pub fn execute_step<'a>(
    request_id: &'a str,
    step: &'a InferenceStep,
    graph: &'a InferenceGraphSpec,
    input: &'a [u8],
    headers: &'a PropagatedHeaders
) -> BoxFuture<'a, RouterResult> {
    async move {
        // If step refers to a node, route to that node
        if let Some(node_name) = &step.node_name {
            return route_step(request_id, node_name, graph, input, headers).await;
        }
        
        // Otherwise, call the service URL
        if let Some(service_url) = &step.service_url {
            return call_service(service_url, input, headers).await;
        }
        
        // If we get here, the step is invalid
        error!("[{}] Step {} has neither a node_name nor a service_url", request_id, step.step_name);
        Err(RouterError::MissingTargetError(step.step_name.clone()))
    }.boxed()
}

/// Route a request through a specific node in the inference graph
pub fn route_step<'a>(
    request_id: &'a str, 
    node_name: &'a str, 
    graph: &'a InferenceGraphSpec, 
    input: &'a [u8],
    headers: &'a PropagatedHeaders
) -> BoxFuture<'a, RouterResult> {
    async move {
        let start = Instant::now();
        
        // Try to find the node in the graph
        let current_node = match graph.nodes.get(node_name) {
            Some(node) => node,
            None => {
                error!("[{}] Node not found: {}", request_id, node_name);
                return Err(RouterError::NodeNotFoundError(node_name.to_string()));
            }
        };
        
        let result = match current_node.router_type {
            RouterType::Splitter => {
                crate::router::splitter::handle_splitter_node(request_id, current_node, graph, input, headers).await
            },
            RouterType::Switch => {
                crate::router::switch::handle_switch_node(request_id, current_node, graph, input, headers).await
            },
            RouterType::Ensemble => {
                crate::router::ensemble::handle_ensemble_node(request_id, current_node, graph, input, headers).await
            },
            RouterType::Sequence => {
                crate::router::sequence::handle_sequence_node(request_id, current_node, graph, input, headers).await
            },
        };
        
        // Track timing for the node
        time_track(start, "node", node_name);
        
        result
    }.boxed()
}

/// Main entry point for routing a request through the graph
pub async fn route_request(
    request_id: &str,
    graph: &InferenceGraphSpec,
    input: &[u8],
    headers: &PropagatedHeaders
) -> RouterResult {
    route_step(request_id, GRAPH_ROOT_NODE_NAME, graph, input, headers).await
} 