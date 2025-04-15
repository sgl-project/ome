use crate::test_setup::{init_test_logger, RequestCounter, start_mock_server, cleanup_mock_servers};
use serde_json::{json, Value};
use std::fs;
use tempfile::tempdir;
use reqwest::Client;
use actix_web::{web, HttpResponse};

// Import from the lib.rs re-export
use inference_router::models::{
    parse_graph_from_string,
    InferenceGraphSpec

};
use inference_router::router::utils::route_request;
use inference_router::router::common::PropagatedHeaders;
use uuid::Uuid;

#[tokio::test]
async fn test_ensemble_router_directly() -> Result<(), Box<dyn std::error::Error>> {
    // Skip this test for now until we can properly fix the router_request return type

    init_test_logger();
    
    // Start mock servers for ensemble
    let service1_counter = RequestCounter::new();
    let service1_url = start_mock_server(
        "service1",
        json!({
            "service": "service1",
            "result": "Service 1 processed the input"
        }),
        service1_counter.clone(),
    ).await?;
    
    let service2_counter = RequestCounter::new();
    let service2_url = start_mock_server(
        "service2",
        json!({
            "service": "service2",
            "result": "Service 2 processed the input"
        }),
        service2_counter.clone(),
    ).await?;
    
    let service3_counter = RequestCounter::new();
    let service3_url = start_mock_server(
        "service3",
        json!({
            "service": "service3",
            "result": "Service 3 processed the input"
        }),
        service3_counter.clone(),
    ).await?;
    
    // Create test graph for ensemble router
    let test_graph = json!({
        "nodes": {
            "root": {
                "routerType": "ensemble",
                "steps": [
                    {
                        "stepName": "service1",
                        "serviceUrl": service1_url,
                        "dependency": "soft"
                    },
                    {
                        "stepName": "service2",
                        "serviceUrl": service2_url,
                        "dependency": "soft"
                    },
                    {
                        "stepName": "service3",
                        "serviceUrl": service3_url,
                        "dependency": "soft"
                    }
                ]
            }
        }
    });
    
    // Parse the graph
    let graph_spec: InferenceGraphSpec = parse_graph_from_string(&test_graph.to_string())?;
    
    // Create test input
    let input = json!({
        "query": "test input for ensemble router"
    });
    let input_bytes = serde_json::to_vec(&input)?;
    
    // Create request ID and headers
    let request_id = Uuid::new_v4().to_string();
    let headers: PropagatedHeaders = vec![];
    
    // Call the router directly
    let result = route_request(&request_id, &graph_spec, &input_bytes, &headers).await?;
    let (output, status) = result;
    
    // Verify status code
    assert_eq!(status, 200, "Expected status code 200");
    
    // Parse and verify the response
    let response: Value = serde_json::from_slice(&output)?;
    
    // Ensemble should include results from all services
    assert!(response.get("service1").is_some(), "Response should include service1 result");
    assert!(response.get("service2").is_some(), "Response should include service2 result");
    assert!(response.get("service3").is_some(), "Response should include service3 result");
    
    // Verify that all services were called
    assert!(service1_counter.get_count() > 0, "Service 1 should have been called");
    assert!(service2_counter.get_count() > 0, "Service 2 should have been called");
    assert!(service3_counter.get_count() > 0, "Service 3 should have been called");
    
    // Cleanup mock servers
    cleanup_mock_servers().await;
    
    Ok(())
}

#[tokio::test]
async fn test_ensemble_with_failing_service() -> Result<(), Box<dyn std::error::Error>> {
    // Skip this test for now until we can properly fix the router_request return type

    init_test_logger();
    
    // Start mock servers - one that succeeds and one that fails
    let success_counter = RequestCounter::new();
    let success_url = start_mock_server(
        "success-service",
        json!({
            "service": "success",
            "result": "Success service response"
        }),
        success_counter.clone(),
    ).await?;
    
    // Create a counter for the failing service
    let fail_counter = RequestCounter::new();
    
    // Start a mock server that returns an error (status 500)
    let fail_url = {
        // Find a free port
        let listener = std::net::TcpListener::bind("127.0.0.1:0").expect("Failed to bind");
        let port = listener.local_addr().unwrap().port();
        let url = format!("http://127.0.0.1:{}", port);
        
        let counter_clone = fail_counter.clone();
        
        // Start the server
        tokio::spawn(async move {
            actix_web::HttpServer::new(move || {
                let counter = counter_clone.clone();
                actix_web::App::new()
                    .route("/{tail:.*}", web::post().to(move |_body: web::Json<Value>| {
                        let c = counter.clone();
                        async move {
                            c.increment();
                            // Return error status
                            HttpResponse::InternalServerError().json(json!({
                                "error": "Simulated server error"
                            }))
                        }
                    }))
            })
            .listen(listener)
            .unwrap()
            .run()
            .await
            .unwrap();
        });
        
        // Wait for server to start
        tokio::time::sleep(std::time::Duration::from_millis(100)).await;
        url
    };
    
    // Create test graph with a soft dependency for the failing service
    let test_graph = json!({
        "nodes": {
            "root": {
                "routerType": "ensemble",
                "steps": [
                    {
                        "stepName": "success-service",
                        "serviceUrl": success_url,
                        "dependency": "soft"
                    },
                    {
                        "stepName": "fail-service",
                        "serviceUrl": fail_url,
                        "dependency": "soft" // Soft dependency should allow the request to continue
                    }
                ]
            }
        }
    });
    
    // Parse the graph
    let graph_spec: InferenceGraphSpec = parse_graph_from_string(&test_graph.to_string())?;
    
    // Create test input
    let input = json!({
        "query": "test input for ensemble with failures"
    });
    let input_bytes = serde_json::to_vec(&input)?;
    
    // Create request ID and headers
    let request_id = Uuid::new_v4().to_string();
    let headers: PropagatedHeaders = vec![];
    
    // Call the router directly
    let result = route_request(&request_id, &graph_spec, &input_bytes, &headers).await?;
    let (output, status) = result;
    
    // Verify status code - should still be 200 since fail-service is a soft dependency
    assert_eq!(status, 200, "Expected status code 200 with soft dependency failure");
    
    // Parse and verify the response
    let response: Value = serde_json::from_slice(&output)?;
    
    // Response should include the successful service result
    assert!(response.get("success-service").is_some(), "Response should include success-service result");
    
    // For the failed service, it should include an error message
    let fail_result = response.get("fail-service").unwrap();
    assert!(fail_result.get("error").is_some(), "Response should include error for fail-service");
    
    // Verify that both services were called
    assert_eq!(success_counter.get_count(), 1, "Success service should have been called once");
    assert_eq!(fail_counter.get_count(), 1, "Fail service should have been called once");
    
    // Cleanup mock servers
    cleanup_mock_servers().await;
    
    Ok(())
}

#[tokio::test]
async fn test_ensemble_mock_servers() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    // Start mock servers for ensemble
    let service1_counter = RequestCounter::new();
    let service1_url = start_mock_server(
        "service1",
        json!({
            "service": "service1",
            "result": "Service 1 processed the input"
        }),
        service1_counter.clone(),
    ).await?;
    
    let service2_counter = RequestCounter::new();
    let service2_url = start_mock_server(
        "service2",
        json!({
            "service": "service2",
            "result": "Service 2 processed the input"
        }),
        service2_counter.clone(),
    ).await?;
    
    let service3_counter = RequestCounter::new();
    let service3_url = start_mock_server(
        "service3",
        json!({
            "service": "service3",
            "result": "Service 3 processed the input"
        }),
        service3_counter.clone(),
    ).await?;
    
    // Create test graph for ensemble router - just for testing purposes
    let test_graph = json!({
        "nodes": {
            "root": {
                "routerType": "ensemble",
                "steps": [
                    {
                        "stepName": "service1",
                        "serviceUrl": service1_url,
                        "dependency": "soft"
                    },
                    {
                        "stepName": "service2",
                        "serviceUrl": service2_url,
                        "dependency": "soft"
                    },
                    {
                        "stepName": "service3",
                        "serviceUrl": service3_url,
                        "dependency": "soft"
                    }
                ]
            }
        }
    });
    
    // Test the mock servers directly
    let client = Client::new();
    
    // Test service 1
    let service1_response = client.post(&service1_url)
        .json(&json!({"test": "data"}))
        .send()
        .await?;
    
    assert_eq!(service1_response.status(), 200);
    let service1_json: serde_json::Value = service1_response.json().await?;
    assert_eq!(service1_json["service"], "service1");
    
    // Test service 2
    let service2_response = client.post(&service2_url)
        .json(&json!({"test": "data"}))
        .send()
        .await?;
    
    assert_eq!(service2_response.status(), 200);
    let service2_json: serde_json::Value = service2_response.json().await?;
    assert_eq!(service2_json["service"], "service2");
    
    // Test service 3
    let service3_response = client.post(&service3_url)
        .json(&json!({"test": "data"}))
        .send()
        .await?;
    
    assert_eq!(service3_response.status(), 200);
    let service3_json: serde_json::Value = service3_response.json().await?;
    assert_eq!(service3_json["service"], "service3");
    
    // Verify that all services were called
    assert_eq!(service1_counter.get_count(), 1, "Service 1 should have been called once");
    assert_eq!(service2_counter.get_count(), 1, "Service 2 should have been called once");
    assert_eq!(service3_counter.get_count(), 1, "Service 3 should have been called once");
    
    // Store the graph to a temporary file for future testing
    let temp_dir = tempdir()?;
    let graph_path = temp_dir.path().join("ensemble-test-graph.json");
    std::fs::write(&graph_path, test_graph.to_string())?;
    
    println!("Created ensemble test graph at: {}", graph_path.display());
    
    // Cleanup mock servers
    cleanup_mock_servers().await;
    
    Ok(())
}

#[tokio::test]
async fn test_ensemble_parallel_execution() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    let service1_counter = RequestCounter::new();
    let service1_url = start_mock_server(
        "service1",
        json!({
            "result": "service1 result"
        }),
        service1_counter.clone(),
    ).await?;
    
    let service2_counter = RequestCounter::new();
    let service2_url = start_mock_server(
        "service2",
        json!({
            "result": "service2 result"
        }),
        service2_counter.clone(),
    ).await?;
    
    let service3_counter = RequestCounter::new();
    let service3_url = start_mock_server(
        "service3",
        json!({
            "result": "service3 result"
        }),
        service3_counter.clone(),
    ).await?;

    // Create a test graph with ensemble router
    let temp_dir = tempdir()?;
    let graph_path = temp_dir.path().join("ensemble-test-graph.json");
    
    let graph = json!({
        "nodes": {
            "root": {
                "routerType": "ensemble",
                "steps": [
                    {
                        "serviceUrl": service1_url,
                        "stepName": "service1"
                    },
                    {
                        "serviceUrl": service2_url,
                        "stepName": "service2"
                    },
                    {
                        "serviceUrl": service3_url,
                        "stepName": "service3"
                    }
                ]
            }
        }
    });
    
    fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
    let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

    // Test ensemble execution
    let input = json!({
        "query": "test input for ensemble router"
    }).to_string().into_bytes();
    let (_response, status) = route_request("test-1", &graph_spec, &input, &PropagatedHeaders::new()).await?;
    assert_eq!(status, 200);
    
    // Verify all services were called
    assert_eq!(service1_counter.get_count(), 1);
    assert_eq!(service2_counter.get_count(), 1);
    assert_eq!(service3_counter.get_count(), 1);

    // Cleanup mock servers
    cleanup_mock_servers().await;
    
    Ok(())
}

#[tokio::test]
async fn test_ensemble_with_hard_dependencies() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    let hard_success_counter = RequestCounter::new();
    let hard_success_url = start_mock_server(
        "hard-success",
        json!({
            "result": "success"
        }),
        hard_success_counter.clone(),
    ).await?;
    
    let service1_counter = RequestCounter::new();
    let service1_url = start_mock_server(
        "service1",
        json!({
            "result": "service1 result"
        }),
        service1_counter.clone(),
    ).await?;

    // Create a test graph with hard dependencies
    let temp_dir = tempdir()?;
    let graph_path = temp_dir.path().join("ensemble-test-graph.json");
    
    let graph = json!({
        "nodes": {
            "root": {
                "routerType": "ensemble",
                "steps": [
                    {
                        "serviceUrl": hard_success_url,
                        "stepName": "hard-success",
                        "dependency": "hard"
                    },
                    {
                        "serviceUrl": service1_url,
                        "stepName": "service1",
                        "dependency": "hard"
                    }
                ]
            }
        }
    });
    
    fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
    let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

    // Test ensemble with hard dependencies
    let input = json!({
        "query": "test input for ensemble router"
    }).to_string().into_bytes();
    let (_response, status) = route_request("test-1", &graph_spec, &input, &PropagatedHeaders::new()).await?;
    assert_eq!(status, 200);
    
    // Verify service calls
    assert_eq!(hard_success_counter.get_count(), 1);
    assert_eq!(service1_counter.get_count(), 1);

    // Cleanup mock servers
    cleanup_mock_servers().await;
    
    Ok(())
}

#[tokio::test]
async fn test_ensemble_with_mixed_dependencies() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    let hard_success_counter = RequestCounter::new();
    let hard_success_url = start_mock_server(
        "hard-success",
        json!({
            "result": "success"
        }),
        hard_success_counter.clone(),
    ).await?;
    
    let soft_success_counter = RequestCounter::new();
    let soft_success_url = start_mock_server(
        "soft-success",
        json!({
            "result": "success"
        }),
        soft_success_counter.clone(),
    ).await?;
    
    let error_counter = RequestCounter::new();
    let error_url = start_mock_server(
        "error",
        json!({
            "error": "Service error"
        }),
        error_counter.clone(),
    ).await?;

    // Create a test graph with mixed dependencies
    let temp_dir = tempdir()?;
    let graph_path = temp_dir.path().join("ensemble-test-graph.json");
    
    let graph = json!({
        "nodes": {
            "root": {
                "routerType": "ensemble",
                "steps": [
                    {
                        "serviceUrl": hard_success_url,
                        "stepName": "hard-success",
                        "dependency": "hard"
                    },
                    {
                        "serviceUrl": soft_success_url,
                        "stepName": "soft-success",
                        "dependency": "soft"
                    },
                    {
                        "serviceUrl": error_url,
                        "stepName": "error-service",
                        "dependency": "soft"
                    }
                ]
            }
        }
    });
    
    fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
    let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

    // Test ensemble with mixed dependencies
    let input = json!({
        "query": "test input for ensemble router"
    }).to_string().into_bytes();
    let (_response, status) = route_request("test-1", &graph_spec, &input, &PropagatedHeaders::new()).await?;
    assert_eq!(status, 200);
    
    // Verify service calls
    assert_eq!(hard_success_counter.get_count(), 1);
    assert_eq!(soft_success_counter.get_count(), 1);
    assert_eq!(error_counter.get_count(), 1);

    // Cleanup mock servers
    cleanup_mock_servers().await;
    
    Ok(())
}

#[tokio::test]
async fn test_nested_ensemble_routers() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    let text_model1_counter = RequestCounter::new();
    let text_model1_url = start_mock_server(
        "text-model1",
        json!({
            "result": "text model 1 result"
        }),
        text_model1_counter.clone(),
    ).await?;
    
    let text_model2_counter = RequestCounter::new();
    let text_model2_url = start_mock_server(
        "text-model2",
        json!({
            "result": "text model 2 result"
        }),
        text_model2_counter.clone(),
    ).await?;
    
    let image_model1_counter = RequestCounter::new();
    let image_model1_url = start_mock_server(
        "image-model1",
        json!({
            "result": "image model 1 result"
        }),
        image_model1_counter.clone(),
    ).await?;
    
    let image_model2_counter = RequestCounter::new();
    let image_model2_url = start_mock_server(
        "image-model2",
        json!({
            "result": "image model 2 result"
        }),
        image_model2_counter.clone(),
    ).await?;
    
    let postprocessor_counter = RequestCounter::new();
    let postprocessor_url = start_mock_server(
        "postprocessor",
        json!({
            "result": "postprocessed result"
        }),
        postprocessor_counter.clone(),
    ).await?;

    // Create a test graph with nested ensembles
    let temp_dir = tempdir()?;
    let graph_path = temp_dir.path().join("ensemble-test-graph.json");
    
    let graph = json!({
        "nodes": {
            "root": {
                "routerType": "sequence",
                "steps": [
                    {
                        "nodeName": "model-ensemble",
                        "stepName": "model-step"
                    },
                    {
                        "serviceUrl": postprocessor_url,
                        "stepName": "postprocess"
                    }
                ]
            },
            "model-ensemble": {
                "routerType": "ensemble",
                "steps": [
                    {
                        "nodeName": "text-models",
                        "stepName": "text-ensemble"
                    },
                    {
                        "nodeName": "image-models",
                        "stepName": "image-ensemble"
                    }
                ]
            },
            "text-models": {
                "routerType": "ensemble",
                "steps": [
                    {
                        "serviceUrl": text_model1_url,
                        "stepName": "text-model1"
                    },
                    {
                        "serviceUrl": text_model2_url,
                        "stepName": "text-model2"
                    }
                ]
            },
            "image-models": {
                "routerType": "ensemble",
                "steps": [
                    {
                        "serviceUrl": image_model1_url,
                        "stepName": "image-model1"
                    },
                    {
                        "serviceUrl": image_model2_url,
                        "stepName": "image-model2"
                    }
                ]
            }
        }
    });
    
    fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
    let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

    // Test nested ensembles
    let input = json!({
        "query": "test input for nested ensembles"
    }).to_string().into_bytes();
    let (_response, status) = route_request("test-1", &graph_spec, &input, &PropagatedHeaders::new()).await?;
    assert_eq!(status, 200);
    
    // Verify service calls
    assert_eq!(text_model1_counter.get_count(), 1);
    assert_eq!(text_model2_counter.get_count(), 1);
    assert_eq!(image_model1_counter.get_count(), 1);
    assert_eq!(image_model2_counter.get_count(), 1);
    assert_eq!(postprocessor_counter.get_count(), 1);

    // Cleanup mock servers
    cleanup_mock_servers().await;
    
    Ok(())
} 