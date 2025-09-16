// Runtime management module - similar to hf_xet/runtime.rs
use once_cell::sync::OnceCell;
use tokio::runtime::Runtime;

// Global Tokio runtime for async operations
static RUNTIME: OnceCell<Runtime> = OnceCell::new();

/// Get or create the global runtime
pub fn get_runtime() -> &'static Runtime {
    RUNTIME.get_or_init(|| {
        tokio::runtime::Builder::new_multi_thread()
            .enable_all()
            .worker_threads(4)
            .thread_name("xet-runtime")
            .build()
            .expect("Failed to create Tokio runtime")
    })
}

/// Block on an async operation using the global runtime
pub fn block_on<F: std::future::Future>(future: F) -> F::Output {
    get_runtime().block_on(future)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_runtime_initialization() {
        let runtime = get_runtime();
        assert!(runtime.handle().metrics().num_workers() > 0);
    }

    #[test]
    fn test_block_on() {
        let result = block_on(async {
            tokio::time::sleep(std::time::Duration::from_millis(10)).await;
            42
        });
        assert_eq!(result, 42);
    }
}
