// Module declarations - following hf_xet structure
mod runtime;
mod progress;
mod token_refresh;
mod error;
mod hf_adapter;
mod xet_integration;
mod xet_downloader;
mod ffi;

// Public exports
pub use error::*;
pub use ffi::*;

// Re-export runtime utilities
pub use runtime::{get_runtime, block_on};

use std::sync::Arc;
use anyhow::Result;

// Main client structure
pub struct XetClient {
    adapter: hf_adapter::HfAdapter,
}

impl XetClient {
    /// Create a new XET client
    pub fn new(
        endpoint: Option<String>,
        token: Option<String>,
        cache_dir: Option<String>,
        max_concurrent: u32,
        enable_dedup: bool,
    ) -> Result<Self> {
        let endpoint = endpoint.unwrap_or_else(|| "https://huggingface.co".to_string());
        let adapter = hf_adapter::HfAdapter::new(
            endpoint,
            token,
            cache_dir,
            max_concurrent as usize,
            enable_dedup,
        )?;
        Ok(Self { adapter })
    }
    
    /// List files in a repository
    pub async fn list_files(&self, repo_id: &str, revision: Option<&str>) -> Result<Vec<hf_adapter::HfFileInfo>> {
        self.adapter.list_files(repo_id, revision).await
    }
    
    /// Download a single file
    pub async fn download_file(
        &self,
        repo_id: &str,
        filename: &str,
        repo_type: Option<&str>,
        revision: Option<&str>,
        local_dir: Option<&str>,
        progress: Option<Arc<dyn Fn(&str, u64, u64) + Send + Sync>>,
    ) -> Result<String> {
        self.adapter.download_file(
            repo_id,
            filename,
            repo_type,
            revision,
            local_dir,
            progress,
        ).await
    }
    
    /// Download entire repository
    pub async fn download_snapshot(
        &self,
        repo_id: &str,
        repo_type: Option<&str>,
        revision: Option<&str>,
        local_dir: &str,
        allow_patterns: Option<Vec<String>>,
        ignore_patterns: Option<Vec<String>>,
        progress: Option<Arc<dyn Fn(&str, u64, u64) + Send + Sync>>,
    ) -> Result<String> {
        self.adapter.download_snapshot(
            repo_id,
            repo_type,
            revision,
            local_dir,
            allow_patterns,
            ignore_patterns,
            progress,
        ).await
    }
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