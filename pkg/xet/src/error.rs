use std::ffi::CString;
use std::os::raw::c_char;

#[repr(C)]
pub struct XetError {
    pub code: i32,
    pub message: *mut c_char,
    pub details: *mut c_char,
}

#[repr(i32)]
pub enum XetErrorCode {
    Ok = 0,
    InvalidConfig = 1,
    AuthFailed = 2,
    NetworkError = 3,
    NotFound = 4,
    PermissionDenied = 5,
    ChecksumMismatch = 6,
    Cancelled = 7,
    IoError = 8,
    Unknown = 99,
}

impl XetError {
    pub fn new(code: XetErrorCode, message: String, details: Option<String>) -> *mut XetError {
        let error = Box::new(XetError {
            code: code as i32,
            message: CString::new(message)
                .unwrap_or_else(|_| CString::new("Invalid error message").unwrap())
                .into_raw(),
            details: details
                .and_then(|d| CString::new(d).ok())
                .map(|s| s.into_raw())
                .unwrap_or(std::ptr::null_mut()),
        });
        Box::into_raw(error)
    }

    pub fn from_anyhow(err: anyhow::Error) -> *mut XetError {
        let message = format!("{}", err);
        let details = format!("{:?}", err);
        Self::new(XetErrorCode::Unknown, message, Some(details))
    }
}

/// Free an error returned by XET functions.
///
/// # Safety
///
/// Caller must ensure that:
/// - `err` is either null or a valid pointer returned by an XET function
/// - `err` is not used after calling this function
#[no_mangle]
pub unsafe extern "C" fn xet_free_error(err: *mut XetError) {
    if !err.is_null() {
        unsafe {
            let error = Box::from_raw(err);
            if !error.message.is_null() {
                let _ = CString::from_raw(error.message);
            }
            if !error.details.is_null() {
                let _ = CString::from_raw(error.details);
            }
        }
    }
}

/// Free a string returned by XET functions.
///
/// # Safety
///
/// Caller must ensure that:
/// - `s` is either null or a valid pointer returned by an XET function
/// - `s` is not used after calling this function
#[no_mangle]
pub unsafe extern "C" fn xet_free_string(s: *mut c_char) {
    if !s.is_null() {
        unsafe {
            let _ = CString::from_raw(s);
        }
    }
}
