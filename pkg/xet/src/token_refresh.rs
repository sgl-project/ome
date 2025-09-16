// Token refresh module - similar to hf_xet/token_refresh.rs
use std::sync::Arc;
use async_trait::async_trait;
use utils::auth::{TokenRefresher, TokenInfo};
use utils::errors::AuthError;
use anyhow::Result;

/// Token refresher implementation for XET
pub struct XetTokenRefresher {
    refresh_fn: Arc<dyn Fn() -> Result<(String, u64)> + Send + Sync>,
}

impl XetTokenRefresher {
    pub fn new(refresh_fn: Arc<dyn Fn() -> Result<(String, u64)> + Send + Sync>) -> Self {
        Self { refresh_fn }
    }
}

#[async_trait]
impl TokenRefresher for XetTokenRefresher {
    async fn refresh(&self) -> Result<TokenInfo, AuthError> {
        match (self.refresh_fn)() {
            Ok((token, expiry)) => Ok((token, expiry)), // TokenInfo is a type alias for (String, u64)
            Err(e) => Err(AuthError::TokenRefreshFailure(e.to_string())),
        }
    }
}