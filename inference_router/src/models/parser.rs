use std::fs;
use std::path::Path;

use log::{debug, error};
use serde_json::Value;

use crate::models::inference_graph::InferenceGraphSpec;
use crate::models::error::RouterError;

/// Parse an inference graph from a JSON file
///
/// # Arguments
///
/// * `file_path` - Path to the JSON file containing the inference graph
///
/// # Returns
///
/// * `Result<InferenceGraphSpec, RouterError>` - The parsed inference graph or an error
pub fn parse_graph_from_file<P: AsRef<Path>>(file_path: P) -> Result<InferenceGraphSpec, RouterError> {
    debug!("Parsing inference graph from file: {:?}", file_path.as_ref());
    
    // Read the file
    let content = fs::read_to_string(file_path)
        .map_err(|e| RouterError::IoError(e))?;
    
    // Parse from string
    parse_graph_from_string(&content)
}

/// Parse an inference graph from a JSON string
///
/// # Arguments
///
/// * `json_str` - JSON string containing the inference graph
///
/// # Returns
///
/// * `Result<InferenceGraphSpec, RouterError>` - The parsed inference graph or an error
pub fn parse_graph_from_string(json_str: &str) -> Result<InferenceGraphSpec, RouterError> {
    debug!("Parsing inference graph from string");
    
    // Parse the JSON
    let graph_spec: InferenceGraphSpec = serde_json::from_str(json_str)
        .map_err(|e| {
            error!("Failed to parse inference graph JSON: {}", e);
            RouterError::GraphParseError(format!("JSON parsing error: {}", e))
        })?;
    
    // Validate the graph
    validate_graph(&graph_spec)?;
    
    Ok(graph_spec)
}

/// Parse an inference graph from a JSON value
///
/// # Arguments
///
/// * `value` - JSON value containing the inference graph
///
/// # Returns
///
/// * `Result<InferenceGraphSpec, RouterError>` - The parsed inference graph or an error
pub fn parse_graph_from_value(value: Value) -> Result<InferenceGraphSpec, RouterError> {
    debug!("Parsing inference graph from Value");
    
    // Convert the Value to an InferenceGraphSpec
    let graph_spec: InferenceGraphSpec = serde_json::from_value(value)
        .map_err(|e| {
            error!("Failed to parse inference graph from Value: {}", e);
            RouterError::GraphParseError(format!("JSON parsing error: {}", e))
        })?;
    
    // Validate the graph
    validate_graph(&graph_spec)?;
    
    Ok(graph_spec)
}

/// Validate an inference graph specification
///
/// # Arguments
///
/// * `graph` - The inference graph to validate
///
/// # Returns
///
/// * `Result<(), RouterError>` - Ok if valid, error otherwise
fn validate_graph(graph: &InferenceGraphSpec) -> Result<(), RouterError> {
    // Check if the root node exists
    if !graph.nodes.contains_key(crate::models::GRAPH_ROOT_NODE_NAME) {
        return Err(RouterError::GraphParseError(format!(
            "Root node '{}' not found in graph", 
            crate::models::GRAPH_ROOT_NODE_NAME
        )));
    }
    
    // Check for cycles in the graph
    // This would be implemented as a depth-first search algorithm to detect cycles
    // Simplified version for now
    for (node_name, node) in &graph.nodes {
        for step in &node.steps {
            if let Some(ref target_node) = step.node_name {
                // Check that referenced nodes exist
                if !graph.nodes.contains_key(target_node) {
                    return Err(RouterError::GraphParseError(format!(
                        "Node '{}' references non-existent node '{}'",
                        node_name, target_node
                    )));
                }
                
                // Ensure step has either nodeName or serviceUrl, not both
                if step.service_url.is_some() {
                    return Err(RouterError::GraphParseError(format!(
                        "Step '{}' in node '{}' has both nodeName and serviceUrl",
                        step.step_name, node_name
                    )));
                }
            } else if step.service_url.is_none() {
                // Ensure step has either nodeName or serviceUrl
                return Err(RouterError::GraphParseError(format!(
                    "Step '{}' in node '{}' has neither nodeName nor serviceUrl",
                    step.step_name, node_name
                )));
            }
            
            // Validate based on router type
            match node.router_type {
                crate::models::RouterType::Switch => {
                    // For switch, all steps except the last one must have a condition
                    if step.condition.is_none() && step != node.steps.last().unwrap() {
                        return Err(RouterError::GraphParseError(format!(
                            "Step '{}' in switch node '{}' has no condition",
                            step.step_name, node_name
                        )));
                    }
                },
                crate::models::RouterType::Splitter => {
                    // For splitter, all steps must have a weight
                    if step.weight.is_none() {
                        return Err(RouterError::GraphParseError(format!(
                            "Step '{}' in splitter node '{}' has no weight",
                            step.step_name, node_name
                        )));
                    }
                },
                _ => {}
            }
        }
    }
    
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;
    
    #[test]
    fn test_parse_valid_graph() {
        let json_str = r#"
        {
            "nodes": {
                "root": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "stepName": "step1",
                            "serviceUrl": "http://example.com/service1",
                            "dependency": "hard"
                        },
                        {
                            "stepName": "step2",
                            "serviceUrl": "http://example.com/service2",
                            "dependency": "soft"
                        }
                    ]
                }
            }
        }
        "#;
        
        let result = parse_graph_from_string(json_str);
        assert!(result.is_ok());
        
        let graph = result.unwrap();
        assert_eq!(graph.nodes.len(), 1);
        assert!(graph.nodes.contains_key("root"));
        
        let root_node = &graph.nodes["root"];
        assert_eq!(root_node.steps.len(), 2);
        assert_eq!(root_node.steps[0].step_name, "step1");
        assert_eq!(root_node.steps[1].step_name, "step2");
    }
    
    #[test]
    fn test_parse_missing_root() {
        let json_str = r#"
        {
            "nodes": {
                "not_root": {
                    "routerType": "sequence",
                    "steps": []
                }
            }
        }
        "#;
        
        let result = parse_graph_from_string(json_str);
        assert!(result.is_err());
        
        match result {
            Err(RouterError::GraphParseError(msg)) => {
                assert!(msg.contains("Root node"));
            },
            _ => panic!("Expected GraphParseError"),
        }
    }
    
    #[test]
    fn test_parse_invalid_json() {
        let json_str = r#"
        {
            "nodes": {
                "root": {
                    "routerType": "sequence",
                    "steps": [
        "#;
        
        let result = parse_graph_from_string(json_str);
        assert!(result.is_err());
        
        match result {
            Err(RouterError::GraphParseError(msg)) => {
                assert!(msg.contains("JSON parsing error"));
            },
            _ => panic!("Expected GraphParseError"),
        }
    }
    
    #[test]
    fn test_parse_from_value() {
        let value = json!({
            "nodes": {
                "root": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "stepName": "step1",
                            "serviceUrl": "http://example.com/service1",
                            "dependency": "hard"
                        }
                    ]
                }
            }
        });
        
        let result = parse_graph_from_value(value);
        assert!(result.is_ok());
        
        let graph = result.unwrap();
        assert_eq!(graph.nodes.len(), 1);
        assert!(graph.nodes.contains_key("root"));
    }
    
    #[test]
    fn test_validate_switch_router() {
        let value = json!({
            "nodes": {
                "root": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "stepName": "step1",
                            "serviceUrl": "http://example.com/service1",
                            "condition": "input.type == \"text\"",
                            "dependency": "hard"
                        },
                        {
                            "stepName": "step2",
                            "serviceUrl": "http://example.com/service2",
                            "dependency": "hard"
                        }
                    ]
                }
            }
        });
        
        let result = parse_graph_from_value(value);
        assert!(result.is_ok());
    }
    
    #[test]
    fn test_validate_invalid_switch_router() {
        let value = json!({
            "nodes": {
                "root": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "stepName": "step1",
                            "serviceUrl": "http://example.com/service1",
                            "dependency": "hard"
                        },
                        {
                            "stepName": "step2",
                            "serviceUrl": "http://example.com/service2",
                            "condition": "input.type == \"text\"",
                            "dependency": "hard"
                        }
                    ]
                }
            }
        });
        
        let result = parse_graph_from_value(value);
        assert!(result.is_err());
    }
} 