// XET Core integration using FileDownloader for CAS operations
use anyhow::{Context, Result};
use async_trait::async_trait;
use cas_client::remote_client::PREFIX_DEFAULT;
use cas_client::{CacheConfig, FileProvider, OutputProvider, CHUNK_CACHE_SIZE_BYTES};
use dirs::home_dir;
use merklehash::MerkleHash;
use progress_tracking::{
    item_tracking::ItemProgressUpdater, ProgressUpdate as TrackerProgressUpdate,
    TrackingProgressUpdater,
};
use std::path::Path;
use std::sync::Arc;
use tokio::sync::Mutex;
use tracing::{debug, info};
use ulid::Ulid;
use utils::auth::{AuthConfig, TokenRefresher};
use utils::errors::AuthError;
use utils::normalized_path_from_user_string;
use xet_core_data::configurations::{
    DataConfig, Endpoint, ProgressConfig, RepoInfo, ShardConfig, TranslatorConfig,
};
use xet_core_data::FileDownloader;

use crate::progress::OperationProgress;
use crate::xet_integration::{XetConnectionInfo, XetFileData, XetTokenManager};

/// Token refresher that uses XetTokenManager to refresh XET CAS tokens.
///
/// This implements the `TokenRefresher` trait required by xet-core's auth system.
/// When the CAS client detects that a token is about to expire, it calls the
/// `refresh()` method to obtain fresh credentials.
struct HfTokenRefresher {
    token_manager: Arc<Mutex<XetTokenManager>>,
    file_data: XetFileData,
}

impl HfTokenRefresher {
    fn new(token_manager: Arc<Mutex<XetTokenManager>>, file_data: XetFileData) -> Self {
        Self {
            token_manager,
            file_data,
        }
    }
}

#[async_trait]
impl TokenRefresher for HfTokenRefresher {
    /// Refresh the XET CAS token by calling the HuggingFace refresh route.
    ///
    /// Returns a tuple of (access_token, expiration_unix_epoch).
    async fn refresh(&self) -> Result<(String, u64), AuthError> {
        debug!(
            "[TokenRefresher] Refreshing XET token via route: {}",
            self.file_data.refresh_route
        );

        let mut manager = self.token_manager.lock().await;
        let connection_info = manager
            .refresh_xet_connection_info(&self.file_data)
            .await
            .map_err(|e| AuthError::TokenRefreshFailure(e.to_string()))?;

        debug!(
            "[TokenRefresher] Token refreshed, new expiration: {}",
            connection_info.expiration_unix_epoch
        );

        Ok((
            connection_info.access_token,
            connection_info.expiration_unix_epoch,
        ))
    }
}

/// XET Downloader that uses xet-core's FileDownloader for CAS operations
pub struct XetDownloader {
    #[allow(dead_code)]
    config: Arc<TranslatorConfig>,
    downloader: Arc<FileDownloader>,
}

impl XetDownloader {
    /// Create a new XET downloader with connection info from HuggingFace.
    ///
    /// # Arguments
    /// * `connection_info` - Initial XET CAS connection info (endpoint, token, expiration)
    /// * `file_data` - XET file metadata containing the refresh route for token renewal
    /// * `token_manager` - Shared token manager for refreshing tokens when they expire
    pub async fn new(
        connection_info: &XetConnectionInfo,
        file_data: &XetFileData,
        token_manager: Arc<Mutex<XetTokenManager>>,
    ) -> Result<Self> {
        // Create a token refresher that will be called by xet-core when the token expires
        let refresher: Arc<dyn TokenRefresher> =
            Arc::new(HfTokenRefresher::new(token_manager, file_data.clone()));

        let config = create_xet_config(
            connection_info.endpoint.clone(),
            Some((
                connection_info.access_token.clone(),
                connection_info.expiration_unix_epoch,
            )),
            Some(refresher),
        )?;

        let config = Arc::new(config);
        let downloader = Arc::new(FileDownloader::new(config.clone()).await?);

        Ok(Self { config, downloader })
    }

    /// Download a file from XET CAS using its hash
    pub async fn download_file(
        &self,
        file_hash: &str,
        destination_path: &Path,
        file_name: &str,
        expected_size: u64,
        progress: Option<OperationProgress>,
    ) -> Result<u64> {
        // Parse the hash string to MerkleHash
        let hash = MerkleHash::from_hex(file_hash)
            .or_else(|_| MerkleHash::from_base64(file_hash))
            .context("Failed to parse file hash")?;

        if let Some(parent) = destination_path.parent() {
            std::fs::create_dir_all(parent)?;
        }

        let output = OutputProvider::File(FileProvider::new(destination_path.to_path_buf()));
        let file_name_arc: Arc<str> = Arc::from(file_name.to_owned());

        let progress_updater = progress.as_ref().map(|tracker| {
            let bridge = Arc::new(ProgressBridge::new(tracker.clone_for_tasks()));
            ItemProgressUpdater::new(bridge)
        });

        if let Some(ref tracker) = progress {
            tracker.ensure_file_entry(file_name, expected_size);
        }

        let bytes_downloaded = self
            .downloader
            .smudge_file_from_hash(&hash, file_name_arc, &output, None, progress_updater)
            .await?;

        info!(
            "Downloaded {} bytes from XET CAS to {:?}",
            bytes_downloaded, destination_path
        );

        Ok(bytes_downloaded)
    }
}

struct ProgressBridge {
    progress: OperationProgress,
}

impl ProgressBridge {
    fn new(progress: OperationProgress) -> Self {
        Self { progress }
    }
}

#[async_trait]
impl TrackingProgressUpdater for ProgressBridge {
    async fn register_updates(&self, updates: TrackerProgressUpdate) {
        self.progress.apply_tracking_update(&updates);
    }

    async fn flush(&self) {
        self.progress.force_emit();
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
            normalized_path_from_user_string(xdg_cache_home)
                .join("huggingface")
                .join("xet")
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
