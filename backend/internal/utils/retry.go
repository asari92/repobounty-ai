package utils

import (
	"context"
	"time"
)

type RetryableFunc func(ctx context.Context) error

func Retry(ctx context.Context, maxAttempts int, delay time.Duration, fn RetryableFunc) error {
	var lastErr error

	for i := 0; i < maxAttempts; i++ {
		if i > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return lastErr
}
