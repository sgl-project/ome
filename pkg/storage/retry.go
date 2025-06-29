package storage

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryOperation executes an operation with exponential backoff retry logic
func RetryOperation(ctx context.Context, config RetryConfig, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute the operation
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !config.RetryableError(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Check if this was the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := calculateDelay(attempt, config)

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next retry
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, lastErr)
}

// calculateDelay calculates the delay for the next retry attempt
func calculateDelay(attempt int, config RetryConfig) time.Duration {
	// Calculate exponential backoff
	delay := float64(config.InitialDelay) * math.Pow(config.Multiplier, float64(attempt))

	// Cap at maximum delay
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	// Add jitter if enabled
	if config.Jitter {
		// Add random jitter between 0% and 25% of the delay
		jitter := rand.Float64() * 0.25 * delay
		delay += jitter
	}

	return time.Duration(delay)
}

// RetryWithBackoff is a convenience function for retrying with default configuration
func RetryWithBackoff(ctx context.Context, operation func() error) error {
	return RetryOperation(ctx, DefaultRetryConfig(), operation)
}
