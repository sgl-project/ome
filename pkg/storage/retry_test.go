package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRetryOperation(t *testing.T) {
	tests := []struct {
		name          string
		attempts      int
		maxRetries    int
		shouldSucceed bool
		retryableErr  bool
		expectedCalls int
	}{
		{
			name:          "Success on first attempt",
			attempts:      1,
			maxRetries:    3,
			shouldSucceed: true,
			retryableErr:  true,
			expectedCalls: 1,
		},
		{
			name:          "Success after retries",
			attempts:      3,
			maxRetries:    3,
			shouldSucceed: true,
			retryableErr:  true,
			expectedCalls: 3,
		},
		{
			name:          "Failure after max retries",
			attempts:      5,
			maxRetries:    3,
			shouldSucceed: false,
			retryableErr:  true,
			expectedCalls: 4, // initial + 3 retries
		},
		{
			name:          "Non-retryable error",
			attempts:      5,
			maxRetries:    3,
			shouldSucceed: false,
			retryableErr:  false,
			expectedCalls: 1, // stops after first error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			operation := func() error {
				callCount++
				if callCount >= tt.attempts {
					return nil
				}
				return errors.New("operation failed")
			}

			config := RetryConfig{
				MaxRetries:   tt.maxRetries,
				InitialDelay: 1 * time.Millisecond,
				MaxDelay:     10 * time.Millisecond,
				Multiplier:   2.0,
				Jitter:       false,
				RetryableError: func(err error) bool {
					return tt.retryableErr
				},
			}

			err := RetryOperation(context.Background(), config, operation)

			if tt.shouldSucceed && err != nil {
				t.Errorf("Expected success but got error: %v", err)
			}
			if !tt.shouldSucceed && err == nil {
				t.Error("Expected error but got success")
			}
			if callCount != tt.expectedCalls {
				t.Errorf("Expected %d calls but got %d", tt.expectedCalls, callCount)
			}
		})
	}
}

func TestRetryOperationWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	operation := func() error {
		callCount++
		if callCount == 2 {
			cancel() // Cancel context on second attempt
		}
		return errors.New("operation failed")
	}

	config := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		RetryableError: func(err error) bool {
			return true
		},
	}

	err := RetryOperation(ctx, config, operation)

	if err == nil {
		t.Error("Expected error but got success")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context canceled error but got: %v", err)
	}
	if callCount > 3 {
		t.Errorf("Expected at most 3 calls but got %d", callCount)
	}
}

func TestCalculateDelay(t *testing.T) {
	config := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}

	tests := []struct {
		attempt     int
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		{0, 100 * time.Millisecond, 100 * time.Millisecond},
		{1, 200 * time.Millisecond, 200 * time.Millisecond},
		{2, 400 * time.Millisecond, 400 * time.Millisecond},
		{3, 800 * time.Millisecond, 800 * time.Millisecond},
		{10, 5 * time.Second, 5 * time.Second}, // Should be capped at MaxDelay
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			delay := calculateDelay(tt.attempt, config)
			if delay < tt.expectedMin || delay > tt.expectedMax {
				t.Errorf("Expected delay between %v and %v, got %v",
					tt.expectedMin, tt.expectedMax, delay)
			}
		})
	}
}

func TestCalculateDelayWithJitter(t *testing.T) {
	config := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}

	// Test that jitter adds variability
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = calculateDelay(2, config) // Same attempt number
	}

	// Check that not all delays are the same (jitter is working)
	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected jitter to produce different delays, but all were the same")
	}

	// Check that delays are within expected range
	baseDelay := 400 * time.Millisecond // 100ms * 2^2
	maxJitteredDelay := time.Duration(float64(baseDelay) * 1.25)

	for _, delay := range delays {
		if delay < baseDelay || delay > maxJitteredDelay {
			t.Errorf("Delay %v is outside expected range [%v, %v]",
				delay, baseDelay, maxJitteredDelay)
		}
	}
}
