// Logging module for XET bindings
use std::env;
use std::sync::Once;
use tracing::debug;
use tracing_subscriber::filter::EnvFilter;
use tracing_subscriber::layer::SubscriberExt;
use tracing_subscriber::util::SubscriberInitExt;
use tracing_subscriber::Layer;

static INIT: Once = Once::new();

/// Initialize logging for the XET binding library
pub fn init_logging() {
    INIT.call_once(|| {
        // If RUST_LOG is not set, use our default
        if env::var("RUST_LOG").is_err() {
            // Check for XET_LOG_LEVEL first, otherwise use default
            let log_level = env::var("XET_LOG_LEVEL").unwrap_or_else(|_| "warn".to_string());
            env::set_var("RUST_LOG", &log_level);
        }

        // Now create filter from environment (will use RUST_LOG we just set)
        let filter = EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("warn"));

        // For simplicity, we'll just use the human-readable format
        // JSON format would require additional dependencies
        let fmt_layer = tracing_subscriber::fmt::layer()
            .with_target(false)
            .with_filter(filter);

        tracing_subscriber::registry().with(fmt_layer).init();

        debug!("XET binding library initialized");
    });
}

/// Log macros for convenience
#[macro_export]
macro_rules! xet_debug {
    ($($arg:tt)*) => {
        tracing::debug!($($arg)*)
    };
}

#[macro_export]
macro_rules! xet_info {
    ($($arg:tt)*) => {
        tracing::info!($($arg)*)
    };
}

#[macro_export]
macro_rules! xet_warn {
    ($($arg:tt)*) => {
        tracing::warn!($($arg)*)
    };
}

#[macro_export]
macro_rules! xet_error {
    ($($arg:tt)*) => {
        tracing::error!($($arg)*)
    };
}
