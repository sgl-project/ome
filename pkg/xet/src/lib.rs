// Module declarations - following hf_xet structure
mod error;
mod ffi;
mod hf_adapter;
mod logging;
mod progress;
mod runtime;
mod xet_downloader;
mod xet_integration;

// Public exports
pub use error::*;
pub use ffi::*;
pub use progress::{XetProgressCallback, XetProgressPhase, XetProgressUpdate};

// Re-export runtime utilities
pub use runtime::{block_on, get_runtime};

use anyhow::Result;
use progress::{OperationProgress, ProgressHandler};
use std::os::raw::c_void;
use std::sync::Arc;

pub(crate) struct DownloadOptions<'a> {
    pub repo_type: Option<&'a str>,
    pub revision: Option<&'a str>,
    pub local_dir: Option<&'a str>,
}

pub(crate) struct SnapshotOptions<'a> {
    pub repo_type: Option<&'a str>,
    pub revision: Option<&'a str>,
    pub local_dir: &'a str,
    pub allow_patterns: Option<Vec<String>>,
    pub ignore_patterns: Option<Vec<String>>,
}

#[derive(Default)]
pub(crate) struct OperationContext {
    pub cancel_check: Option<Arc<dyn Fn() -> bool + Send + Sync>>,
    pub progress: Option<OperationProgress>,
}

impl OperationContext {
    pub fn new(
        cancel_check: Option<Arc<dyn Fn() -> bool + Send + Sync>>,
        progress: Option<OperationProgress>,
    ) -> Self {
        Self {
            cancel_check,
            progress,
        }
    }
}

// Main client structure
pub struct XetClient {
    adapter: hf_adapter::HfAdapter,
    progress: ProgressHandler,
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
        // Initialize logging on first client creation
        crate::logging::init_logging();

        let endpoint = endpoint.unwrap_or_else(|| "https://huggingface.co".to_string());
        let adapter = hf_adapter::HfAdapter::new(
            endpoint,
            token,
            cache_dir,
            max_concurrent as usize,
            enable_dedup,
        )?;
        Ok(Self {
            adapter,
            progress: ProgressHandler::default(),
        })
    }

    pub(crate) fn configure_progress_callback(
        &self,
        callback: Option<XetProgressCallback>,
        user_data: *mut c_void,
        throttle_ms: u32,
    ) {
        self.progress.configure(callback, user_data, throttle_ms);
    }

    pub(crate) fn new_progress_operation(&self) -> Option<OperationProgress> {
        self.progress.new_operation()
    }

    /// List files in a repository
    pub async fn list_files(
        &self,
        repo_id: &str,
        revision: Option<&str>,
    ) -> Result<Vec<hf_adapter::HfFileInfo>> {
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
    ) -> Result<String> {
        self.download_file_with_options(
            repo_id,
            filename,
            DownloadOptions {
                repo_type,
                revision,
                local_dir,
            },
            OperationContext::default(),
        )
        .await
    }

    pub(crate) async fn download_file_with_options(
        &self,
        repo_id: &str,
        filename: &str,
        options: DownloadOptions<'_>,
        context: OperationContext,
    ) -> Result<String> {
        let OperationContext {
            cancel_check,
            progress,
        } = context;

        self.adapter
            .download_file_with_cancel(
                repo_id,
                filename,
                options.repo_type,
                options.revision,
                options.local_dir,
                cancel_check,
                progress,
            )
            .await
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
    ) -> Result<String> {
        self.download_snapshot_with_options(
            repo_id,
            SnapshotOptions {
                repo_type,
                revision,
                local_dir,
                allow_patterns,
                ignore_patterns,
            },
            OperationContext::default(),
        )
        .await
    }

    pub(crate) async fn download_snapshot_with_options(
        &self,
        repo_id: &str,
        options: SnapshotOptions<'_>,
        context: OperationContext,
    ) -> Result<String> {
        let OperationContext {
            cancel_check,
            progress,
        } = context;

        self.adapter
            .download_snapshot(
                repo_id,
                options.repo_type,
                options.revision,
                options.local_dir,
                options.allow_patterns,
                options.ignore_patterns,
                cancel_check,
                progress,
            )
            .await
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