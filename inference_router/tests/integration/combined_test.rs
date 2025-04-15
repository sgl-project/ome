use crate::test_setup::{init_test_logger, RequestCounter, start_mock_server, run_test_with_cleanup};
use serde_json::{json, Value};
use std::fs;
use tempfile::tempdir;
use reqwest::Client;
use actix_web::{web, HttpResponse};
use inference_router::models::InferenceGraphSpec;
use inference_router::router::utils::route_request;
use inference_router::router::common::PropagatedHeaders;

/// This test creates a complex graph that combines all router types:
/// - Root node is a sequence that calls a preprocessing service and then a switch
/// - Switch routes to different processing pipelines based on input type
/// - Each pipeline has an ensemble of models running in parallel
/// - One pipeline uses a splitter for load balancing
#[tokio::test]
async fn test_combined_router_types() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        // Start mock servers for various purposes
        
        // Preprocessing service
        let preprocess_counter = RequestCounter::new();
        let preprocess_url = start_mock_server(
            "preprocessing",
            json!({
                "processed": true,
                "meta": {
                    "preprocessed_by": "mock-preprocessor"
                }
            }),
            preprocess_counter.clone(),
        ).await?;
        
        // Text processing service
        let text_counter = RequestCounter::new();
        let text_url = start_mock_server(
            "text-processor",
            json!({
                "type": "text",
                "result": "Text processor processed the input"
            }),
            text_counter.clone(),
        ).await?;
        
        // Image processing service
        let image_counter = RequestCounter::new();
        let image_url = start_mock_server(
            "image-processor",
            json!({
                "type": "image",
                "result": "Image processor processed the input"
            }),
            image_counter.clone(),
        ).await?;
        
        // Create a combined test graph
        let test_graph = json!({
            "nodes": {
                "root": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "stepName": "process-text",
                            "nodeName": "text-processing",
                            "condition": "type == \"text\"",
                            "dependency": "hard"
                        },
                        {
                            "stepName": "process-image",
                            "nodeName": "image-processing",
                            "condition": "type == \"image\"",
                            "dependency": "hard"
                        }
                    ]
                },
                "text-processing": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "stepName": "preprocess",
                            "serviceUrl": preprocess_url,
                            "dependency": "hard"
                        },
                        {
                            "stepName": "process-text",
                            "serviceUrl": text_url,
                            "data": "$response",
                            "dependency": "hard"
                        }
                    ]
                },
                "image-processing": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "stepName": "preprocess",
                            "serviceUrl": preprocess_url,
                            "dependency": "hard"
                        },
                        {
                            "stepName": "process-image",
                            "serviceUrl": image_url,
                            "data": "$response",
                            "dependency": "hard"
                        }
                    ]
                }
            }
        });
        
        // Parse the test graph
        let graph: InferenceGraphSpec = serde_json::from_value(test_graph)?;
        
        // Test text processing path
        let text_input = json!({
            "type": "text",
            "content": "Test text content"
        }).to_string().into_bytes();
        
        let headers = PropagatedHeaders::new();
        let (text_response, status) = route_request("test-text", &graph, &text_input, &headers)
            .await
            .map_err(|e| format!("Text processing failed: {}", e))?;
        
        assert_eq!(status, 200, "Text processing should return 200 status");
        let text_json: serde_json::Value = serde_json::from_slice(&text_response)?;
        assert_eq!(text_json["type"], "text", "Response should be from text processor");
        
        // Test image processing path
        let image_input = json!({
            "type": "image",
            "content": "Test image content"
        }).to_string().into_bytes();
        
        let (image_response, status) = route_request("test-image", &graph, &image_input, &headers)
            .await
            .map_err(|e| format!("Image processing failed: {}", e))?;
        
        assert_eq!(status, 200, "Image processing should return 200 status");
        let image_json: serde_json::Value = serde_json::from_slice(&image_response)?;
        assert_eq!(image_json["type"], "image", "Response should be from image processor");
        
        // Verify that all services were called the expected number of times
        assert_eq!(preprocess_counter.get_count(), 2, "Preprocess service should have been called twice");
        assert_eq!(text_counter.get_count(), 1, "Text processor should have been called once");
        assert_eq!(image_counter.get_count(), 1, "Image processor should have been called once");
        
        Ok(())
    }).await
}

#[tokio::test]
async fn test_combined_mock_servers() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        // Start mock servers for various purposes
        
        // Preprocessing service
        let preprocess_counter = RequestCounter::new();
        let preprocess_url = start_mock_server(
            "preprocessing",
            json!({
                "processed": true,
                "meta": {
                    "preprocessed_by": "mock-preprocessor"
                }
            }),
            preprocess_counter.clone(),
        ).await?;
        
        // Text processing service
        let text_counter = RequestCounter::new();
        let text_url = start_mock_server(
            "text-processor",
            json!({
                "type": "text",
                "result": "Text processor processed the input"
            }),
            text_counter.clone(),
        ).await?;
        
        // Image processing service
        let image_counter = RequestCounter::new();
        let image_url = start_mock_server(
            "image-processor",
            json!({
                "type": "image",
                "result": "Image processor processed the input"
            }),
            image_counter.clone(),
        ).await?;
        
        // Create a combined test graph
        let test_graph = json!({
            "nodes": {
                "root": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "stepName": "process-text",
                            "nodeName": "text-processing",
                            "condition": "type == \"text\"",
                            "dependency": "hard"
                        },
                        {
                            "stepName": "process-image",
                            "nodeName": "image-processing",
                            "condition": "type == \"image\"",
                            "dependency": "hard"
                        }
                    ]
                },
                "text-processing": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "stepName": "preprocess",
                            "serviceUrl": preprocess_url,
                            "dependency": "hard"
                        },
                        {
                            "stepName": "process-text",
                            "serviceUrl": text_url,
                            "data": "$response",
                            "dependency": "hard"
                        }
                    ]
                },
                "image-processing": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "stepName": "preprocess",
                            "serviceUrl": preprocess_url,
                            "dependency": "hard"
                        },
                        {
                            "stepName": "process-image",
                            "serviceUrl": image_url,
                            "data": "$response",
                            "dependency": "hard"
                        }
                    ]
                }
            }
        });
        
        // Test the mock servers directly
        let client = Client::new();
        
        // Test preprocessing service
        let preprocess_response = client.post(&preprocess_url)
            .json(&json!({"test": "data"}))
            .send()
            .await?;
        
        assert_eq!(preprocess_response.status(), 200);
        let preprocess_json: serde_json::Value = preprocess_response.json().await?;
        assert_eq!(preprocess_json["processed"], true);
        
        // Test text processor
        let text_response = client.post(&text_url)
            .json(&json!({"test": "data"}))
            .send()
            .await?;
        
        assert_eq!(text_response.status(), 200);
        let text_json: serde_json::Value = text_response.json().await?;
        assert_eq!(text_json["type"], "text");
        
        // Test image processor
        let image_response = client.post(&image_url)
            .json(&json!({"test": "data"}))
            .send()
            .await?;
        
        assert_eq!(image_response.status(), 200);
        let image_json: serde_json::Value = image_response.json().await?;
        assert_eq!(image_json["type"], "image");
        
        // Verify that all services were called
        assert_eq!(preprocess_counter.get_count(), 1, "Preprocess service should have been called once");
        assert_eq!(text_counter.get_count(), 1, "Text processor should have been called once");
        assert_eq!(image_counter.get_count(), 1, "Image processor should have been called once");
        
        // Store the graph to a temporary file for future testing
        let temp_dir = tempdir()?;
        let graph_path = temp_dir.path().join("combined-test-graph.json");
        fs::write(&graph_path, test_graph.to_string())?;
        
        println!("Created combined test graph at: {}", graph_path.display());
        
        Ok(())
    }).await
}

#[tokio::test]
async fn test_complex_routing_pipeline() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        // Start mock servers for the test
        
        let validator_counter = RequestCounter::new();
        let validator_url = start_mock_server(
            "validator",
            json!({
                "validated": true,
                "type": "$input.type"
            }),
            validator_counter.clone(),
        ).await?;
        
        let text_model1_counter = RequestCounter::new();
        let text_model1_url = start_mock_server(
            "text-model1",
            json!({
                "model": "text1",
                "result": "Text model 1 processed the input"
            }),
            text_model1_counter.clone(),
        ).await?;
        
        let text_model2_counter = RequestCounter::new();
        let text_model2_url = start_mock_server(
            "text-model2",
            json!({
                "model": "text2",
                "result": "Text model 2 processed the input"
            }),
            text_model2_counter.clone(),
        ).await?;
        
        let image_model1_counter = RequestCounter::new();
        let image_model1_url = start_mock_server(
            "image-model1",
            json!({
                "model": "image1",
                "result": "Image model 1 processed the input"
            }),
            image_model1_counter.clone(),
        ).await?;
        
        let image_model2_counter = RequestCounter::new();
        let image_model2_url = start_mock_server(
            "image-model2",
            json!({
                "model": "image2",
                "result": "Image model 2 processed the input"
            }),
            image_model2_counter.clone(),
        ).await?;
        
        let postprocess_counter = RequestCounter::new();
        let postprocess_url = start_mock_server(
            "postprocessor",
            json!({
                "postprocessed": true,
                "result": "processed"
            }),
            postprocess_counter.clone(),
        ).await?;

        // Create a complex test graph that combines all router types
        let temp_dir = tempdir()?;
        let graph_path = temp_dir.path().join("test-graph.json");
        
        let graph = json!({
            "nodes": {
                "root": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "serviceUrl": validator_url,
                            "stepName": "validator"
                        },
                        {
                            "nodeName": "type-router",
                            "stepName": "route-by-type",
                            "data": "$response"
                        },
                        {
                            "serviceUrl": postprocess_url,
                            "stepName": "postprocessor",
                            "data": "$response"
                        }
                    ]
                },
                "type-router": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "condition": "type == \"text\"",
                            "nodeName": "text-ensemble",
                            "stepName": "text-processing"
                        },
                        {
                            "condition": "type == \"image\"",
                            "nodeName": "image-splitter",
                            "stepName": "image-processing"
                        }
                    ]
                },
                "text-ensemble": {
                    "routerType": "ensemble",
                    "steps": [
                        {
                            "serviceUrl": text_model1_url,
                            "stepName": "text-model1",
                            "dependency": "soft"
                        },
                        {
                            "serviceUrl": text_model2_url,
                            "stepName": "text-model2",
                            "dependency": "soft"
                        }
                    ]
                },
                "image-splitter": {
                    "routerType": "splitter",
                    "steps": [
                        {
                            "serviceUrl": image_model1_url,
                            "stepName": "image-model1",
                            "weight": 70,
                            "dependency": "hard"
                        },
                        {
                            "serviceUrl": image_model2_url,
                            "stepName": "image-model2",
                            "weight": 30,
                            "dependency": "hard"
                        }
                    ]
                }
            },
            "root_node": "root"
        });
        
        fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
        let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

        // Test text processing path
        let text_input = json!({
            "type": "text",
            "content": "test text"
        }).to_string().into_bytes();
        
        let (_text_response, status) = route_request("test-1", &graph_spec, &text_input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        
        let text_json: serde_json::Value = serde_json::from_slice(&_text_response)?;
        assert!(text_json["postprocessed"].as_bool().unwrap());
        assert_eq!(text_json["result"], "processed");

        // Test image processing path with multiple requests to verify distribution
        let num_requests = 100;
        for i in 0..num_requests {
            let image_input = json!({
                "type": "image",
                "content": "test image"
            }).to_string().into_bytes();
            
            let (_image_response, status) = route_request(&format!("test-image-{}", i), &graph_spec, &image_input, &PropagatedHeaders::new()).await?;
            assert_eq!(status, 200);
        }

        // Verify model call counts for distribution
        // Due to the random nature, we can't assert exact counts, but image1 and image2 should both be called
        assert!(image_model1_counter.get_count() > 0, "image1 model should have been called");
        assert!(image_model2_counter.get_count() > 0, "image2 model should have been called");
        assert_eq!(validator_counter.get_count(), num_requests + 1); // +1 for the text request
        assert_eq!(postprocess_counter.get_count(), num_requests + 1); // +1 for the text request
        
        // Text models should each be called once for the text request
        assert_eq!(text_model1_counter.get_count(), 1);
        assert_eq!(text_model2_counter.get_count(), 1);
        
        Ok(())
    }).await
}

#[tokio::test]
async fn test_error_handling_in_complex_pipeline() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        let validator_counter = RequestCounter::new();
        let validator_url = start_mock_server(
            "validator",
            json!({
                "validated": true,
                "type": "$input.type"
            }),
            validator_counter.clone(),
        ).await?;
        
        let success_counter = RequestCounter::new();
        let success_url = start_mock_server(
            "success-service",
            json!({
                "status": "success",
                "result": "Success service response"
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
        
        let postprocess_counter = RequestCounter::new();
        let postprocess_url = start_mock_server(
            "postprocessor",
            json!({
                "postprocessed": true,
                "result": "processed"
            }),
            postprocess_counter.clone(),
        ).await?;

        // Create a test graph with mixed dependencies
        let temp_dir = tempdir()?;
        let graph_path = temp_dir.path().join("test-graph.json");
        
        let graph = json!({
            "nodes": {
                "root": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "serviceUrl": validator_url,
                            "stepName": "validator"
                        },
                        {
                            "nodeName": "processing",
                            "stepName": "processing",
                            "data": "$response"
                        },
                        {
                            "serviceUrl": postprocess_url,
                            "stepName": "postprocessor",
                            "data": "$response"
                        }
                    ]
                },
                "processing": {
                    "routerType": "ensemble",
                    "steps": [
                        {
                            "serviceUrl": success_url,
                            "stepName": "success-service",
                            "dependency": "hard"
                        },
                        {
                            "serviceUrl": error_url,
                            "stepName": "error-service",
                            "dependency": "soft"
                        }
                    ]
                }
            },
            "root_node": "root"
        });
        
        fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
        let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

        // Test with input that includes error-service but should still complete successfully
        let input = json!({
            "type": "test",
            "content": "test input"
        }).to_string().into_bytes();
        
        let result = route_request("test-1", &graph_spec, &input, &PropagatedHeaders::new()).await;
        assert!(result.is_ok(), "Test should succeed since error-service is a soft dependency");
        let (_, status) = result.unwrap();
        assert_eq!(status, 200);

        // Verify service call counts
        assert_eq!(validator_counter.get_count(), 1);
        assert_eq!(success_counter.get_count(), 1);
        assert_eq!(error_counter.get_count(), 1);
        assert_eq!(postprocess_counter.get_count(), 1); // Should reach postprocessor now

        Ok(())
    }).await
}

#[tokio::test]
async fn test_conditional_routing_with_data_transformation() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        let classifier_counter = RequestCounter::new();
        let classifier_url = start_mock_server(
            "classifier",
            json!({
                "type": "text",
                "sentiment": "positive",
                "confidence": 0.9
            }),
            classifier_counter.clone(),
        ).await?;
        
        let positive_counter = RequestCounter::new();
        let positive_url = start_mock_server(
            "positive-handler",
            json!({
                "result": "Handled positive sentiment"
            }),
            positive_counter.clone(),
        ).await?;
        
        let negative_counter = RequestCounter::new();
        let negative_url = start_mock_server(
            "negative-handler",
            json!({
                "result": "Handled negative sentiment"
            }),
            negative_counter.clone(),
        ).await?;
        
        let low_confidence_counter = RequestCounter::new();
        let low_confidence_url = start_mock_server(
            "low-confidence-handler",
            json!({
                "result": "Handled low confidence case"
            }),
            low_confidence_counter.clone(),
        ).await?;

        // Create a test graph with conditional routing based on transformed data
        let temp_dir = tempdir()?;
        let graph_path = temp_dir.path().join("test-graph.json");
        
        let graph = json!({
            "nodes": {
                "root": {
                    "routerType": "sequence",
                    "steps": [
                        {
                            "serviceUrl": classifier_url,
                            "stepName": "classifier"
                        },
                        {
                            "nodeName": "sentiment-router",
                            "stepName": "route-by-sentiment",
                            "data": "$response"
                        }
                    ]
                },
                "sentiment-router": {
                    "routerType": "switch",
                    "steps": [
                        {
                            "condition": "confidence < 0.7",
                            "serviceUrl": low_confidence_url,
                            "stepName": "low-confidence"
                        },
                        {
                            "condition": "sentiment == \"positive\"",
                            "serviceUrl": positive_url,
                            "stepName": "positive"
                        },
                        {
                            "condition": "sentiment == \"negative\"",
                            "serviceUrl": negative_url,
                            "stepName": "negative"
                        },
                        {
                            "serviceUrl": positive_url,
                            "stepName": "default"
                        }
                    ]
                }
            },
            "root_node": "root"
        });
        
        fs::write(&graph_path, serde_json::to_string_pretty(&graph)?)?;
        let graph_spec: InferenceGraphSpec = serde_json::from_str(&fs::read_to_string(&graph_path)?)?;

        // Test with different inputs
        let input = json!({
            "text": "test input"
        }).to_string().into_bytes();
        
        let (_response, status) = route_request("test-1", &graph_spec, &input, &PropagatedHeaders::new()).await?;
        assert_eq!(status, 200);
        
        // Verify that the classifier was called and the response was routed correctly
        assert_eq!(classifier_counter.get_count(), 1);
        assert_eq!(positive_counter.get_count(), 1);
        assert_eq!(negative_counter.get_count(), 0);
        assert_eq!(low_confidence_counter.get_count(), 0);

        Ok(())
    }).await
}