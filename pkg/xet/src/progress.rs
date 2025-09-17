// Progress tracking module - similar to hf_xet/progress_update.rs
use async_trait::async_trait;
use progress_tracking::{ProgressUpdate, TrackingProgressUpdater};
use std::sync::Arc;

/// Wrapper for progress callbacks that implements TrackingProgressUpdater
#[allow(dead_code)]
pub struct ProgressReporter {
    callback: crate::ProgressCallback,
}

#[allow(dead_code)]
impl ProgressReporter {
    pub fn new(callback: crate::ProgressCallback) -> Self {
        Self { callback }
    }
}

#[async_trait]
impl TrackingProgressUpdater for ProgressReporter {
    async fn register_updates(&self, updates: ProgressUpdate) {
        // Convert item progress to our callback format
        for item in &updates.item_updates {
            (self.callback)(&item.item_name, item.bytes_completed, item.total_bytes);
        }
    }

    async fn flush(&self) {
        // Nothing to do on flush for our use case
    }
}

/// C-compatible progress callback type for FFI
pub type CProgressCallback = extern "C" fn(*const libc::c_char, u64, u64, *mut libc::c_void);

/// Wrapper struct to make C callback thread-safe
struct CCallbackWrapper {
    callback: CProgressCallback,
    user_data: *mut libc::c_void,
}

// Safety: The C callback and user_data are expected to be thread-safe from C side
unsafe impl Send for CCallbackWrapper {}
unsafe impl Sync for CCallbackWrapper {}

/// Wrapper to convert C progress callback to Rust closure
pub fn wrap_c_progress_callback(
    callback: Option<CProgressCallback>,
    user_data: *mut libc::c_void,
) -> Option<crate::ProgressCallback> {
    callback.map(|cb| {
        let wrapper = Arc::new(CCallbackWrapper {
            callback: cb,
            user_data,
        });
        Arc::new(move |path: &str, downloaded: u64, total: u64| {
            use std::ffi::CString;
            if let Ok(c_path) = CString::new(path) {
                (wrapper.callback)(c_path.as_ptr(), downloaded, total, wrapper.user_data);
            }
        }) as crate::ProgressCallback
    })
}