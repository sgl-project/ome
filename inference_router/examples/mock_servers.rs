use env_logger;
use serde_json::json;
use tokio;

mod testing {
    include!("../src/testing/mod.rs");
}

#[tokio::main]
async fn main() {
    // Configure logging
    env_logger::init_from_env(env_logger::Env::default().default_filter_or("info"));

    println!("Starting mock servers for integration testing...");

    // Use fixed ports for easier testing
    let classification_port = 8081;
    let text_port = 8082;
    let image_port = 8083;

    // Create servers with fixed ports and mock responses
    let classification_server = testing::MockServer::new("classification")
        .with_port(classification_port)
        .with_response(json!({
            "classification": "passed",
            "confidence": 0.95,
        }));

    let text_server = testing::MockServer::new("text-processor")
        .with_port(text_port)
        .with_response(json!({
            "processed": true,
            "type": "text",
            "result": "This is processed text",
        }));

    let image_server = testing::MockServer::new("image-processor")
        .with_port(image_port)
        .with_response(json!({
            "processed": true,
            "type": "image",
            "dimensions": {
                "width": 800,
                "height": 600
            }
        }));

    // Start all servers and get their handles and URLs
    let (_classification_counter, classification_handle, classification_url) = classification_server.start().await;
    let (_text_counter, text_handle, text_url) = text_server.start().await;
    let (_image_counter, image_handle, image_url) = image_server.start().await;

    // Spawn the server futures so they actually run
    tokio::spawn(classification_handle);
    tokio::spawn(text_handle);
    tokio::spawn(image_handle);

    println!("\nMock servers started:");
    println!("  - Classification server: {}", classification_url);
    println!("  - Text processing server: {}", text_url);
    println!("  - Image processing server: {}", image_url);

    // Save these URLs before moving them into the async block
    let class_url = classification_url.clone();
    let txt_url = text_url.clone();
    let img_url = image_url.clone();

    // Test connectivity to make sure the servers are up
    {
        // Clone these values specifically for the async block
        let test_class_url = class_url.clone();
        let test_txt_url = txt_url.clone();
        let test_img_url = img_url.clone();
        
        tokio::spawn(async move {
            tokio::time::sleep(tokio::time::Duration::from_secs(2)).await;
            
            println!("\nTesting server connectivity...");
            let client = reqwest::Client::new();
            
            for (name, url) in [
                ("Classification", &test_class_url),
                ("Text", &test_txt_url),
                ("Image", &test_img_url),
            ] {
                match client
                    .post(url)
                    .header("Content-Type", "application/json")
                    .body(r#"{"test":true}"#)
                    .send()
                    .await
                {
                    Ok(resp) => {
                        println!("  ✓ {} server is reachable - status {}", name, resp.status());
                        match resp.text().await {
                            Ok(body) => println!("    Response: {}", body),
                            Err(e) => println!("    Error reading response: {}", e),
                        }
                    }
                    Err(e) => {
                        println!("  ✗ {} server is not reachable: {}", name, e);
                    }
                }
            }
        });
    }

    // Create a sample inference graph using the mock servers
    let graph_json = create_sample_graph(&class_url, &txt_url, &img_url);

    println!("\nSample inference graph JSON for testing:");
    println!("{}", graph_json);
    println!("\nSave this to a file and use it with the inference router.");
    println!("Example: inference-router --graph-string '{}'", graph_json);

    println!("\nPress Ctrl+C to stop the servers");

    // Block until Ctrl+C is pressed
    tokio::signal::ctrl_c()
        .await
        .expect("Failed to listen for Ctrl+C");

    println!("Shutting down mock servers...");
}

/// Create a sample inference graph using the mock server URLs
fn create_sample_graph(classification_url: &str, text_url: &str, image_url: &str) -> String {
    let graph = json!({
        "nodes": {
            "root": {
                "routerType": "switch",
                "steps": [
                    {
                        "stepName": "process-text",
                        "nodeName": "text-processing",
                        "condition": "type.text"
                    },
                    {
                        "stepName": "process-image",
                        "nodeName": "image-processing",
                        "condition": "type.image"
                    }
                ]
            },
            "text-processing": {
                "routerType": "sequence",
                "steps": [
                    {
                        "stepName": "classify-content",
                        "serviceUrl": classification_url,
                        "dependency": "hard"
                    },
                    {
                        "stepName": "process-text",
                        "serviceUrl": text_url,
                        "data": "$response",
                        "dependency": "soft"
                    }
                ]
            },
            "image-processing": {
                "routerType": "sequence",
                "steps": [
                    {
                        "stepName": "classify-content",
                        "serviceUrl": classification_url,
                        "dependency": "hard"
                    },
                    {
                        "stepName": "process-image",
                        "serviceUrl": image_url,
                        "data": "$response",
                        "dependency": "soft"
                    }
                ]
            }
        }
    });

    serde_json::to_string_pretty(&graph).unwrap()
}