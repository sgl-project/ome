package gcs

import (
	"context"
	"fmt"
	"time"
)

// isRetryableError determines if a GCS error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context errors
	if err == context.DeadlineExceeded {
		return true
	}

	// Default to not retryable
	return false
}

// retryWithBackoff retries an operation with exponential backoff
func retryWithBackoff(ctx context.Context, maxRetries int, initialDelay time.Duration, fn func() error) error {
	var lastErr error
	delay := initialDelay

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			return err
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Don't sleep after the last attempt
		if attempt < maxRetries-1 {
			select {
			case <-time.After(delay):
				// Exponential backoff with jitter
				delay = time.Duration(float64(delay) * 1.5)
				if delay > 30*time.Second {
					delay = 30 * time.Second
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, lastErr)
}
