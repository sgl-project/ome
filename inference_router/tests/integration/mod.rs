pub mod combined_test;
pub mod ensemble_test;
pub mod sequence_test;
pub mod splitter_test;
pub mod switch_test;
pub mod test_mock_servers;
pub mod test_setup;

// Re-export test_setup to avoid repeating dependencies in every test file
pub use crate::test_setup::*;

// Add test hooks for setup/cleanup
#[cfg(test)]
mod test_hooks {
    use super::test_setup::{cleanup_mock_servers_sync, init_test_logger};
    use std::sync::Once;
    
    static CLEANUP: Once = Once::new();
    static INIT: Once = Once::new();
    
    // Set up logging once
    #[ctor::ctor]
    fn init() {
        // Initialize logging
        INIT.call_once(|| {
            init_test_logger();
        });
        
        // Register panic handler for cleanup
        std::panic::set_hook(Box::new(|_| {
            // Clean up servers in case of test panic
            cleanup_mock_servers_sync();
        }));
    }
    
    // Ensure mock servers are cleaned up when tests finish
    #[ctor::dtor]
    fn cleanup() {
        CLEANUP.call_once(|| {
            println!("Test complete - cleaning up mock servers");
            cleanup_mock_servers_sync();
        });
    }
}

// Add more test modules as they are created 

// Add cleanup code to be called at the end of the test suite
#[cfg(test)]
mod tests {
    use crate::test_setup::cleanup_mock_servers;

    // Run after all tests to make sure all mock servers are cleaned up
    #[tokio::test]
    #[ignore]
    async fn cleanup_after_tests() {
        // This test is marked as ignored but will be run manually in the main function
        cleanup_mock_servers().await;
        println!("Test complete - mock servers will be cleaned up automatically.");
    }
} 