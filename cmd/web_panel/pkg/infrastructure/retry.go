package infrastructure

import (
	"fmt"
	"time"
)

// RetryConnect retries connectFn up to maxAttempts, waiting delay each time.
func RetryConnect[T any](maxAttempts int, delay time.Duration, connectFn func() (T, error)) (T, error) {
	var zero T
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err := connectFn()
		if err == nil {
			return conn, nil
		}
		lastErr = err
		fmt.Printf("Connect attempt %d/%d failed: %v\n", attempt, maxAttempts, err)
		time.Sleep(delay)
	}
	return zero, fmt.Errorf("all attempts failed: %w", lastErr)
}
