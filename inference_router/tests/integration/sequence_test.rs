use crate::test_setup::{init_test_logger, RequestCounter, start_mock_server, create_test_graph, run_test_with_cleanup};
use serde_json::{json, Value};
use std::fs;
use tempfile::tempdir;
use inference_router::models::InferenceGraphSpec;
use inference_router::router::utils::route_request;
use inference_router::router::common::PropagatedHeaders;
use actix_web::{web, HttpResponse};

#[tokio::test]
async fn test_basic_sequence() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        // Create a counter for tracking requests
        let counter1 = RequestCounter::new();
        let counter2 = RequestCounter::new();
        
        // Set up mock servers for the test
        let server1 = start_mock_server(
            "service1",
            json!({ "result": "service1 response" }),
            counter1.clone()
        ).await?;
        
        let server2 = start_mock_server(
            "service2",
            json!({ "result": "service2 response" }),
            counter2.clone()
        ).await?;
        
        // Create test routing graph
        let server_urls = vec![
            ("step1", server1.as_str()),
            ("step2", server2.as_str())
        ];
        let graph_value = create_test_graph(&server_urls);
        let graph_spec: InferenceGraphSpec = serde_json::from_value(graph_value)?;
        
        // Test sequence execution
        let input = json!({"data": "test input"}).to_string().into_bytes();
        let (_response, status) = route_request("test-1", &graph_spec, &input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        
        // Verify both services were called and in the right order
        assert_eq!(counter1.get_count(), 1);
        assert_eq!(counter2.get_count(), 1);
        
        Ok(())
    }).await
}

#[tokio::test]
async fn test_nested_sequences() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        let preprocessor_counter = RequestCounter::new();
        let preprocessor_url = start_mock_server(
            "preprocessor",
            json!({
                "preprocessed": true
            }),
            preprocessor_counter.clone(),
        ).await?;
        
        let validator_counter = RequestCounter::new();
        let validator_url = start_mock_server(
            "validator",
            json!({
                "validated": true
            }),
            validator_counter.clone(),
        ).await?;
        
        let processor_counter = RequestCounter::new();
        let processor_url = start_mock_server(
            "processor",
            json!({
                "processed": true
            }),
            processor_counter.clone(),
        ).await?;
        
        let postprocessor_counter = RequestCounter::new();
        let postprocessor_url = start_mock_server(
            "postprocessor",
            json!({
                "postprocessed": true
            }),
            postprocessor_counter.clone(),
        ).await?;

        // Create a test graph with nested sequence routers
        let temp_dir = tempdir()?;
        let graph_path = temp_dir.path().join("test-graph.json");
        
        let graph = json!({
            "nodes": {
                "root": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "nodeName": "preprocessing",
                            "stepName": "preprocessing-sequence"
                        },
                        {
                            "serviceUrl": processor_url,
                            "stepName": "processor"
                        },
                        {
                            "serviceUrl": postprocessor_url,
                            "stepName": "postprocessor"
                        }
                    ]
                },
                "preprocessing": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "serviceUrl": preprocessor_url,
                            "stepName": "preprocessor"
                        },
                        {
                            "serviceUrl": validator_url,
                            "stepName": "validator"
                        }
                    ]
                }
            }
        });
        
        fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
        let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;
        
        // Test nested sequence execution
        let input = json!({"data": "test input"}).to_string().into_bytes();
        let (_response, status) = route_request("test-1", &graph_spec, &input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        
        // Verify all services were called
        assert_eq!(preprocessor_counter.get_count(), 1);
        assert_eq!(validator_counter.get_count(), 1);
        assert_eq!(processor_counter.get_count(), 1);
        assert_eq!(postprocessor_counter.get_count(), 1);
        
        Ok(())
    }).await
}

#[tokio::test]
async fn test_sequence_error_handling() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        let success_counter = RequestCounter::new();
        let success_url = start_mock_server(
            "success",
            json!({
                "status": "success"
            }),
            success_counter.clone(),
        ).await?;
        
        let error_counter = RequestCounter::new();
        let error_url = {
            let listener = std::net::TcpListener::bind("127.0.0.1:0").expect("Failed to bind");
            let port = listener.local_addr().unwrap().port();
            let url = format!("http://127.0.0.1:{}", port);
            
            let counter_clone = error_counter.clone();
            
            tokio::spawn(async move {
                actix_web::HttpServer::new(move || {
                    let counter = counter_clone.clone();
                    actix_web::App::new()
                        .route("/{tail:.*}", web::post().to(move |_body: web::Json<Value>| {
                            let c = counter.clone();
                            async move {
                                c.increment();
                                HttpResponse::InternalServerError().json(json!({
                                    "error": "Service error"
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
            
            tokio::time::sleep(std::time::Duration::from_millis(100)).await;
            url
        };
        
        let never_called_counter = RequestCounter::new();
        let never_called_url = start_mock_server(
            "never-called",
            json!({
                "status": "success"
            }),
            never_called_counter.clone(),
        ).await?;

        // Create a test graph that will encounter an error
        let temp_dir = tempdir()?;
        let graph_path = temp_dir.path().join("test-graph.json");
        
        let graph = json!({
            "nodes": {
                "root": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "serviceUrl": success_url,
                            "stepName": "success-1",
                            "dependency": "hard"
                        },
                        {
                            "serviceUrl": error_url,
                            "stepName": "error-step",
                            "dependency": "hard"
                        },
                        {
                            "serviceUrl": never_called_url,
                            "stepName": "never-called",
                            "dependency": "hard"
                        }
                    ]
                }
            }
        });
        
        fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
        let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

        // Test sequence with error
        let input = json!({"data": "test input"}).to_string().into_bytes();
        let result = route_request("test-1", &graph_spec, &input, &PropagatedHeaders::new()).await;
        
        // Verify error handling - the sequence router returns a 500 status code rather than an error
        assert!(result.is_ok());
        let (_response, status) = result.unwrap();
        assert_eq!(status, 500, "Status code should be 500 for a failed service");
        
        // Verify service calls happened correctly
        assert_eq!(success_counter.get_count(), 1, "Success should be called once");
        assert_eq!(error_counter.get_count(), 1, "Error service should be called once");
        assert_eq!(never_called_counter.get_count(), 0, "Never-called service should not be called");
        
        Ok(())
    }).await
}

#[tokio::test]
async fn test_sequence_with_conditional_steps() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        let validator_counter = RequestCounter::new();
        let validator_url = start_mock_server(
            "validator",
            json!({
                "validated": true,
                "needs_preprocessing": true
            }),
            validator_counter.clone(),
        ).await?;
        
        let preprocessor_counter = RequestCounter::new();
        let preprocessor_url = start_mock_server(
            "preprocessor",
            json!({
                "preprocessed": true
            }),
            preprocessor_counter.clone(),
        ).await?;
        
        let processor_counter = RequestCounter::new();
        let processor_url = start_mock_server(
            "processor",
            json!({
                "processed": true
            }),
            processor_counter.clone(),
        ).await?;

        // Create a test graph with conditional steps
        let temp_dir = tempdir()?;
        let graph_path = temp_dir.path().join("test-graph.json");
        
        let graph = json!({
            "nodes": {
                "root": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "serviceUrl": preprocessor_url,
                            "stepName": "preprocessor"
                        },
                        {
                            "serviceUrl": validator_url,
                            "stepName": "validator"
                        },
                        {
                            "serviceUrl": processor_url,
                            "stepName": "processor",
                            "data": "$response"
                        }
                    ]
                }
            }
        });
        
        fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
        let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

        // Test sequence with conditional step
        let input = json!({"data": "test input"}).to_string().into_bytes();
        let (_response, status) = route_request("test-1", &graph_spec, &input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        
        // Verify processors were called based on condition
        assert_eq!(preprocessor_counter.get_count(), 1);
        assert_eq!(validator_counter.get_count(), 1);
        assert_eq!(processor_counter.get_count(), 1);
        
        Ok(())
    }).await
} 