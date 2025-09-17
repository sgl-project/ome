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
    char** out_path
);

XetError* xet_download_snapshot(
    XetClient* client,
    const char* repo_id,
    const char* repo_type,
    const char* revision,
    const char* local_dir,
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