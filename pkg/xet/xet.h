#ifndef XET_H
#define XET_H

#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

// Opaque client handle
typedef struct XetClient XetClient;

// Progress reporting
typedef enum {
    XET_PROGRESS_PHASE_SCANNING = 0,
    XET_PROGRESS_PHASE_DOWNLOADING = 1,
    XET_PROGRESS_PHASE_FINALIZING = 2,
} XetProgressPhase;

typedef struct {
    XetProgressPhase phase;
    uint64_t total_bytes;
    uint64_t completed_bytes;
    uint32_t total_files;
    uint32_t completed_files;
    const char* current_file;
    uint64_t current_file_completed_bytes;
    uint64_t current_file_total_bytes;
} XetProgressUpdate;

typedef void (*XetProgressCallback)(const XetProgressUpdate* update, void* user_data);

typedef bool (*XetCancellationCallback)(void* user_data);

typedef struct {
    XetCancellationCallback callback;
    void* user_data;
} XetCancellationToken;

// Configuration structure
typedef struct {
    const char* endpoint;
    const char* token;
    const char* cache_dir;
    uint32_t max_concurrent_downloads;
    bool enable_dedup;
} XetConfig;

// Download request structure
typedef struct {
    const char* repo_id;
    const char* repo_type;
    const char* revision;
    const char* filename;
    const char* local_dir;
} XetDownloadRequest;

// Snapshot download request
typedef struct {
    const char* repo_id;
    const char* repo_type;
    const char* revision;
    const char* local_dir;
    const char** allow_patterns;
    size_t allow_patterns_len;
    const char** ignore_patterns;
    size_t ignore_patterns_len;
} XetSnapshotRequest;

// File information
typedef struct {
    char* path;
    char* hash;
    uint64_t size;
} XetFileInfo;

// File list
typedef struct {
    XetFileInfo* files;
    size_t count;
} XetFileList;

// Error structure
typedef struct {
    int32_t code;
    char* message;
    char* details;
} XetError;


// Version check - link-time verification
extern void xet_version_1_0_0(void);

// Client lifecycle
XetClient* xet_client_new(const XetConfig* config);
void xet_client_free(XetClient* client);
XetError* xet_client_set_progress_callback(
    XetClient* client,
    XetProgressCallback callback,
    void* user_data,
    uint32_t throttle_ms
);

// Repository operations
XetError* xet_list_files(
    XetClient* client,
    const char* repo_id,
    const char* revision,
    XetFileList** out_files
);

// Download operations
XetError* xet_download_file(
    XetClient* client,
    const XetDownloadRequest* request,
    const XetCancellationToken* cancel_token,
    char** out_path
);

XetError* xet_download_snapshot(
    XetClient* client,
    const char* repo_id,
    const char* repo_type,
    const char* revision,
    const char* local_dir,
    const XetCancellationToken* cancel_token,
    char** out_path
);

// Memory management
void xet_free_error(XetError* err);
void xet_free_file_list(XetFileList* list);
void xet_free_string(char* str);

#ifdef __cplusplus
}
#endif

#endif // XET_H