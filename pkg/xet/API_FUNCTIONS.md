# API Function Index

| Symbol | Layer | Location | Notes |
| --- | --- | --- | --- |
| `xet_version_1_0_0` | Rust | `src/lib.rs:61` | Link-time ABI guard referenced by Go `#cgo`.
| `xet_client_new` | Rust FFI | `src/ffi.rs:56` | Creates an `XetClient` from `XetConfig`.
| `xet_client_free` | Rust FFI | `src/ffi.rs:94` | Drops the client handle; safe on NULL.
| `xet_list_files` | Rust FFI | `src/ffi.rs:111` | Returns a heap-allocated `XetFileList`.
| `xet_download_file` | Rust FFI | `src/ffi.rs:178` | Downloads one file; returns heap string path.
| `xet_download_snapshot` | Rust FFI | `src/ffi.rs:252` | Downloads repository snapshot.
| `xet_free_file_list` | Rust FFI | `src/ffi.rs:325` | Frees list + nested strings.
| `xet_free_error` | Rust FFI | `src/error.rs:55` | Frees `XetError` payload.
| `xet_free_string` | Rust FFI | `src/error.rs:77` | Frees `char*` returned by Rust.
| `SetLogLevel` | Go | `xet.go:42` | Updates `RUST_LOG` before client creation.
| `NewClient` | Go | `xet.go:102` | Wraps `xet_client_new`.
| `(*Client) Close` | Go | `xet.go:149` | Releases native handle.
| `(*Client) ListFiles` | Go | `xet.go:161` | Converts `XetFileList` to Go slice.
| `(*Client) DownloadFile` | Go | `xet.go:200` | Wraps `xet_download_file`.
| `(*Client) DownloadFileWithContext` | Go | `xet.go:218` | Placeholder for cancellation.
| `(*Client) DownloadSnapshot` | Go | `xet.go:225` | Wraps `xet_download_snapshot`.
| `HfHubDownload` | Go | `hf_compat.go:72` | HF-compatible single-file download.
| `SnapshotDownload` | Go | `hf_compat.go:138` | HF-compatible snapshot download.
| `ListRepoFiles` | Go | `hf_compat.go:211` | HF-compatible list operation.