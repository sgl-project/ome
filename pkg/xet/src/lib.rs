mod ffi;
mod error;
mod hf_adapter;
mod xet_integration;
mod xet_downloader;

pub use ffi::*;
pub use error::*;

use once_cell::sync::OnceCell;
use tokio::runtime::Runtime;

// Global Tokio runtime for async operations
static RUNTIME: OnceCell<Runtime> = OnceCell::new();

pub fn get_runtime() -> &'static Runtime {
    RUNTIME.get_or_init(|| {
        tokio::runtime::Builder::new_multi_thread()
            .enable_all()
            .build()
            .expect("Failed to create Tokio runtime")
    })
}

// Version check symbol for link-time verification
#[no_mangle]
pub extern "C" fn xet_version_1_0_0() {
    // This function exists purely as a link-time version check
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_runtime_initialization() {
        let runtime = get_runtime();
        assert!(runtime.handle().metrics().num_workers() > 0);
    }
}