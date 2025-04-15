use crate::test_setup::{init_test_logger, RequestCounter, start_mock_server, run_test_with_cleanup};
use serde_json::json;
use reqwest::Client;

#[tokio::test]
async fn test_mock_servers_from_examples() -> Result<(), Box<dyn std::error::Error>> {
    init_test_logger();
    
    run_test_with_cleanup(|| async {
        // Start a mock server for testing
        let counter = RequestCounter::new();
        let server_url = start_mock_server(
            "test-server",
            json!({
                "result": "Hello from mock server",
                "status": "success"
            }),
            counter.clone(),
        ).await?;
        
        // Send a test request to the mock server
        let client = Client::new();
        let test_body = json!({
            "input": "Test input data"
        });
        
        let response = client.post(&server_url)
            .json(&test_body)
            .send()
            .await?;
        
        // Verify the response
        assert!(response.status().is_success());
        let response_body: serde_json::Value = response.json().await?;
        assert_eq!(response_body["result"], "Hello from mock server");
        assert_eq!(response_body["status"], "success");
        
        // Verify the counter was incremented
        assert_eq!(counter.get_count(), 1);
        
        // Send another request and verify counter again
        client.post(&server_url)
            .json(&json!({ "input": "Another test" }))
            .send()
            .await?;
        assert_eq!(counter.get_count(), 2);
        
        Ok(())
    }).await
} 