import logging
import os
import sys

# Default log level
DEFAULT_LOG_LEVEL = "INFO"

# Get log level from environment variable or use default
log_level_name = os.environ.get("OME_LOG_LEVEL", DEFAULT_LOG_LEVEL).upper()
log_level = getattr(logging, log_level_name, logging.INFO)

# Configure logging format
log_format = "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
date_format = "%Y-%m-%d %H:%M:%S"

# --- Explicit Configuration for 'ome' logger (Restored) ---
# Create logger instance
logger = logging.getLogger(__name__.split(".")[0])  # Should resolve to 'ome'
logger.setLevel(log_level)  # Set level directly on our logger

# Create console handler
handler = logging.StreamHandler(sys.stderr)
handler.setLevel(log_level)  # Set level on handler

# Create formatter and add it to the handler
formatter = logging.Formatter(log_format, datefmt=date_format)
handler.setFormatter(formatter)

# Add the handler to the logger
# Prevent adding duplicate handlers if this module is imported multiple times
if not logger.hasHandlers():
    logger.addHandler(handler)

# Prevent propagation to root logger to avoid duplicate messages if root is also configured
logger.propagate = False
