use crate::progress::{OperationProgress, XetProgressPhase};
use crate::xet_integration::{parse_xet_file_data_from_headers, XetFileData, XetTokenManager};
use anyhow::{anyhow, Result};
use futures::stream::{self, StreamExt};
use serde::Deserialize;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::Duration;
use tokio::fs;
use tokio::io::AsyncWriteExt;
use tokio::time::sleep;
use tracing::{debug, info};

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
    pub xet_hash: Option<String>, // XET hash if available
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
}

const MAX_HTTP_RETRIES: usize = 3;
const RETRY_BACKOFF_MS: u64 = 200;

impl HfAdapter {
    async fn send_with_retry<F, S>(
        &self,
        mut builder: F,
        description: &str,
        is_success: S,
    ) -> Result<reqwest::Response>
    where
        F: FnMut() -> reqwest::RequestBuilder,
        S: Fn(&reqwest::Response) -> bool,
    {
        for attempt in 0..=MAX_HTTP_RETRIES {
            match builder().send().await {
                Ok(resp) => {
                    if is_success(&resp) {
                        return Ok(resp);
                    }

                    debug!(
                        "[RETRY] {} attempt {} failed with HTTP {}",
                        description,
                        attempt + 1,
                        resp.status()
                    );

                    if attempt == MAX_HTTP_RETRIES {
                        return Err(anyhow!(
                            "{} failed after {} attempts: HTTP {}",
                            description,
                            attempt + 1,
                            resp.status()
                        ));
                    }
                }
                Err(err) => {
                    debug!(
                        "[RETRY] {} attempt {} errored: {}",
                        description,
                        attempt + 1,
                        err
                    );

                    if attempt == MAX_HTTP_RETRIES {
                        return Err(anyhow!(
                            "{} failed after {} attempts: {}",
                            description,
                            attempt + 1,
                            err
                        ));
                    }
                }
            }

            sleep(Duration::from_millis(
                RETRY_BACKOFF_MS * (attempt as u64 + 1),
            ))
            .await;
        }

        unreachable!("retry loop should always return or err");
    }

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

        let xet_token_manager =
            Arc::new(tokio::sync::Mutex::new(XetTokenManager::new(token.clone())));

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
        let response = self
            .send_with_retry(
                || self.client.get(&url),
                "list files",
                |resp| resp.status().is_success(),
            )
            .await?;

        // Parse the HF API response
        let tree_items: Vec<HfTreeItem> = response.json().await?;

        // Convert to HfFileInfo, filtering out directories
        let files: Vec<HfFileInfo> = tree_items
            .into_iter()
            .filter(|item| item.item_type == "file")
            .map(|item| {
                HfFileInfo {
                    path: item.path,
                    hash: item.oid.clone(), // Git OID
                    size: item.size,
                    xet_hash: item.xet_hash, // XET hash if available
                }
            })
            .collect();

        Ok(files)
    }

    #[allow(clippy::too_many_arguments)]
    pub async fn download_file_with_cancel(
        &self,
        repo_id: &str,
        filename: &str,
        repo_type: Option<&str>,
        revision: Option<&str>,
        local_dir: Option<&str>,
        cancel_check: Option<Arc<dyn Fn() -> bool + Send + Sync>>,
        progress: Option<OperationProgress>,
    ) -> Result<String> {
        let revision = revision.unwrap_or("main");

        if let Some(ref tracker) = progress {
            tracker.set_phase(XetProgressPhase::Scanning, true);
        }

        // First, get the file info to determine metadata
        let files = self.list_files(repo_id, Some(revision)).await?;
        let file_info = files
            .iter()
            .find(|f| f.path == filename)
            .cloned()
            .ok_or_else(|| anyhow!("File {} not found in repository", filename))?;

        if let Some(ref tracker) = progress {
            tracker.set_total_hint(1, file_info.size);
            tracker.set_phase(XetProgressPhase::Downloading, true);
            tracker.ensure_file_entry(&file_info.path, file_info.size);
            tracker.update_file_absolute(&file_info.path, 0, file_info.size, true);
        }

        let output = self
            .download_file_with_info(
                repo_id,
                repo_type,
                revision,
                local_dir,
                &file_info,
                cancel_check,
                progress.as_ref().map(|p| p.clone_for_tasks()),
            )
            .await?;

        if let Some(tracker) = progress {
            tracker.finalize();
        }

        Ok(output)
    }

    #[allow(clippy::too_many_arguments)]
    pub async fn download_snapshot(
        &self,
        repo_id: &str,
        repo_type: Option<&str>,
        revision: Option<&str>,
        local_dir: &str,
        allow_patterns: Option<Vec<String>>,
        ignore_patterns: Option<Vec<String>>,
        cancel_check: Option<Arc<dyn Fn() -> bool + Send + Sync>>,
        progress: Option<OperationProgress>,
    ) -> Result<String> {
        let revision = revision.unwrap_or("main");
        if let Some(ref tracker) = progress {
            tracker.set_phase(XetProgressPhase::Scanning, true);
        }

        // List all files in the repository
        let files = self.list_files(repo_id, Some(revision)).await?;

        // Apply pattern filtering
        let filtered_files: Vec<_> = files
            .into_iter()
            .filter(|f| {
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

        let total_bytes: u64 = filtered_files.iter().map(|f| f.size).sum();
        if let Some(ref tracker) = progress {
            tracker.set_total_hint(filtered_files.len(), total_bytes);
            tracker.set_phase(XetProgressPhase::Downloading, true);
        }

        // Create local directory if needed
        fs::create_dir_all(local_dir).await?;

        // Download files in parallel with controlled concurrency
        let max_concurrent = self.max_concurrent.max(1).min(filtered_files.len().max(1));
        let semaphore = Arc::new(tokio::sync::Semaphore::new(max_concurrent));
        let cancel_check = cancel_check.map(|c| c as Arc<_>);
        let progress_shared = progress.as_ref().map(|p| p.clone_for_tasks());

        let download_futures = filtered_files.into_iter().map(|file| {
            let semaphore = semaphore.clone();
            let adapter = self.clone();
            let repo_id = repo_id.to_string();
            let repo_type = repo_type.map(|s| s.to_string());
            let revision = revision.to_string();
            let local_dir = local_dir.to_string();
            let cancel_check = cancel_check.clone();
            let progress = progress_shared.clone();

            async move {
                let _permit = semaphore.acquire().await?;

                if is_cancelled(&cancel_check) {
                    return Err(anyhow!("Download cancelled"));
                }

                adapter
                    .download_file_with_info(
                        &repo_id,
                        repo_type.as_deref(),
                        &revision,
                        Some(&local_dir),
                        &file,
                        cancel_check.clone(),
                        progress.as_ref().map(|p| p.clone_for_tasks()),
                    )
                    .await
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

        if let Some(tracker) = progress {
            tracker.finalize();
        }

        Ok(local_dir.to_string())
    }

    #[allow(clippy::too_many_arguments)]
    async fn download_file_with_info(
        &self,
        repo_id: &str,
        _repo_type: Option<&str>,
        revision: &str,
        local_dir: Option<&str>,
        file_info: &HfFileInfo,
        cancel_check: Option<Arc<dyn Fn() -> bool + Send + Sync>>,
        progress: Option<OperationProgress>,
    ) -> Result<String> {
        if is_cancelled(&cancel_check) {
            return Err(anyhow!("Download cancelled"));
        }

        let destination = determine_destination(
            local_dir,
            self.cache_dir.as_deref(),
            repo_id,
            revision,
            &file_info.path,
        );

        if let Some(parent) = destination.parent() {
            fs::create_dir_all(parent).await?;
        }

        // Check cache hit
        if destination.exists() {
            if let Ok(metadata) = fs::metadata(&destination).await {
                if metadata.len() == file_info.size {
                    debug!("[CACHE HIT] {} ({} bytes)", file_info.path, file_info.size);
                    if let Some(ref tracker) = progress {
                        tracker.ensure_file_entry(&file_info.path, file_info.size);
                        tracker.update_file_absolute(
                            &file_info.path,
                            file_info.size,
                            file_info.size,
                            true,
                        );
                    }
                    return Ok(destination.to_string_lossy().to_string());
                } else {
                    debug!(
                        "[CACHE MISS] {} - size mismatch (cached: {}, expected: {})",
                        file_info.path,
                        metadata.len(),
                        file_info.size
                    );
                }
            }
        }

        // Construct the HF download URL
        let download_url = format!(
            "{}/{}/resolve/{}/{}",
            self.endpoint, repo_id, revision, file_info.path
        );

        // Make a HEAD request without following redirects to capture XET headers
        let no_redirect_client = reqwest::Client::builder()
            .redirect(reqwest::redirect::Policy::none())
            .build()?;

        let auth_header = self.token.as_ref().map(|t| format!("Bearer {}", t));

        let head_response = self
            .send_with_retry(
                || {
                    let mut builder = no_redirect_client.head(&download_url);
                    if let Some(ref auth) = auth_header {
                        builder = builder.header(reqwest::header::AUTHORIZATION, auth.clone());
                    }
                    builder
                },
                "head request",
                |resp| resp.status().is_success() || resp.status().is_redirection(),
            )
            .await?;

        let xet_file_data = parse_xet_file_data_from_headers(head_response.headers());

        // Try XET download if available and enabled
        if let Some(xet_data) = xet_file_data {
            if self.enable_dedup {
                info!("[XET] File has XET support - hash: {}", xet_data.file_hash);
                debug!("[XET] Refresh route: {}", xet_data.refresh_route);

                match self
                    .download_with_xet(
                        &file_info.path,
                        &xet_data,
                        &destination,
                        file_info.size,
                        cancel_check.clone(),
                        progress.as_ref().map(|p| p.clone_for_tasks()),
                    )
                    .await
                {
                    Ok(path) => return Ok(path),
                    Err(err) => {
                        debug!("[XET] Falling back to HTTP download: {err:?}");
                    }
                }
            } else {
                debug!("[XET] File has XET support but dedup is disabled");
            }
        } else {
            debug!("[XET] No XET metadata found for file");
        }

        if is_cancelled(&cancel_check) {
            return Err(anyhow!("Download cancelled"));
        }

        // Regular HTTP download (fallback or primary)
        let response = self
            .send_with_retry(
                || self.client.get(&download_url),
                "download request",
                |resp| resp.status().is_success(),
            )
            .await?;

        let expected_total = response.content_length().unwrap_or(file_info.size);
        if let Some(ref tracker) = progress {
            tracker.ensure_file_entry(&file_info.path, expected_total);
        }

        let mut stream = response.bytes_stream();
        let mut file = fs::File::create(&destination).await?;
        let mut downloaded: u64 = 0;

        while let Some(chunk) = stream.next().await {
            let chunk = chunk?;
            downloaded += chunk.len() as u64;

            if is_cancelled(&cancel_check) {
                return Err(anyhow!("Download cancelled"));
            }

            file.write_all(&chunk).await?;
            if let Some(ref tracker) = progress {
                tracker.update_file_absolute(&file_info.path, downloaded, expected_total, false);
            }
        }

        file.flush().await?;

        if let Some(ref tracker) = progress {
            tracker.update_file_absolute(&file_info.path, downloaded, expected_total, true);
        }

        Ok(destination.to_string_lossy().to_string())
    }

    async fn download_with_xet(
        &self,
        file_name: &str,
        xet_file_data: &XetFileData,
        dest_path: &Path,
        expected_size: u64,
        cancel_check: Option<Arc<dyn Fn() -> bool + Send + Sync>>,
        progress: Option<OperationProgress>,
    ) -> Result<String> {
        use crate::xet_downloader::XetDownloader;

        if is_cancelled(&cancel_check) {
            return Err(anyhow!("Download cancelled"));
        }

        // Get XET connection info
        let mut token_manager = self.xet_token_manager.lock().await;
        let connection_info = token_manager
            .refresh_xet_connection_info(xet_file_data)
            .await?;
        drop(token_manager); // Release the lock early

        info!(
            "[XET] Using xet-core FileDownloader for hash: {}",
            xet_file_data.file_hash
        );
        debug!("[XET] Endpoint: {}", connection_info.endpoint);

        // Create XET downloader with connection info
        let xet_downloader = XetDownloader::new(&connection_info, self.token.clone()).await?;

        if let Some(ref tracker) = progress {
            tracker.ensure_file_entry(file_name, expected_size);
        }

        if is_cancelled(&cancel_check) {
            return Err(anyhow!("Download cancelled"));
        }

        // Download using xet-core's FileDownloader
        let _bytes_downloaded = xet_downloader
            .download_file(
                &xet_file_data.file_hash,
                dest_path,
                file_name,
                expected_size,
                progress.as_ref().map(|p| p.clone_for_tasks()),
            )
            .await?;

        if let Some(ref tracker) = progress {
            tracker.update_file_absolute(file_name, expected_size, expected_size, true);
        }

        Ok(dest_path.to_string_lossy().to_string())
    }
}

fn determine_destination(
    local_dir: Option<&str>,
    cache_dir: Option<&Path>,
    repo_id: &str,
    revision: &str,
    filename: &str,
) -> PathBuf {
    if let Some(local_dir) = local_dir {
        let mut path = PathBuf::from(local_dir);
        path.push(filename);
        return path;
    }

    if let Some(cache_dir) = cache_dir {
        let mut path = cache_dir.to_path_buf();
        path.push(repo_id.replace('/', "--"));
        path.push(revision);
        path.push(filename);
        return path;
    }

    PathBuf::from(filename)
}

fn is_cancelled(cancel_check: &Option<Arc<dyn Fn() -> bool + Send + Sync>>) -> bool {
    cancel_check
        .as_ref()
        .map(|cancel| cancel())
        .unwrap_or(false)
}