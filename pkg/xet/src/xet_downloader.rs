// XET Core integration using FileDownloader for CAS operations
use std::sync::Arc;
use std::path::{Path, PathBuf};
use anyhow::{Result, Context};
use xet_core_data::{FileDownloader, XetFileInfo, data_client};
use xet_core_data::configurations::{TranslatorConfig, DataConfig, ShardConfig, RepoInfo, ProgressConfig, Endpoint};
use cas_client::{CacheConfig, FileProvider, OutputProvider, CHUNK_CACHE_SIZE_BYTES};
use cas_object::CompressionScheme;
use cas_client::remote_client::PREFIX_DEFAULT;
use progress_tracking::TrackingProgressUpdater;
use progress_tracking::item_tracking::ItemProgressUpdater;
use utils::auth::{AuthConfig, TokenRefresher};
use utils::normalized_path_from_user_string;
use xet_runtime::{XetRuntime, global_semaphore_handle, GlobalSemaphoreHandle};
use ulid::Ulid;
use dirs::home_dir;
use tracing::{info, debug};
use merklehash::MerkleHash;

use crate::xet_integration::{XetConnectionInfo, XetFileData};

/// XET Downloader that uses xet-core's FileDownloader for CAS operations
pub struct XetDownloader {
    config: Arc<TranslatorConfig>,
    downloader: Arc<FileDownloader>,
}

impl XetDownloader {
    /// Create a new XET downloader with connection info from HuggingFace
    pub async fn new(
        connection_info: &XetConnectionInfo,
        _hf_token: Option<String>,
    ) -> Result<Self> {
        let config = create_xet_config(
            connection_info.endpoint.clone(),
            Some((connection_info.access_token.clone(), connection_info.expiration_unix_epoch)),
            None, // No token refresher for now
        )?;
        
        let config = Arc::new(config);
        let downloader = Arc::new(FileDownloader::new(config.clone()).await?);
        
        Ok(Self {
            config,
            downloader,
        })
    }

    /// Download a file from XET CAS using its hash
    pub async fn download_file(
        &self,
        file_hash: &str,
        destination_path: &Path,
        progress_callback: Option<Arc<dyn Fn(&str, u64, u64) + Send + Sync>>,
    ) -> Result<u64> {
        // Parse the hash string to MerkleHash
        // Try hex first (HuggingFace format), then base64 as fallback
        let hash = MerkleHash::from_hex(file_hash)
            .or_else(|_| MerkleHash::from_base64(file_hash))
            .context("Failed to parse file hash")?;
        
        // Create parent directory if needed
        if let Some(parent) = destination_path.parent() {
            std::fs::create_dir_all(parent)?;
        }
        
        // Create output provider for the file
        let output = OutputProvider::File(FileProvider::new(destination_path.to_path_buf()));
        
        // Create progress updater if callback provided
        let progress_updater = progress_callback.map(|callback| {
            Arc::new(XetProgressUpdater::new(callback)) as Arc<dyn TrackingProgressUpdater>
        });
        
        let file_name = destination_path.file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("file");
        
        let progress_tracker = progress_updater.map(ItemProgressUpdater::new);
        
        // Use FileDownloader to get the file from CAS
        let bytes_downloaded = self.downloader.smudge_file_from_hash(
            &hash,
            Arc::from(file_name),
            &output,
            None, // No range
            progress_tracker,
        ).await?;
        
        info!("Downloaded {} bytes from XET CAS to {:?}", bytes_downloaded, destination_path);
        
        Ok(bytes_downloaded)
    }
}

/// Create XET configuration compatible with xet-core
fn create_xet_config(
    endpoint: String,
    token_info: Option<(String, u64)>,
    token_refresher: Option<Arc<dyn TokenRefresher>>,
) -> Result<TranslatorConfig> {
    // Use same cache path logic as hf_xet
    let cache_root_path = {
        if let Ok(cache) = std::env::var("HF_XET_CACHE") {
            normalized_path_from_user_string(cache)
        } else if let Ok(hf_home) = std::env::var("HF_HOME") {
            normalized_path_from_user_string(hf_home).join("xet")
        } else if let Ok(xdg_cache_home) = std::env::var("XDG_CACHE_HOME") {
            normalized_path_from_user_string(xdg_cache_home).join("huggingface").join("xet")
        } else {
            home_dir()
                .unwrap_or_else(|| std::env::current_dir().unwrap())
                .join(".cache")
                .join("huggingface")
                .join("xet")
        }
    };

    info!("Using XET cache path: {:?}", cache_root_path);

    let (token, token_expiration) = token_info.unzip();
    let auth_cfg = AuthConfig::maybe_new(token, token_expiration, token_refresher);

    // Create endpoint tag for cache separation
    let endpoint_tag = {
        let endpoint_prefix = endpoint
            .chars()
            .take(16)
            .map(|c| if c.is_alphanumeric() { c } else { '_' })
            .collect::<String>();

        let endpoint_hash = merklehash::compute_data_hash(endpoint.as_bytes()).base64();
        format!("{endpoint_prefix}-{}", &endpoint_hash[..16])
    };

    let cache_path = cache_root_path.join(endpoint_tag);
    std::fs::create_dir_all(&cache_path)?;

    let staging_root = cache_path.join("staging");
    std::fs::create_dir_all(&staging_root)?;

    Ok(TranslatorConfig {
        data_config: DataConfig {
            endpoint: Endpoint::Server(endpoint),
            compression: None,
            auth: auth_cfg,
            prefix: PREFIX_DEFAULT.into(),
            cache_config: CacheConfig {
                cache_directory: cache_path.join("chunk-cache"),
                cache_size: *CHUNK_CACHE_SIZE_BYTES,
            },
            staging_directory: None,
        },
        shard_config: ShardConfig {
            prefix: PREFIX_DEFAULT.into(),
            cache_directory: cache_path.join("shard-cache"),
            session_directory: staging_root.join("shard-session"),
            global_dedup_policy: Default::default(),
        },
        repo_info: Some(RepoInfo {
            repo_paths: vec!["".into()],
        }),
        session_id: Some(Ulid::new().to_string()),
        progress_config: ProgressConfig { aggregate: true },
    })
}

/// Progress updater wrapper for xet-core
struct XetProgressUpdater {
    callback: Arc<dyn Fn(&str, u64, u64) + Send + Sync>,
}

impl XetProgressUpdater {
    fn new(callback: Arc<dyn Fn(&str, u64, u64) + Send + Sync>) -> Self {
        Self { callback }
    }
}

#[async_trait::async_trait]
impl TrackingProgressUpdater for XetProgressUpdater {
    async fn register_updates(&self, updates: progress_tracking::ProgressUpdate) {
        // Convert item progress to our callback format
        for item in &updates.item_updates {
            (self.callback)(
                &item.item_name,
                item.bytes_completed,
                item.total_bytes,
            );
        }
    }
    
    async fn flush(&self) {
        // Nothing to do on flush
    }
}