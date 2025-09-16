use std::path::{Path, PathBuf};
use std::sync::Arc;
use anyhow::{Result, anyhow};
use reqwest;
use serde::Deserialize;
use tokio::fs;
use futures::stream::{self, StreamExt};
use crate::xet_integration::{XetFileData, XetTokenManager, parse_xet_file_data_from_headers};
// For xet-core integration (commented out for now)
// use utils::auth::TokenRefresher;
// use utils::errors::AuthError;
// use progress_tracking::{TrackingProgressUpdater, ProgressUpdate, ItemProgressUpdate};
// use async_trait::async_trait;

#[derive(Clone)]
pub struct HfAdapter {
    endpoint: String,
    token: Option<String>,
    cache_dir: Option<PathBuf>,
    max_concurrent: usize,
    enable_dedup: bool,
    client: reqwest::Client,
    xet_token_manager: Arc<tokio::sync::Mutex<XetTokenManager>>,
}

#[derive(Debug, Clone)]
pub struct HfFileInfo {
    pub path: String,
    pub hash: String,
    pub size: u64,
    pub xet_hash: Option<String>,  // XET hash if available
}

// HF API response structures
#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct HfTreeItem {
    #[serde(rename = "type")]
    item_type: String,
    oid: String,
    size: u64,
    path: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    xet_hash: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    lfs: Option<LfsInfo>,
}

#[derive(Debug, Deserialize)]
struct LfsInfo {
    oid: String,
    size: u64,
    #[serde(rename = "pointerSize")]
    pointer_size: u64,
}

/* Token refresher implementation for xet-core (not used yet)
struct HfTokenRefresher {
    token: String,
}

#[async_trait]
impl TokenRefresher for HfTokenRefresher {
    async fn refresh(&self) -> Result<(String, u64), AuthError> {
        // For now, return the same token with 1 hour expiration
        // In production, this would refresh the token from HF API
        Ok((self.token.clone(), 3600))
    }
}
*/

// Progress updater wrapper for FFI callback
struct HfProgressUpdater {
    callback: Arc<dyn Fn(&str, u64, u64) + Send + Sync>,
    current_file: String,
    total_size: u64,
}

/* Not used yet - requires xet-core traits
#[async_trait]
impl TrackingProgressUpdater for HfProgressUpdater {
    async fn register_updates(&self, updates: ProgressUpdate) {
        // Convert progress updates to our callback format
        for item in updates.item_updates {
            if item.item_name.as_ref() == self.current_file {
                (self.callback)(
                    &self.current_file,
                    item.bytes_completed,
                    item.total_bytes,
                );
            }
        }
    }

    async fn flush(&self) {
        // Notify completion
        (self.callback)(&self.current_file, self.total_size, self.total_size);
    }
}
*/

impl HfAdapter {
    pub fn new(
        endpoint: String,
        token: Option<String>,
        cache_dir: Option<String>,
        max_concurrent: usize,
        enable_dedup: bool,
    ) -> Result<Self> {
        let cache_dir = cache_dir.map(PathBuf::from);
        
        let mut headers = reqwest::header::HeaderMap::new();
        if let Some(ref token) = token {
            headers.insert(
                reqwest::header::AUTHORIZATION,
                reqwest::header::HeaderValue::from_str(&format!("Bearer {}", token))?,
            );
        }
        
        let client = reqwest::Client::builder()
            .default_headers(headers)
            .build()?;
        
        let xet_token_manager = Arc::new(tokio::sync::Mutex::new(
            XetTokenManager::new(token.clone())
        ));
        
        Ok(HfAdapter {
            endpoint,
            token,
            cache_dir,
            max_concurrent,
            enable_dedup,
            client,
            xet_token_manager,
        })
    }

    pub async fn list_files(
        &self,
        repo_id: &str,
        revision: Option<&str>,
    ) -> Result<Vec<HfFileInfo>> {
        let revision = revision.unwrap_or("main");
        let url = format!("{}/api/models/{}/tree/{}", self.endpoint, repo_id, revision);
        
        // Make HTTP request to HF API
        let response = self.client.get(&url).send().await?;
        
        if !response.status().is_success() {
            return Err(anyhow!(
                "Failed to list files: HTTP {}",
                response.status()
            ));
        }
        
        // Parse the HF API response
        let tree_items: Vec<HfTreeItem> = response.json().await?;
        
        // Convert to HfFileInfo, filtering out directories
        let files: Vec<HfFileInfo> = tree_items
            .into_iter()
            .filter(|item| item.item_type == "file")
            .map(|item| {
                HfFileInfo {
                    path: item.path,
                    hash: item.oid.clone(),  // Git OID
                    size: item.size,
                    xet_hash: item.xet_hash,  // XET hash if available
                }
            })
            .collect();
        
        Ok(files)
    }

    pub async fn download_file_with_cancel(
        &self,
        repo_id: &str,
        filename: &str,
        repo_type: Option<&str>,
        revision: Option<&str>,
        local_dir: Option<&str>,
        progress_callback: Option<Arc<dyn Fn(&str, u64, u64) + Send + Sync>>,
        cancel_check: Option<Arc<dyn Fn() -> bool + Send + Sync>>,
    ) -> Result<String> {
        // Check for cancellation
        if let Some(ref cancel) = cancel_check {
            if cancel() {
                return Err(anyhow!("Download cancelled"));
            }
        }
        let repo_type = repo_type.unwrap_or("models");
        let revision = revision.unwrap_or("main");
        
        // First, get the file info to get the metadata
        let files = self.list_files(repo_id, Some(revision)).await?;
        let file_info = files
            .iter()
            .find(|f| f.path == filename)
            .ok_or_else(|| anyhow!("File {} not found in repository", filename))?
            .clone();
        
        // Determine destination path
        let dest_path = if let Some(local_dir) = local_dir {
            let mut path = PathBuf::from(local_dir);
            path.push(filename);
            path
        } else if let Some(cache_dir) = &self.cache_dir {
            let mut path = cache_dir.clone();
            path.push(repo_id.replace('/', "--"));
            path.push(revision);
            path.push(filename);
            path
        } else {
            PathBuf::from(filename)
        };

        // Create parent directory if needed
        if let Some(parent) = dest_path.parent() {
            fs::create_dir_all(parent).await?;
        }

        // Check if file already exists in cache (simple caching based on size)
        if dest_path.exists() {
            if let Ok(metadata) = fs::metadata(&dest_path).await {
                if metadata.len() == file_info.size {
                    // File already cached - report it
                    eprintln!("  [CACHE HIT] {} ({}MB)", filename, file_info.size / 1_048_576);
                    if let Some(cb) = progress_callback {
                        cb(filename, file_info.size, file_info.size);
                    }
                    return Ok(dest_path.to_string_lossy().to_string());
                } else {
                    // Size mismatch, re-download
                    eprintln!("  [CACHE MISS] {} - size mismatch (cached: {}, expected: {})", 
                        filename, metadata.len(), file_info.size);
                }
            }
        }

        // Construct the HF download URL
        let download_url = format!(
            "{}/{}/resolve/{}/{}",
            self.endpoint,
            repo_id,
            revision,
            filename
        );

        // Make a HEAD request without following redirects to capture XET headers
        // XET headers are only present on the initial HF response, not after CDN redirect
        let no_redirect_client = reqwest::Client::builder()
            .redirect(reqwest::redirect::Policy::none())  // Don't follow redirects
            .build()?;
            
        let head_response = no_redirect_client.head(&download_url)
            .header(reqwest::header::AUTHORIZATION, 
                    self.token.as_ref().map(|t| format!("Bearer {}", t)).unwrap_or_default())
            .send().await?;
        
        // Check for both success (200) and redirect (302) responses
        // HuggingFace returns 302 with XET headers
        let xet_file_data = if head_response.status().is_success() || head_response.status().is_redirection() {
            let headers = head_response.headers();
            parse_xet_file_data_from_headers(headers)
        } else {
            None
        };
        
        // Try XET download if available and enabled
        if let Some(xet_data) = xet_file_data {
            if self.enable_dedup {
                eprintln!("  [XET] File has XET support - hash: {}", xet_data.file_hash);
                eprintln!("  [XET] Refresh route: {}", xet_data.refresh_route);
                
                // Try to download using XET
                match self.download_with_xet(
                    &xet_data,
                    &dest_path,
                    file_info.size,
                    progress_callback.clone(),
                    cancel_check.clone(),
                ).await {
                    Ok(path) => {
                        return Ok(path);
                    }
                    Err(_) => {
                        // Fall back to regular HTTP download
                    }
                }
            } else {
                eprintln!("  [XET] File has XET support but dedup is disabled");
            }
        } else {
            eprintln!("  [XET] No XET metadata found for file");
        }

        // Create progress updater if callback provided (commented out - needs xet-core traits)
        // let progress_updater = progress_callback.as_ref().map(|cb| {
        //     Arc::new(HfProgressUpdater {
        //         callback: cb.clone(),
        //         current_file: filename.to_string(),
        //         total_size: file_info.size,
        //     }) as Arc<dyn TrackingProgressUpdater>
        // });

        // Regular HTTP download (fallback or primary)
        let response = self.client.get(&download_url).send().await?;
        
        if !response.status().is_success() {
            return Err(anyhow!(
                "Failed to download file: HTTP {}",
                response.status()
            ));
        }
        
        // Check for cancellation before downloading content
        if let Some(ref cancel) = cancel_check {
            if cancel() {
                return Err(anyhow!("Download cancelled"));
            }
        }
        
        // Get content length for progress reporting
        let total_size = response
            .headers()
            .get(reqwest::header::CONTENT_LENGTH)
            .and_then(|v| v.to_str().ok())
            .and_then(|v| v.parse::<u64>().ok())
            .unwrap_or(file_info.size);
        
        // Download with progress reporting
        let mut downloaded = 0u64;
        let content = response.bytes().await?;
        downloaded = content.len() as u64;
        
        // Report progress (commented out until we have xet-core traits)
        // if let Some(updater) = progress_updater {
        //     let update = ProgressUpdate {
        //         item_updates: vec![ItemProgressUpdate {
        //             item_name: Arc::from(filename),
        //             total_bytes: total_size,
        //             bytes_completed: downloaded,
        //             bytes_completion_increment: downloaded,
        //         }],
        //         total_bytes: total_size,
        //         total_bytes_increment: 0,
        //         total_bytes_completed: downloaded,
        //         total_bytes_completion_increment: downloaded,
        //         total_bytes_completion_rate: None,
        //         total_transfer_bytes: total_size,
        //         total_transfer_bytes_increment: 0,
        //         total_transfer_bytes_completed: downloaded,
        //         total_transfer_bytes_completion_increment: downloaded,
        //         total_transfer_bytes_completion_rate: None,
        //     };
        //     updater.register_updates(update).await;
        //     updater.flush().await;
        // }
        
        // Report via direct callback
        if let Some(cb) = progress_callback {
            cb(filename, downloaded, total_size);
        }
        
        // Write to destination
        fs::write(&dest_path, &content).await?;
        
        Ok(dest_path.to_string_lossy().to_string())
    }

    // Convenience method without cancellation
    pub async fn download_file(
        &self,
        repo_id: &str,
        filename: &str,
        repo_type: Option<&str>,
        revision: Option<&str>,
        local_dir: Option<&str>,
        progress_callback: Option<Arc<dyn Fn(&str, u64, u64) + Send + Sync>>,
    ) -> Result<String> {
        self.download_file_with_cancel(
            repo_id,
            filename,
            repo_type,
            revision,
            local_dir,
            progress_callback,
            None,
        ).await
    }

    pub async fn download_snapshot(
        &self,
        repo_id: &str,
        repo_type: Option<&str>,
        revision: Option<&str>,
        local_dir: &str,
        allow_patterns: Option<Vec<String>>,
        ignore_patterns: Option<Vec<String>>,
        progress_callback: Option<Arc<dyn Fn(&str, u64, u64) + Send + Sync>>,
    ) -> Result<String> {
        // List all files in the repository
        let files = self.list_files(repo_id, revision).await?;
        
        // Apply pattern filtering
        let filtered_files: Vec<_> = files.into_iter()
            .filter(|f| {
                // Simple pattern matching - in production would use glob patterns
                if let Some(ref allow) = allow_patterns {
                    if !allow.iter().any(|p| f.path.contains(p)) {
                        return false;
                    }
                }
                if let Some(ref ignore) = ignore_patterns {
                    if ignore.iter().any(|p| f.path.contains(p)) {
                        return false;
                    }
                }
                true
            })
            .collect();

        // Create local directory if needed
        fs::create_dir_all(local_dir).await?;

        // Download files in parallel with controlled concurrency
        let max_concurrent = self.max_concurrent.min(filtered_files.len());
        let semaphore = Arc::new(tokio::sync::Semaphore::new(max_concurrent));
        
        let download_futures = filtered_files.iter().map(|file| {
            let semaphore = semaphore.clone();
            let repo_id = repo_id.to_string();
            let file_path = file.path.clone();
            let repo_type = repo_type.map(|s| s.to_string());
            let revision = revision.map(|s| s.to_string());
            let local_dir = local_dir.to_string();
            let progress_callback = progress_callback.clone();
            let adapter = self.clone();
            
            async move {
                let _permit = semaphore.acquire().await?;
                
                adapter.download_file(
                    &repo_id,
                    &file_path,
                    repo_type.as_deref(),
                    revision.as_deref(),
                    Some(&local_dir),
                    progress_callback,
                ).await
            }
        });

        // Execute all downloads and collect results
        let results: Vec<Result<String>> = stream::iter(download_futures)
            .buffer_unordered(max_concurrent)
            .collect()
            .await;

        // Check for errors
        for result in results {
            result?;
        }

        Ok(local_dir.to_string())
    }
    
    /// Download a file using XET/CAS
    async fn download_with_xet(
        &self,
        xet_file_data: &XetFileData,
        dest_path: &Path,
        expected_size: u64,
        progress_callback: Option<Arc<dyn Fn(&str, u64, u64) + Send + Sync>>,
        cancel_check: Option<Arc<dyn Fn() -> bool + Send + Sync>>,
    ) -> Result<String> {
        use crate::xet_downloader::XetDownloader;
        
        // Get XET connection info
        let mut token_manager = self.xet_token_manager.lock().await;
        let connection_info = token_manager.refresh_xet_connection_info(xet_file_data).await?;
        drop(token_manager);  // Release the lock early
        
        eprintln!("  [XET] Using xet-core FileDownloader for hash: {}", xet_file_data.file_hash);
        eprintln!("  [XET] Endpoint: {}", connection_info.endpoint);
        
        // Create XET downloader with connection info
        let xet_downloader = XetDownloader::new(&connection_info, self.token.clone()).await?;
        
        // Check for cancellation before starting
        if let Some(ref cancel) = cancel_check {
            if cancel() {
                return Err(anyhow!("Download cancelled"));
            }
        }
        
        // Download using xet-core's FileDownloader
        let bytes_downloaded = xet_downloader.download_file(
            &xet_file_data.file_hash,
            dest_path,
            progress_callback,
        ).await?;
        
        // Size verification is not critical - CAS may return slightly different sizes
        
        Ok(dest_path.to_string_lossy().to_string())
    }
}