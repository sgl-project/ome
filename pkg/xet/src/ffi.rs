use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_void};
use std::ptr;
use std::sync::Arc;

use anyhow::{Context, Result};

use crate::error::{XetError, XetErrorCode};
use crate::hf_adapter::{HfAdapter, HfFileInfo};
use crate::get_runtime;

// Opaque client handle
pub struct XetClient {
    adapter: HfAdapter,
}

#[repr(C)]
pub struct XetConfig {
    pub endpoint: *const c_char,
    pub token: *const c_char,
    pub cache_dir: *const c_char,
    pub max_concurrent_downloads: u32,
    pub enable_dedup: bool,
}

#[repr(C)]
pub struct XetDownloadRequest {
    pub repo_id: *const c_char,
    pub repo_type: *const c_char,
    pub revision: *const c_char,
    pub filename: *const c_char,
    pub local_dir: *const c_char,
}

#[repr(C)]
pub struct XetFileInfoC {
    pub path: *mut c_char,
    pub hash: *mut c_char,
    pub size: u64,
}

#[repr(C)]
pub struct XetFileList {
    pub files: *mut XetFileInfoC,
    pub count: usize,
}

// Progress callback type
pub type XetProgressCallback = extern "C" fn(
    file_path: *const c_char,
    downloaded: u64,
    total: u64,
    user_data: *mut c_void,
);

// Cancellation check callback
pub type XetCancelCallback = extern "C" fn(user_data: *mut c_void) -> bool;

// Helper to convert C string to Rust String
unsafe fn c_str_to_string(s: *const c_char) -> Option<String> {
    if s.is_null() {
        None
    } else {
        CStr::from_ptr(s).to_str().ok().map(|s| s.to_string())
    }
}

// Client creation and destruction
#[no_mangle]
pub extern "C" fn xet_client_new(config: *const XetConfig) -> *mut XetClient {
    if config.is_null() {
        return ptr::null_mut();
    }

    unsafe {
        let config = &*config;
        
        let endpoint = c_str_to_string(config.endpoint)
            .unwrap_or_else(|| "https://huggingface.co".to_string());
        let token = c_str_to_string(config.token);
        let cache_dir = c_str_to_string(config.cache_dir);
        let max_concurrent = if config.max_concurrent_downloads > 0 {
            config.max_concurrent_downloads as usize
        } else {
            4
        };

        match HfAdapter::new(endpoint, token, cache_dir, max_concurrent, config.enable_dedup) {
            Ok(adapter) => {
                let client = Box::new(XetClient { adapter });
                Box::into_raw(client)
            }
            Err(_) => ptr::null_mut(),
        }
    }
}

#[no_mangle]
pub extern "C" fn xet_client_free(client: *mut XetClient) {
    if !client.is_null() {
        unsafe {
            let _ = Box::from_raw(client);
        }
    }
}

// List files in a repository
#[no_mangle]
pub extern "C" fn xet_list_files(
    client: *mut XetClient,
    repo_id: *const c_char,
    revision: *const c_char,
    out_files: *mut *mut XetFileList,
) -> *mut XetError {
    if client.is_null() || repo_id.is_null() || out_files.is_null() {
        return XetError::new(
            XetErrorCode::InvalidConfig,
            "Invalid parameters".to_string(),
            None,
        );
    }

    unsafe {
        let client = &*client;
        let repo_id = match c_str_to_string(repo_id) {
            Some(s) => s,
            None => {
                return XetError::new(
                    XetErrorCode::InvalidConfig,
                    "Invalid repo_id".to_string(),
                    None,
                );
            }
        };
        let revision = c_str_to_string(revision);

        let runtime = get_runtime();
        let result = runtime.block_on(async {
            client.adapter.list_files(&repo_id, revision.as_deref()).await
        });

        match result {
            Ok(files) => {
                let count = files.len();
                let mut c_files = Vec::with_capacity(count);
                
                for file in files {
                    let c_file = XetFileInfoC {
                        path: CString::new(file.path).unwrap().into_raw(),
                        hash: CString::new(file.hash).unwrap().into_raw(),
                        size: file.size,
                    };
                    c_files.push(c_file);
                }

                let file_list = Box::new(XetFileList {
                    files: c_files.as_mut_ptr(),
                    count,
                });
                std::mem::forget(c_files); // Prevent deallocation
                
                *out_files = Box::into_raw(file_list);
                ptr::null_mut()
            }
            Err(e) => XetError::from_anyhow(e),
        }
    }
}

// Download a file
#[no_mangle]
pub extern "C" fn xet_download_file(
    client: *mut XetClient,
    request: *const XetDownloadRequest,
    progress: Option<XetProgressCallback>,
    _user_data: *mut c_void,
    out_path: *mut *mut c_char,
) -> *mut XetError {
    if client.is_null() || request.is_null() || out_path.is_null() {
        return XetError::new(
            XetErrorCode::InvalidConfig,
            "Invalid parameters".to_string(),
            None,
        );
    }

    unsafe {
        // Debug log
        if let Ok(mut f) = std::fs::OpenOptions::new().create(true).append(true).open("/tmp/xet-debug.log") {
            use std::io::Write;
            writeln!(f, "\n=== FFI xet_download_file called ===").ok();
        }
        
        let client = &*client;
        let request = &*request;
        
        let repo_id = match c_str_to_string(request.repo_id) {
            Some(s) => s,
            None => {
                return XetError::new(
                    XetErrorCode::InvalidConfig,
                    "Invalid repo_id".to_string(),
                    None,
                );
            }
        };
        
        let filename = match c_str_to_string(request.filename) {
            Some(s) => s,
            None => {
                return XetError::new(
                    XetErrorCode::InvalidConfig,
                    "Invalid filename".to_string(),
                    None,
                );
            }
        };

        let repo_type = c_str_to_string(request.repo_type);
        let revision = c_str_to_string(request.revision);
        let local_dir = c_str_to_string(request.local_dir);

        // Create progress callback wrapper if provided
        let progress_callback = progress.map(|cb| {
            Arc::new(move |path: &str, downloaded: u64, total: u64| {
                // Marshal the callback to C
                let c_path = CString::new(path).unwrap_or_else(|_| CString::new("").unwrap());
                // Note: user_data is not used in this implementation
                // In production, you'd need a different approach to handle user_data safely
                cb(c_path.as_ptr(), downloaded, total, ptr::null_mut());
            }) as Arc<dyn Fn(&str, u64, u64) + Send + Sync>
        });

        let runtime = get_runtime();
        let result = runtime.block_on(async {
            client.adapter.download_file(
                &repo_id,
                &filename,
                repo_type.as_deref(),
                revision.as_deref(),
                local_dir.as_deref(),
                progress_callback,
            ).await
        });

        match result {
            Ok(path) => {
                *out_path = CString::new(path).unwrap().into_raw();
                ptr::null_mut()
            }
            Err(e) => XetError::from_anyhow(e),
        }
    }
}

// Download snapshot (all files) with parallel downloads
#[no_mangle]
pub extern "C" fn xet_download_snapshot(
    client: *mut XetClient,
    repo_id: *const c_char,
    repo_type: *const c_char,
    revision: *const c_char,
    local_dir: *const c_char,
    progress: Option<XetProgressCallback>,
    user_data: *mut c_void,
    out_path: *mut *mut c_char,
) -> *mut XetError {
    if client.is_null() || repo_id.is_null() || local_dir.is_null() || out_path.is_null() {
        return XetError::new(
            XetErrorCode::InvalidConfig,
            "Invalid parameters".to_string(),
            None,
        );
    }

    unsafe {
        let client = &*client;
        let repo_id = match c_str_to_string(repo_id) {
            Some(s) => s,
            None => {
                return XetError::new(
                    XetErrorCode::InvalidConfig,
                    "Invalid repo_id".to_string(),
                    None,
                );
            }
        };
        
        let repo_type = c_str_to_string(repo_type);
        let revision = c_str_to_string(revision);
        let local_dir = match c_str_to_string(local_dir) {
            Some(s) => s,
            None => {
                return XetError::new(
                    XetErrorCode::InvalidConfig,
                    "Invalid local_dir".to_string(),
                    None,
                );
            }
        };

        // Create progress callback wrapper if provided
        let progress_callback = progress.map(|cb| {
            Arc::new(move |path: &str, downloaded: u64, total: u64| {
                let c_path = CString::new(path).unwrap_or_else(|_| CString::new("").unwrap());
                cb(c_path.as_ptr(), downloaded, total, ptr::null_mut());
            }) as Arc<dyn Fn(&str, u64, u64) + Send + Sync>
        });

        let runtime = get_runtime();
        let result = runtime.block_on(async {
            client.adapter.download_snapshot(
                &repo_id,
                repo_type.as_deref(),
                revision.as_deref(),
                &local_dir,
                None, // allow_patterns
                None, // ignore_patterns
                progress_callback,
            ).await
        });

        match result {
            Ok(path) => {
                *out_path = CString::new(path).unwrap().into_raw();
                ptr::null_mut()
            }
            Err(e) => XetError::from_anyhow(e),
        }
    }
}

// Free file list
#[no_mangle]
pub extern "C" fn xet_free_file_list(list: *mut XetFileList) {
    if !list.is_null() {
        unsafe {
            let list = Box::from_raw(list);
            for i in 0..list.count {
                let file = &*list.files.add(i);
                if !file.path.is_null() {
                    let _ = CString::from_raw(file.path);
                }
                if !file.hash.is_null() {
                    let _ = CString::from_raw(file.hash);
                }
            }
            // Free the array itself
            Vec::from_raw_parts(list.files, list.count, list.count);
        }
    }
}