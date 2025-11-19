# Error Mapping (Rust → C → Go)

| Rust source | C surface | Go surface | Notes |
| --- | --- | --- | --- |
| `XetErrorCode::Ok` (`error.rs:11`) | `code = 0` | Not surfaced (success path) | Reserved for completeness; no `XetError` returned.
| `XetErrorCode::InvalidConfig` (`error.rs:12`) | `code = 1` | `*xet.XetError` with `Code=1` | Used in `ffi.rs` when required pointers or UTF-8 strings are missing.
| `XetErrorCode::AuthFailed` (`error.rs:14`) | `code = 2` | `*xet.XetError` with `Code=2` | Used when HTTP 401 (Unauthorized) responses are detected. Can be checked via `xet.IsAuthFailedError()` or `xetErr.IsAuthFailed()`.
| `XetErrorCode::NetworkError` (`error.rs:16`) | `code = 3` | Currently unused | Prefer for transport errors once classified.
| `XetErrorCode::NotFound` (`error.rs:17`) | `code = 4` | `*xet.XetError` with `Code=4` | Used when HTTP 404 (Not Found) responses are detected.
| `XetErrorCode::PermissionDenied` (`error.rs:18`) | `code = 5` | `*xet.XetError` with `Code=5` | Used when HTTP 403 (Forbidden) responses are detected.
| `XetErrorCode::ChecksumMismatch` (`error.rs:19`) | `code = 6` | Currently unused | Intended for CAS integrity mismatches.
| `XetErrorCode::Cancelled` (`error.rs:20`) | `code = 7` | Currently unused | Use when cancellation propagates through FFI.
| `XetErrorCode::IoError` (`error.rs:21`) | `code = 8` | Currently unused | Map filesystem errors (e.g., disk full) once detected.
| `XetErrorCode::Unknown` (`error.rs:22`) | `code = 99` | Default in Go `convertError` | `XetError::from_anyhow` uses this for all uncategorized failures.

### Construction paths
- `ffi.rs` emits structured errors via `XetError::new` when validating incoming pointers and UTF-8 conversions.
- Any `anyhow::Error` bubbling up (network, CAS, IO) is turned into `Unknown` with `message` and `details` set to the formatted error/`Debug` output.
- Go’s `convertError` preserves `code`, `message`, and `details`, wrapping them in `*xet.XetError`.

### Adding new errors
1. Add or reuse an entry in `XetErrorCode` (`error.rs`).
2. Update FFI call sites to select the appropriate code instead of `Unknown`.
3. Extend Go `XetError` handling/tests to assert the new `Code`.
4. Regenerate `xet.h` so the enum values remain synchronized (even though `xet.h` exposes only the struct fields).