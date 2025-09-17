use std::collections::HashMap;
use std::ffi::CString;
use std::os::raw::{c_char, c_void};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use progress_tracking::ProgressUpdate as TrackingProgressUpdate;

#[repr(C)]
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum XetProgressPhase {
    Scanning = 0,
    Downloading = 1,
    Finalizing = 2,
}

#[repr(C)]
pub struct XetProgressUpdate {
    pub phase: XetProgressPhase,
    pub total_bytes: u64,
    pub completed_bytes: u64,
    pub total_files: u32,
    pub completed_files: u32,
    pub current_file: *const c_char,
    pub current_file_completed_bytes: u64,
    pub current_file_total_bytes: u64,
}

pub type XetProgressCallback = unsafe extern "C" fn(*const XetProgressUpdate, *mut c_void);

#[derive(Clone)]
pub(crate) struct ProgressHandler {
    inner: Arc<Mutex<ProgressHandlerConfig>>,
}

#[derive(Clone, Copy)]
struct ProgressHandlerConfig {
    callback: Option<XetProgressCallback>,
    user_data: usize,
    throttle: Duration,
}

impl Default for ProgressHandler {
    fn default() -> Self {
        Self {
            inner: Arc::new(Mutex::new(ProgressHandlerConfig::disabled())),
        }
    }
}

impl ProgressHandlerConfig {
    fn disabled() -> Self {
        Self {
            callback: None,
            user_data: 0,
            throttle: Duration::from_millis(200),
        }
    }
}

impl ProgressHandler {
    pub fn configure(
        &self,
        callback: Option<XetProgressCallback>,
        user_data: *mut c_void,
        throttle_ms: u32,
    ) {
        let mut guard = self.inner.lock().expect("progress mutex poisoned");
        *guard = ProgressHandlerConfig {
            callback,
            user_data: user_data as usize,
            throttle: if throttle_ms == 0 {
                Duration::from_millis(200)
            } else {
                Duration::from_millis(throttle_ms as u64)
            },
        };
    }

    pub fn new_operation(&self) -> Option<OperationProgress> {
        let config = *self.inner.lock().expect("progress mutex poisoned");
        config
            .callback
            .map(|callback| OperationProgress::new(config, callback))
    }
}

#[derive(Clone)]
pub(crate) struct OperationProgress {
    callback: XetProgressCallback,
    user_data: usize,
    throttle: Duration,
    inner: Arc<Mutex<OperationProgressState>>,
}

struct OperationProgressState {
    phase: XetProgressPhase,
    total_bytes: u64,
    completed_bytes: u64,
    files: HashMap<String, FileProgress>,
    total_files_hint: Option<usize>,
    last_emit: Option<Instant>,
}

#[derive(Default)]
struct FileProgress {
    total_bytes: u64,
    completed_bytes: u64,
}

enum EmitFileInfo<'a> {
    None,
    WithData {
        name: &'a str,
        completed: u64,
        total: u64,
    },
}

impl OperationProgress {
    fn new(config: ProgressHandlerConfig, callback: XetProgressCallback) -> Self {
        Self {
            callback,
            user_data: config.user_data,
            throttle: config.throttle,
            inner: Arc::new(Mutex::new(OperationProgressState {
                phase: XetProgressPhase::Scanning,
                total_bytes: 0,
                completed_bytes: 0,
                files: HashMap::new(),
                total_files_hint: None,
                last_emit: None,
            })),
        }
    }

    pub fn set_phase(&self, phase: XetProgressPhase, force: bool) {
        {
            let mut state = self.inner.lock().expect("progress mutex poisoned");
            if state.phase != phase {
                state.phase = phase;
            }
        }
        self.emit(EmitFileInfo::None, force);
    }

    pub fn set_total_hint(&self, total_files: usize, total_bytes: u64) {
        let mut state = self.inner.lock().expect("progress mutex poisoned");
        state.total_files_hint = Some(total_files);
        if total_bytes > state.total_bytes {
            state.total_bytes = total_bytes;
        }
    }

    pub fn set_total_bytes(&self, total_bytes: u64) {
        let mut state = self.inner.lock().expect("progress mutex poisoned");
        if total_bytes > state.total_bytes {
            state.total_bytes = total_bytes;
        }
    }

    pub fn set_completed_bytes(&self, completed_bytes: u64) {
        let mut state = self.inner.lock().expect("progress mutex poisoned");
        if completed_bytes > state.completed_bytes {
            state.completed_bytes = completed_bytes;
        }
    }

    pub fn ensure_file_entry(&self, name: &str, total_bytes: u64) {
        let mut state = self.inner.lock().expect("progress mutex poisoned");
        let entry = state.files.entry(name.to_string()).or_default();
        if total_bytes > 0 {
            entry.total_bytes = entry.total_bytes.max(total_bytes);
        }
    }

    pub fn update_file_absolute(&self, name: &str, completed: u64, total: u64, force: bool) {
        let mut state = self.inner.lock().expect("progress mutex poisoned");
        let entry = state.files.entry(name.to_string()).or_default();
        if total > 0 {
            entry.total_bytes = entry.total_bytes.max(total);
        }
        if completed > entry.completed_bytes {
            let delta = completed - entry.completed_bytes;
            entry.completed_bytes = completed;
            state.completed_bytes = state.completed_bytes.saturating_add(delta);
        }
        drop(state);
        self.emit(
            EmitFileInfo::WithData {
                name,
                completed,
                total,
            },
            force,
        );
    }

    pub fn apply_tracking_update(&self, update: &TrackingProgressUpdate) {
        self.set_total_bytes(update.total_transfer_bytes);
        self.set_completed_bytes(update.total_transfer_bytes_completed);

        for item in &update.item_updates {
            self.update_file_absolute(
                &item.item_name,
                item.bytes_completed,
                item.total_bytes,
                false,
            );
        }
    }

    pub fn finalize(&self) {
        self.set_phase(XetProgressPhase::Finalizing, true);
    }

    pub fn force_emit(&self) {
        self.emit(EmitFileInfo::None, true);
    }

    pub fn clone_for_tasks(&self) -> Self {
        Self {
            callback: self.callback,
            user_data: self.user_data,
            throttle: self.throttle,
            inner: self.inner.clone(),
        }
    }

    fn emit(&self, file_info: EmitFileInfo<'_>, force: bool) {
        let mut state = self.inner.lock().expect("progress mutex poisoned");
        if !force {
            if let Some(last) = state.last_emit {
                if last.elapsed() < self.throttle {
                    return;
                }
            }
        }

        let now = Instant::now();

        let total_files_hint = state.total_files_hint.unwrap_or_else(|| state.files.len());
        let total_files = total_files_hint.min(u32::MAX as usize) as u32;
        let completed_files = state
            .files
            .values()
            .filter(|entry| entry.total_bytes > 0 && entry.completed_bytes >= entry.total_bytes)
            .count()
            .min(u32::MAX as usize) as u32;

        let mut temp_storage: Option<CString> = None;
        let mut file_name_ptr = std::ptr::null();
        let mut file_completed = 0;
        let mut file_total = 0;

        if let EmitFileInfo::WithData {
            name,
            completed,
            total,
        } = file_info
        {
            if let Ok(value) = CString::new(name) {
                file_name_ptr = value.as_ptr();
                temp_storage = Some(value);
            }
            file_completed = completed;
            file_total = total;
        }

        let update = XetProgressUpdate {
            phase: state.phase,
            total_bytes: state.total_bytes,
            completed_bytes: state.completed_bytes,
            total_files,
            completed_files,
            current_file: file_name_ptr,
            current_file_completed_bytes: file_completed,
            current_file_total_bytes: file_total,
        };

        state.last_emit = Some(now);
        let callback = self.callback;
        let user_data_ptr = self.user_data as *mut c_void;
        drop(state);

        let _keep_alive = temp_storage;

        unsafe {
            callback(&update as *const XetProgressUpdate, user_data_ptr);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicUsize, Ordering};

    extern "C" fn test_callback(update: *const XetProgressUpdate, counter: *mut c_void) {
        let counter = unsafe { &*(counter as *const AtomicUsize) };
        assert!(!update.is_null());
        counter.fetch_add(1, Ordering::Relaxed);
    }

    #[test]
    fn test_progress_emits() {
        let handler = ProgressHandler::default();
        let hits = AtomicUsize::new(0);
        handler.configure(Some(test_callback), &hits as *const _ as *mut c_void, 0);
        let progress = handler
            .new_operation()
            .expect("operation should be available");
        progress.set_phase(XetProgressPhase::Scanning, true);
        progress.set_total_hint(1, 100);
        progress.ensure_file_entry("file", 100);
        progress.update_file_absolute("file", 50, 100, true);
        progress.update_file_absolute("file", 100, 100, true);
        progress.finalize();

        assert!(hits.load(Ordering::Relaxed) >= 3);
    }
}
