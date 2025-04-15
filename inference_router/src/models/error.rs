use serde::{Deserialize, Serialize};
use thiserror::Error;
use std::fmt;

/// RouterError represents errors that can occur in the inference router
#[derive(Error, Debug)]
pub enum RouterError {
    #[error("Failed to parse inference graph: {0}")]
    GraphParseError(String),

    #[error("Node not found: {0}")]
    NodeNotFoundError(String),

    #[error("Service error: {0}")]
    ServiceError(String),

    #[error("HTTP error: {0}")]
    HttpError(#[from] reqwest::Error),

    #[error("JSON error: {0}")]
    JsonError(#[from] serde_json::Error),

    #[error("IO error: {0}")]
    IoError(#[from] std::io::Error),

    #[error("Invalid condition: {0}")]
    InvalidConditionError(String),

    #[error("No matching condition found for switch router")]
    NoMatchingConditionError,

    #[error("Missing required target for step: {0}")]
    MissingTargetError(String),

    #[error("No valid steps in ensemble router")]
    NoValidEnsembleStepsError,

    #[error("Configuration error: {0}")]
    ConfigError(String),
}

/// InferenceGraphRoutingError is the error response returned to clients
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct InferenceGraphRoutingError {
    pub message: String,
    pub error: String,
}

impl fmt::Display for InferenceGraphRoutingError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.message, self.error)
    }
} 