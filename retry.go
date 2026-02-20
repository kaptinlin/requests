package requests

import (
	"math"
	"math/rand/v2"
	"net/http"
	"time"
)

const (
	// minBackoffDelay is the minimum backoff delay to prevent negative durations.
	minBackoffDelay = 0 * time.Second
)

// BackoffStrategy defines a function that returns the delay before the next retry.
type BackoffStrategy func(attempt int) time.Duration

// DefaultBackoffStrategy provides a simple constant delay between retries.
func DefaultBackoffStrategy(delay time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
		return delay
	}
}

// LinearBackoffStrategy increases the delay linearly with each retry attempt.
// The delay increments by `initialInterval` with each attempt.
func LinearBackoffStrategy(initialInterval time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
		return initialInterval * time.Duration(attempt+1)
	}
}

// ExponentialBackoffStrategy increases the delay exponentially with each retry attempt.
func ExponentialBackoffStrategy(initialInterval time.Duration, multiplier float64, maxBackoffTime time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
		delay := initialInterval * time.Duration(math.Pow(multiplier, float64(attempt)))
		return min(delay, maxBackoffTime)
	}
}

// JitterBackoffStrategy wraps a base backoff strategy and applies random jitter.
// The fraction parameter controls the jitter range: the delay is adjusted by ±fraction
// of the base delay. For example, a fraction of 0.25 means ±25% jitter.
func JitterBackoffStrategy(base BackoffStrategy, fraction float64) BackoffStrategy {
	return func(attempt int) time.Duration {
		delay := base(attempt)
		if fraction <= 0 {
			return delay
		}
		jitter := float64(delay) * fraction * (2*rand.Float64() - 1)
		result := time.Duration(float64(delay) + jitter)
		return max(result, minBackoffDelay)
	}
}

// RetryConfig defines the configuration for retrying requests.
type RetryConfig struct {
	MaxRetries int             // Maximum number of retry attempts
	Strategy   BackoffStrategy // The backoff strategy function
	RetryIf    RetryIfFunc     // Custom function to determine retry based on request and response
}

// RetryIfFunc defines the function signature for retry conditions.
type RetryIfFunc func(req *http.Request, resp *http.Response, err error) bool

// DefaultRetryIf is a simple retry condition that retries on 5xx status codes.
func DefaultRetryIf(req *http.Request, resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp != nil && resp.StatusCode >= 500
}
