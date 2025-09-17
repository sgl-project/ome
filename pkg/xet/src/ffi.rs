use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_void};
use std::ptr;
use std::sync::Arc;

use crate::error::{XetError, XetErrorCode};
use crate::progress::XetProgressCallback;
use crate::{block_on, DownloadOptions, OperationContext, SnapshotOptions, XetClient};

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

#[repr(C)]
pub struct XetCancellationToken {
    pub callback: Option<unsafe extern "C" fn(*mut c_void) -> bool>,
    pub user_data: *mut c_void,
}

// Helper to construct a cancellation checker closure
unsafe fn make_cancel_check(
    token: *const XetCancellationToken,
) -> Option<Arc<dyn Fn() -> bool + Send + Sync>> {
    if token.is_null() {
        return None;
    }

    let token = &*token;
    let callback = token.callback?;
    let user_data = token.user_data as usize;
    Some(Arc::new(move || unsafe {
        callback(user_data as *mut c_void)
    }))
}

// Helper to convert C string to Rust String
unsafe fn c_str_to_string(s: *const c_char) -> Option<String> {
    if s.is_null() {
        None
    } else {
        CStr::from_ptr(s).to_str().ok().map(|s| s.to_string())
    }
}

/// Create a new XET client.
///
/// # Safety
///
/// Caller must ensure that:
/// - `config` is either null or a valid pointer to XetConfig
/// - Returned pointer must be freed with `xet_client_free`
#[no_mangle]
pub unsafe extern "C" fn xet_client_new(config: *const XetConfig) -> *mut XetClient {
    if config.is_null() {
        return ptr::null_mut();
    }

    unsafe {
        let config = &*config;

        let endpoint = c_str_to_string(config.endpoint);
        let token = c_str_to_string(config.token);
        let cache_dir = c_str_to_string(config.cache_dir);
        let max_concurrent = if config.max_concurrent_downloads > 0 {
            config.max_concurrent_downloads
        } else {
            4
        };

        match XetClient::new(
            endpoint,
            token,
            cache_dir,
            max_concurrent,
            config.enable_dedup,
        ) {
            Ok(client) => Box::into_raw(Box::new(client)),
            Err(_) => ptr::null_mut(),
        }
    }
}

/// Free an XET client.
///
/// # Safety
///
/// Caller must ensure that:
/// - `client` is either null or a valid pointer returned by `xet_client_new`
/// - `client` is not used after calling this function
#[no_mangle]
pub unsafe extern "C" fn xet_client_free(client: *mut XetClient) {
    if !client.is_null() {
        unsafe {
            let _ = Box::from_raw(client);
        }
    }
}

/// Register (or clear) the FFI progress callback.
///
/// # Safety
///
/// * `client` must be a valid pointer returned by `xet_client_new`.
/// * `callback` and `user_data` must remain valid for the duration of the registration.
/// * Callers must eventually unregister (pass NULL) before freeing the client.
#[no_mangle]
pub unsafe extern "C" fn xet_client_set_progress_callback(
    client: *mut XetClient,
    callback: Option<XetProgressCallback>,
    user_data: *mut c_void,
    throttle_ms: u32,
) -> *mut XetError {
    if client.is_null() {
        return XetError::new(
            XetErrorCode::InvalidConfig,
            "Invalid client".to_string(),
            None,
        );
    }

    let client_ref = &*client;
    client_ref.configure_progress_callback(callback, user_data, throttle_ms);
    ptr::null_mut()
}

/// List files in a repository.
///
/// # Safety
///
/// Caller must ensure that:
/// - All pointers are valid or null
/// - Strings are valid UTF-8
/// - `out_files` must be freed with `xet_free_file_list`
#[no_mangle]
pub unsafe extern "C" fn xet_list_files(
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

        let result = block_on(async { client.list_files(&repo_id, revision.as_deref()).await });

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

/// Download a file from a repository.
///
/// # Safety
///
/// Caller must ensure that:
/// - All pointers are valid or null
/// - Strings are valid UTF-8
/// - `out_path` must be freed with `xet_free_string`
#[no_mangle]
pub unsafe extern "C" fn xet_download_file(
    client: *mut XetClient,
    request: *const XetDownloadRequest,
    cancel_token: *const XetCancellationToken,
    out_path: *mut *mut c_char,
) -> *mut XetError {
    if client.is_null() || request.is_null() || out_path.is_null() {
        return XetError::new(
            XetErrorCode::InvalidConfig,
            "Invalid parameters".to_string(),
            None,
        );
    }

    let client_ref = unsafe { &*client };
    let request_ref = unsafe { &*request };

    let repo_id = match unsafe { c_str_to_string(request_ref.repo_id) } {
        Some(s) => s,
        None => {
            return XetError::new(
                XetErrorCode::InvalidConfig,
                "Invalid repo_id".to_string(),
                None,
            );
        }
    };

    let filename = match unsafe { c_str_to_string(request_ref.filename) } {
        Some(s) => s,
        None => {
            return XetError::new(
                XetErrorCode::InvalidConfig,
                "Invalid filename".to_string(),
                None,
            );
        }
    };

    let repo_type = unsafe { c_str_to_string(request_ref.repo_type) };
    let revision = unsafe { c_str_to_string(request_ref.revision) };
    let local_dir = unsafe { c_str_to_string(request_ref.local_dir) };

    let cancel_check = unsafe { make_cancel_check(cancel_token) };
    let progress = client_ref.new_progress_operation();
    let options = DownloadOptions {
        repo_type: repo_type.as_deref(),
        revision: revision.as_deref(),
        local_dir: local_dir.as_deref(),
    };
    let context = OperationContext::new(cancel_check, progress);

    let result = block_on(async {
        client_ref
            .download_file_with_options(&repo_id, &filename, options, context)
            .await
    });

    match result {
        Ok(path) => {
            unsafe {
                *out_path = CString::new(path).unwrap().into_raw();
            }
            ptr::null_mut()
        }
        Err(e) => XetError::from_anyhow(e),
    }
}

/// Download all files from a repository.
///
/// # Safety
///
/// Caller must ensure that:
/// - All pointers are valid or null
/// - Strings are valid UTF-8
/// - `out_path` must be freed with `xet_free_string`
#[no_mangle]
pub unsafe extern "C" fn xet_download_snapshot(
    client: *mut XetClient,
    repo_id: *const c_char,
    repo_type: *const c_char,
    revision: *const c_char,
    local_dir: *const c_char,
    cancel_token: *const XetCancellationToken,
    out_path: *mut *mut c_char,
) -> *mut XetError {
    if client.is_null() || repo_id.is_null() || local_dir.is_null() || out_path.is_null() {
        return XetError::new(
            XetErrorCode::InvalidConfig,
            "Invalid parameters".to_string(),
            None,
        );
    }

    let client_ref = unsafe { &*client };
    let repo_id = match unsafe { c_str_to_string(repo_id) } {
        Some(s) => s,
        None => {
            return XetError::new(
                XetErrorCode::InvalidConfig,
                "Invalid repo_id".to_string(),
                None,
            );
        }
    };

    let repo_type = unsafe { c_str_to_string(repo_type) };
    let revision = unsafe { c_str_to_string(revision) };
    let local_dir = match unsafe { c_str_to_string(local_dir) } {
        Some(s) => s,
        None => {
            return XetError::new(
                XetErrorCode::InvalidConfig,
                "Invalid local_dir".to_string(),
                None,
            );
        }
    };

    let cancel_check = unsafe { make_cancel_check(cancel_token) };
    let progress = client_ref.new_progress_operation();
    let options = SnapshotOptions {
        repo_type: repo_type.as_deref(),
        revision: revision.as_deref(),
        local_dir: &local_dir,
        allow_patterns: None,
        ignore_patterns: None,
    };
    let context = OperationContext::new(cancel_check, progress);

    let result = block_on(async {
        client_ref
            .download_snapshot_with_options(&repo_id, options, context)
            .await
    });

    match result {
        Ok(path) => {
            unsafe {
                *out_path = CString::new(path).unwrap().into_raw();
            }
            ptr::null_mut()
        }
        Err(e) => XetError::from_anyhow(e),
    }
}

/// Free a file list returned by `xet_list_files`.
///
/// # Safety
///
/// Caller must ensure that:
/// - `list` is either null or a valid pointer returned by `xet_list_files`
/// - `list` is not used after calling this function
#[no_mangle]
pub unsafe extern "C" fn xet_free_file_list(list: *mut XetFileList) {
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