use anyhow::{anyhow, Context, Result};
use reqwest::header::{HeaderMap, HeaderValue};
use std::time::{SystemTime, UNIX_EPOCH};

/// XET file metadata extracted from HuggingFace API responses
#[derive(Debug, Clone)]
pub struct XetFileData {
    pub file_hash: String,
    pub refresh_route: String,
}

/// Connection information for the XET CAS server
#[derive(Debug, Clone)]
pub struct XetConnectionInfo {
    pub endpoint: String,
    pub access_token: String,
    pub expiration_unix_epoch: u64,
}

/// Token type for XET operations
#[derive(Debug, Clone, Copy)]
#[allow(dead_code)]
pub enum XetTokenType {
    Read,
    Write,
}

#[allow(dead_code)]
impl std::fmt::Display for XetTokenType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            XetTokenType::Read => write!(f, "read"),
            XetTokenType::Write => write!(f, "write"),
        }
    }
}

/// Parse XET file metadata from HTTP response headers
pub fn parse_xet_file_data_from_headers(headers: &HeaderMap) -> Option<XetFileData> {
    // Check for X-Xet-Hash header
    let file_hash = headers
        .get("x-xet-hash")
        .and_then(|v| v.to_str().ok())
        .map(|s| s.to_string())?;

    // Check for Link header with xet-auth relation
    let refresh_route = if let Some(link_header) = headers.get("link") {
        if let Ok(link_str) = link_header.to_str() {
            // Parse Link header to find xet-auth relation
            parse_link_header_for_xet_auth(link_str)
        } else {
            // Fallback to X-Xet-Refresh-Route header
            headers
                .get("x-xet-refresh-route")
                .and_then(|v| v.to_str().ok())
                .map(|s| s.to_string())
        }
    } else {
        headers
            .get("x-xet-refresh-route")
            .and_then(|v| v.to_str().ok())
            .map(|s| s.to_string())
    }?;

    Some(XetFileData {
        file_hash,
        refresh_route,
    })
}

/// Parse Link header to extract xet-auth URL
fn parse_link_header_for_xet_auth(link_str: &str) -> Option<String> {
    // Link header format: <url>; rel="relation", <url2>; rel="relation2"
    for link_part in link_str.split(',') {
        let link_part = link_part.trim();
        if link_part.contains("rel=\"xet-auth\"") || link_part.contains("rel='xet-auth'") {
            // Extract URL from <url> format
            if let Some(url_start) = link_part.find('<') {
                if let Some(url_end) = link_part.find('>') {
                    return Some(link_part[url_start + 1..url_end].to_string());
                }
            }
        }
    }
    None
}

/// Parse XET connection info from HTTP response headers
pub fn parse_xet_connection_info_from_headers(headers: &HeaderMap) -> Option<XetConnectionInfo> {
    let endpoint = headers
        .get("x-xet-cas-url")
        .and_then(|v| v.to_str().ok())
        .map(|s| s.to_string())?;

    let access_token = headers
        .get("x-xet-access-token")
        .and_then(|v| v.to_str().ok())
        .map(|s| s.to_string())?;

    let expiration_unix_epoch = headers
        .get("x-xet-token-expiration")
        .and_then(|v| v.to_str().ok())
        .and_then(|s| s.parse::<u64>().ok())?;

    Some(XetConnectionInfo {
        endpoint,
        access_token,
        expiration_unix_epoch,
    })
}

/// XET token manager with automatic refresh
pub struct XetTokenManager {
    client: reqwest::Client,
    #[allow(dead_code)]
    hf_token: Option<String>,
    cached_connection_info: Option<(XetConnectionInfo, String)>, // (info, refresh_route)
}

impl XetTokenManager {
    pub fn new(hf_token: Option<String>) -> Self {
        let mut headers = HeaderMap::new();
        if let Some(ref token) = hf_token {
            if let Ok(header_value) = HeaderValue::from_str(&format!("Bearer {}", token)) {
                headers.insert(reqwest::header::AUTHORIZATION, header_value);
            }
        }

        let client = reqwest::Client::builder()
            .default_headers(headers)
            .build()
            .unwrap_or_default();

        Self {
            client,
            hf_token,
            cached_connection_info: None,
        }
    }

    /// Check if the cached token is still valid
    fn is_token_valid(&self) -> bool {
        if let Some((ref info, _)) = self.cached_connection_info {
            let now = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map(|d| d.as_secs())
                .unwrap_or(0);

            // Consider token valid if it has at least 60 seconds remaining
            info.expiration_unix_epoch > now + 60
        } else {
            false
        }
    }

    /// Refresh XET connection info using the refresh route
    pub async fn refresh_xet_connection_info(
        &mut self,
        file_data: &XetFileData,
    ) -> Result<XetConnectionInfo> {
        // Check if we have a valid cached token for this route
        if self.is_token_valid() {
            if let Some((ref info, ref cached_route)) = self.cached_connection_info {
                if cached_route == &file_data.refresh_route {
                    return Ok(info.clone());
                }
            }
        }

        let response = self
            .client
            .get(&file_data.refresh_route)
            .send()
            .await
            .context("Failed to fetch XET connection info")?;

        if !response.status().is_success() {
            return Err(anyhow!(
                "Failed to get XET token: HTTP {}",
                response.status()
            ));
        }

        let headers = response.headers();
        let connection_info = parse_xet_connection_info_from_headers(headers)
            .ok_or_else(|| anyhow!("XET headers not found in response"))?;

        // Cache the connection info
        self.cached_connection_info =
            Some((connection_info.clone(), file_data.refresh_route.clone()));

        Ok(connection_info)
    }

    /// Fetch XET connection info directly from repo info
    #[allow(dead_code)]
    pub async fn fetch_xet_connection_info_from_repo(
        &mut self,
        token_type: XetTokenType,
        repo_id: &str,
        repo_type: &str,
        revision: Option<&str>,
        endpoint: &str,
    ) -> Result<XetConnectionInfo> {
        let revision = revision.unwrap_or("main");
        let url = format!(
            "{}/api/{}s/{}/xet-{}-token/{}",
            endpoint, repo_type, repo_id, token_type, revision
        );

        let response = self
            .client
            .get(&url)
            .send()
            .await
            .context("Failed to fetch XET token")?;

        if !response.status().is_success() {
            return Err(anyhow!(
                "Failed to get XET token: HTTP {}",
                response.status()
            ));
        }

        let headers = response.headers();
        let connection_info = parse_xet_connection_info_from_headers(headers)
            .ok_or_else(|| anyhow!("XET headers not found in response"))?;

        Ok(connection_info)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_link_header() {
        let link = r#"<https://huggingface.co/api/models/test/xet-read-token/abc123>; rel="xet-auth", <https://cas-server.xethub.hf.co/reconstruction/hash>; rel="xet-reconstruction-info""#;

        let result = parse_link_header_for_xet_auth(link);
        assert_eq!(
            result,
            Some("https://huggingface.co/api/models/test/xet-read-token/abc123".to_string())
        );
    }

    #[test]
    fn test_parse_xet_connection_info() {
        let mut headers = HeaderMap::new();
        headers.insert(
            "x-xet-cas-url",
            HeaderValue::from_static("https://cas-server.xethub.hf.co"),
        );
        headers.insert("x-xet-access-token", HeaderValue::from_static("test-token"));
        headers.insert(
            "x-xet-token-expiration",
            HeaderValue::from_static("1758055996"),
        );

        let result = parse_xet_connection_info_from_headers(&headers);
        assert!(result.is_some());

        let info = result.unwrap();
        assert_eq!(info.endpoint, "https://cas-server.xethub.hf.co");
        assert_eq!(info.access_token, "test-token");
        assert_eq!(info.expiration_unix_epoch, 1758055996);
    }
}
