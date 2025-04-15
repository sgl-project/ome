use log::info;
use std::collections::{HashMap, HashSet, VecDeque};

use crate::models::error::RouterError;
use crate::models::{
    InferenceGraphSpec
};

/// Run comprehensive validation on an inference graph
pub fn validate_graph_structure(graph: &InferenceGraphSpec) -> Result<(), RouterError> {
    // Check for cycles in the graph
    validate_graph_cycles(graph)?;
    
    // Check for unreachable nodes
    validate_reachable_nodes(graph)?;
    
    // Check for valid router configurations
    validate_router_configurations(graph)?;
    
    Ok(())
}

/// Validate that the graph doesn't contain any cycles
pub fn validate_graph_cycles(graph: &InferenceGraphSpec) -> Result<(), RouterError> {
    info!("Checking for cycles in the graph...");
    
    // Build adjacency list to represent the graph

    let adjacency_list = get_graph_adjacency_list(&graph);
    
    // Check for cycles using DFS
    let mut visited = HashSet::new();
    let mut stack = HashSet::new();
    
    for node in adjacency_list.keys() {
        if !visited.contains(node) {
            if has_cycle(&adjacency_list, node, &mut visited, &mut stack) {
                return Err(RouterError::GraphParseError(format!(
                    "Cycle detected in graph involving node '{}'", node
                )));
            }
        }
    }
    
    info!("No cycles detected in the graph");
    Ok(())
}

fn get_graph_adjacency_list(graph: &&InferenceGraphSpec) -> HashMap<String, Vec<String>> {
    let mut adjacency_list: HashMap<String, Vec<String>> = HashMap::new();

    for (node_name, node) in &graph.nodes {
        let mut neighbors = Vec::new();

        for step in &node.steps {
            if let Some(target_node) = &step.node_name {
                neighbors.push(target_node.clone());
            }
        }

        adjacency_list.insert(node_name.clone(), neighbors);
    }
    adjacency_list
}

/// Helper function for cycle detection using DFS
fn has_cycle(
    adjacency_list: &HashMap<String, Vec<String>>,
    node: &String,
    visited: &mut HashSet<String>,
    stack: &mut HashSet<String>,
) -> bool {
    visited.insert(node.clone());
    stack.insert(node.clone());
    
    if let Some(neighbors) = adjacency_list.get(node) {
        for neighbor in neighbors {
            if !visited.contains(neighbor) {
                if has_cycle(adjacency_list, neighbor, visited, stack) {
                    return true;
                }
            } else if stack.contains(neighbor) {
                return true;
            }
        }
    }
    
    stack.remove(node);
    false
}

/// Validate that all nodes in the graph are reachable from the root node
pub fn validate_reachable_nodes(graph: &InferenceGraphSpec) -> Result<(), RouterError> {
    info!("Checking for unreachable nodes...");

    // Check that root node exists
    if !graph.nodes.contains_key("root") {
        return Err(RouterError::NodeNotFoundError(
            "Root node 'root' not found in the graph".to_string()
        ));
    }

    let adjacency_list = get_graph_adjacency_list(&graph);
    
    // Run BFS from root to find all reachable nodes
    let mut reachable = HashSet::new();
    let mut queue = VecDeque::new();
    
    queue.push_back("root".to_string());
    reachable.insert("root".to_string());
    
    while let Some(node) = queue.pop_front() {
        if let Some(neighbors) = adjacency_list.get(&node) {
            for neighbor in neighbors {
                if !reachable.contains(neighbor) {
                    reachable.insert(neighbor.clone());
                    queue.push_back(neighbor.clone());
                }
            }
        }
    }
    
    // Check for unreachable nodes
    let unreachable: Vec<String> = graph.nodes.keys()
        .filter(|k| !reachable.contains(*k))
        .cloned()
        .collect();
    
    if !unreachable.is_empty() {
        return Err(RouterError::ConfigError(format!(
            "Unreachable nodes detected: {}", unreachable.join(", ")
        )));
    }
    
    info!("All nodes are reachable from the root");
    Ok(())
}

/// Validate router-specific configurations
pub fn validate_router_configurations(graph: &InferenceGraphSpec) -> Result<(), RouterError> {
    info!("Validating router configurations...");
    
    for (node_name, node) in &graph.nodes {
        match node.router_type {
            crate::models::RouterType::Switch => {
                // For switch routers, check that at least one step has a condition
                // or the last step doesn't have a condition (default case)
                let has_condition = node.steps.iter().any(|s| s.condition.is_some());
                let last_has_no_condition = node.steps.last()
                    .map(|s| s.condition.is_none())
                    .unwrap_or(false);
                
                if node.steps.len() > 1 && !has_condition {
                    return Err(RouterError::InvalidConditionError(format!(
                        "Switch router '{}' has multiple steps but no conditions", node_name
                    )));
                }
                
                if node.steps.len() > 1 && !last_has_no_condition {
                    return Err(RouterError::InvalidConditionError(format!(
                        "Switch router '{}' should have its last step without a condition as a default case", node_name
                    )));
                }
            },
            crate::models::RouterType::Splitter => {
                // For splitter routers, check that all steps have weights and they sum to 100
                let all_have_weights = node.steps.iter().all(|s| s.weight.is_some());
                let sum_of_weights: i32 = node.steps.iter()
                    .filter_map(|s| s.weight)
                    .sum();
                
                if !all_have_weights {
                    return Err(RouterError::MissingTargetError(format!(
                        "Splitter router '{}' has steps without weights", node_name
                    )));
                }
                
                if sum_of_weights != 100 {
                    return Err(RouterError::ConfigError(format!(
                        "Splitter router '{}' has weights that don't sum to 100 (sum: {})", 
                        node_name, sum_of_weights
                    )));
                }
            },
            crate::models::RouterType::Ensemble => {
                // For ensemble routers, check that all steps have a service URL
                let all_have_service_url = node.steps.iter()
                    .all(|s| s.service_url.is_some());
                
                if !all_have_service_url {
                    return Err(RouterError::MissingTargetError(format!(
                        "Ensemble router '{}' has steps without service URLs", node_name
                    )));
                }

                // Check if there are no valid steps
                if node.steps.is_empty() {
                    return Err(RouterError::NoValidEnsembleStepsError);
                }
            },
            _ => {} // No special validation for other router types
        }
    }
    
    info!("Router configurations validated successfully");
    Ok(())
}

/// Helper function to print detailed information about a graph
pub fn print_graph_details(graph: &InferenceGraphSpec) {
    info!("Graph has {} nodes:", graph.nodes.len());
    
    for (name, node) in &graph.nodes {
        info!("  Node '{}' ({:?}): {} steps", name, node.router_type, node.steps.len());
        
        for step in &node.steps {
            let target = if let Some(ref node_name) = step.node_name {
                format!("node '{}'", node_name)
            } else if let Some(ref service_url) = step.service_url {
                format!("service '{}'", service_url)
            } else {
                "unknown target".to_string()
            };
            
            info!("    Step '{}': {}", step.step_name, target);
        }
    }
} 