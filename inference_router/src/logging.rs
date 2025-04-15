use tracing_subscriber::layer::SubscriberExt;
use tracing_subscriber::util::SubscriberInitExt;
use tracing_subscriber::{filter::LevelFilter, EnvFilter, Layer};

/// Init logging using env variables LOG_LEVEL and LOG_FORMAT:
///     - LOG_LEVEL may be TRACE, DEBUG, INFO, WARN or ERROR (default to INFO)
///     - LOG_FORMAT may be TEXT or JSON (default to TEXT)
///     - LOG_COLORIZE may be "false" or "true" (default to "true" or ansi supported platforms)
/// 
/// If LOG_LEVEL environment variable is not set, the log_level parameter will be used.
pub fn init_logging(json_output: bool, log_level: &str) {
    let mut layers = Vec::new();

    // STDOUT/STDERR layer
    let ansi = std::env::var("LOG_COLORIZE") != Ok("1".to_string());
    let fmt_layer = tracing_subscriber::fmt::layer()
        .with_file(true)
        .with_ansi(ansi)
        .with_line_number(true);

    let fmt_layer = match json_output {
        true => fmt_layer.json().flatten_event(true).boxed(),
        false => fmt_layer.boxed(),
    };
    layers.push(fmt_layer);

    // Filter events with LOG_LEVEL
    let varname = "LOG_LEVEL";
    let env_filter = if let Ok(env_log_level) = std::env::var(varname) {
        // Environment variable takes precedence
        // Note: We can't log here because the logger isn't initialized yet
        
        // Override to avoid simple logs to be spammed with tokio level information
        let log_level = match &env_log_level[..] {
            "warn" => "inference_router=warn,text_generation_router=warn",
            "info" => "inference_router=info,text_generation_router=info",
            "debug" => "inference_router=debug,text_generation_router=debug",
            log_level => log_level,
        };
        EnvFilter::builder()
            .with_default_directive(LevelFilter::INFO.into())
            .parse_lossy(log_level)
    } else {
        // Use provided log_level parameter
        // Note: We can't log here because the logger isn't initialized yet
        
        // Apply the same overrides as for environment variables
        let log_level_str = match log_level.to_lowercase().as_str() {
            "warn" => "inference_router=warn,text_generation_router=warn",
            "info" => "inference_router=info,text_generation_router=info",
            "debug" => "inference_router=debug,text_generation_router=debug",
            _ => log_level,
        };
        EnvFilter::builder()
            .with_default_directive(LevelFilter::INFO.into())
            .parse_lossy(log_level_str)
    };

    tracing_subscriber::registry()
        .with(env_filter)
        .with(layers)
        .init();
}