use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// RouterType defines the type of routing to be performed
#[derive(Debug, Serialize, Deserialize, Clone, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum RouterType {
    Splitter,
    Switch,
    Ensemble,
    Sequence,
}

/// DependencyType defines whether a step is a hard or soft dependency
#[derive(Debug, Serialize, Deserialize, Clone, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum DependencyType {
    Hard,
    Soft,
}

/// InferenceStep represents a single step in the inference graph
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct InferenceStep {
    #[serde(rename = "stepName")]
    pub step_name: String,
    #[serde(rename = "nodeName")]
    pub node_name: Option<String>,
    #[serde(rename = "serviceUrl")]
    pub service_url: Option<String>,
    pub weight: Option<i32>,
    pub condition: Option<String>,
    pub data: Option<String>,
    pub dependency: Option<DependencyType>,
}

/// InferenceNode represents a node in the inference graph
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct InferenceNode {
    #[serde(rename = "routerType")]
    pub router_type: RouterType,
    pub steps: Vec<InferenceStep>,
}

/// InferenceGraphSpec represents the complete inference graph specification
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct InferenceGraphSpec {
    pub nodes: HashMap<String, InferenceNode>,
}

// Constants
pub const GRAPH_ROOT_NODE_NAME: &str = "root"; 