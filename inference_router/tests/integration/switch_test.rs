use crate::test_setup::{init_test_logger, RequestCounter, start_mock_server, run_test_with_cleanup};
use serde_json::json;
use std::fs;
use tempfile::tempdir;
use inference_router::models::InferenceGraphSpec;
use inference_router::router::utils::route_request;
use inference_router::router::common::PropagatedHeaders;

#[tokio::test]
async fn test_switch_basic_routing() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        // Start mock servers for different conditions
        let text_counter = RequestCounter::new();
        let text_url = start_mock_server(
            "text-model",
            json!({
                "type": "text",
                "result": "Text model processed the input"
            }),
            text_counter.clone(),
        ).await?;
        
        let image_counter = RequestCounter::new();
        let image_url = start_mock_server(
            "image-model",
            json!({
                "type": "image",
                "result": "Image model processed the input"
            }),
            image_counter.clone(),
        ).await?;
        
        let default_counter = RequestCounter::new();
        let default_url = start_mock_server(
            "default-model",
            json!({
                "type": "unknown",
                "result": "Default model processed the input"
            }),
            default_counter.clone(),
        ).await?;

        // Create a test graph with a switch router
        let temp_dir = tempdir()?;
        let graph_path = temp_dir.path().join("test-graph.json");
        
        let graph = json!({
            "nodes": {
                "root": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "condition": "type.text",
                            "serviceUrl": text_url,
                            "stepName": "text-processor"
                        },
                        {
                            "condition": "type.image",
                            "serviceUrl": image_url,
                            "stepName": "image-processor"
                        },
                        {
                            "condition": "type.unknown",
                            "serviceUrl": default_url,
                            "stepName": "unknown-processor"
                        },
                        {
                            "serviceUrl": default_url,
                            "stepName": "default"
                        }
                    ]
                }
            },
            "root_node": "root"
        });
        
        fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
        let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

        // Test text input
        let text_input = json!({
            "content": "Test text content",
            "type": {
                "text": true
            }
        }).to_string().into_bytes();
        let (_response, status) = route_request("test-1", &graph_spec, &text_input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        assert_eq!(text_counter.get_count(), 1);

        // Test image input
        let image_input = json!({
            "content": "Test image content",
            "type": {
                "image": true
            }
        }).to_string().into_bytes();
        let (_response, status) = route_request("test-2", &graph_spec, &image_input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        assert_eq!(image_counter.get_count(), 1);

        // Test unknown input
        let unknown_input = json!({
            "content": "Test unknown content",
            "type": {
                "unknown": true
            }
        }).to_string().into_bytes();
        let (_response, status) = route_request("test-3", &graph_spec, &unknown_input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        assert_eq!(default_counter.get_count(), 1);

        Ok(())
    }).await
}

#[tokio::test]
async fn test_switch_boolean_conditions() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        let is_premium_counter = RequestCounter::new();
        let premium_url = start_mock_server(
            "premium-model",
            json!({
                "result": "Premium model processed the input"
            }),
            is_premium_counter.clone(),
        ).await?;
        
        let standard_counter = RequestCounter::new();
        let standard_url = start_mock_server(
            "standard-model",
            json!({
                "result": "Standard model processed the input"
            }),
            standard_counter.clone(),
        ).await?;

        // Create a test graph with boolean conditions
        let temp_dir = tempdir()?;
        let graph_path = temp_dir.path().join("test-graph.json");
        
        let graph = json!({
            "nodes": {
                "root": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "condition": "flags.is_premium",
                            "serviceUrl": premium_url,
                            "stepName": "premium-service"
                        },
                        {
                            "condition": "user.standard",
                            "serviceUrl": standard_url,
                            "stepName": "standard-service"
                        },
                        {
                            "serviceUrl": standard_url,
                            "stepName": "default-service"
                        }
                    ]
                }
            },
            "root_node": "root"
        });
        
        fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
        let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

        // Test premium user
        let premium_input = json!({
            "flags": {"is_premium": true}
        }).to_string().into_bytes();
        let (_response, status) = route_request("test-1", &graph_spec, &premium_input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        assert_eq!(is_premium_counter.get_count(), 1);

        // Test standard user
        let standard_input = json!({
            "flags": {"is_premium": false}
        }).to_string().into_bytes();
        let (_response, status) = route_request("test-2", &graph_spec, &standard_input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        assert_eq!(standard_counter.get_count(), 1);

        Ok(())
    }).await
}

#[tokio::test]
async fn test_nested_switch_routers() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        let premium_text_counter = RequestCounter::new();
        let premium_text_url = start_mock_server(
            "premium-text-model",
            json!({
                "result": "Premium text model processed the input"
            }),
            premium_text_counter.clone(),
        ).await?;
        
        let premium_image_counter = RequestCounter::new();
        let premium_image_url = start_mock_server(
            "premium-image-model",
            json!({
                "result": "Premium image model processed the input"
            }),
            premium_image_counter.clone(),
        ).await?;

        let standard_text_counter = RequestCounter::new();
        let standard_text_url = start_mock_server(
            "standard-text-model",
            json!({
                "result": "Standard text model processed the input"
            }),
            standard_text_counter.clone(),
        ).await?;

        let standard_image_counter = RequestCounter::new();
        let standard_image_url = start_mock_server(
            "standard-image-model",
            json!({
                "result": "Standard image model processed the input"
            }),
            standard_image_counter.clone(),
        ).await?;

        // Create a nested switch router with tier and content-type routing
        let temp_dir = tempdir()?;
        let graph_path = temp_dir.path().join("test-graph.json");
        
        let graph = json!({
            "nodes": {
                "root": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "condition": "flags.is_premium",
                            "nodeName": "premium-node",
                            "stepName": "premium-route"
                        },
                        {
                            "condition": "user.standard",
                            "nodeName": "standard-node",
                            "stepName": "standard-route"
                        },
                        {
                            "nodeName": "standard-node",
                            "stepName": "default-route"
                        }
                    ]
                },
                "premium-node": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "serviceUrl": premium_text_url,
                            "stepName": "premium-text",
                            "condition": "type.text"
                        },
                        {
                            "serviceUrl": premium_image_url,
                            "stepName": "premium-image",
                            "condition": "type.image"
                        }
                    ]
                },
                "standard-node": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "serviceUrl": standard_text_url,
                            "stepName": "standard-text",
                            "condition": "type.text"
                        },
                        {
                            "serviceUrl": standard_image_url,
                            "stepName": "standard-image",
                            "condition": "type.image"
                        }
                    ]
                }
            },
            "root_node": "root"
        });
        
        fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
        let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

        // Test premium text
        let premium_text_input = json!({
            "flags": {"is_premium": true},
            "type": {
                "text": true
            }
        }).to_string().into_bytes();
        let (_response, status) = route_request("test-1", &graph_spec, &premium_text_input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        assert_eq!(premium_text_counter.get_count(), 1);

        // Test premium image
        let premium_image_input = json!({
            "flags": {"is_premium": true},
            "type": {
                "image": true
            }
        }).to_string().into_bytes();
        let (_response, status) = route_request("test-2", &graph_spec, &premium_image_input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        assert_eq!(premium_image_counter.get_count(), 1);

        // Test standard text
        let standard_text_input = json!({
            "flags": {"is_premium": false},
            "type": {
                "text": true
            }
        }).to_string().into_bytes();
        let (_response, status) = route_request("test-3", &graph_spec, &standard_text_input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        assert_eq!(standard_text_counter.get_count(), 1);

        // Test standard image
        let standard_image_input = json!({
            "flags": {"is_premium": false},
            "type": {
                "image": true
            }
        }).to_string().into_bytes();
        let (_response, status) = route_request("test-4", &graph_spec, &standard_image_input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        assert_eq!(standard_image_counter.get_count(), 1);

        Ok(())
    }).await
}

#[tokio::test]
async fn test_switch_error_handling() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        let error_counter = RequestCounter::new();
        let error_url = start_mock_server(
            "error-model",
            json!({
                "error": "Service error"
            }),
            error_counter.clone(),
        ).await?;

        // Create a test graph with invalid conditions
        let temp_dir = tempdir()?;
        let graph_path = temp_dir.path().join("test-graph.json");
        
        let graph = json!({
            "nodes": {
                "root": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "serviceUrl": error_url,
                            "stepName": "error-route",
                            "condition": "invalid.condition"
                        }
                    ]
                }
            },
            "root_node": "root"
        });
        
        fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
        let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

        // Test invalid condition
        let input = json!({"test": "data"}).to_string().into_bytes();
        let result = route_request("test-1", &graph_spec, &input, &PropagatedHeaders::new()).await;
        assert!(result.is_err());
        assert_eq!(error_counter.get_count(), 0);

        Ok(())
    }).await
} 