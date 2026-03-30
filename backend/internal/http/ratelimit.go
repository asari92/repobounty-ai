package http

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	limits map[string]*rateLimitEntry
	mu     sync.RWMutex
	r      rate.Limit
	b      int
}

func NewRateLimiter(rps int, burst int) *RateLimiter {
	return &RateLimiter{
		limits: make(map[string]*rateLimitEntry),
		r:      rate.Limit(rps),
		b:      burst,
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.limits[ip]
	if !exists {
		entry = &rateLimitEntry{limiter: rate.NewLimiter(rl.r, rl.b), lastSeen: time.Now()}
		rl.limits[ip] = entry
	} else {
		entry.lastSeen = time.Now()
	}
	return entry.limiter
}

func (rl *RateLimiter) cleanup(ctx context.Context) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for k, entry := range rl.limits {
		if entry.lastSeen.Before(cutoff) {
			delete(rl.limits, k)
		}
	}
}

var rateLimitCancel context.CancelFunc

func StopRateLimitCleanup() {
	if rateLimitCancel != nil {
		rateLimitCancel()
	}
}

func RateLimitMiddleware(rps int, burst int) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(rps, burst)

	ctx, cancel := context.WithCancel(context.Background())
	rateLimitCancel = cancel

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				limiter.cleanup(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			if !limiter.getLimiter(ip).Allow() {
				logger.Warn("rate limit exceeded",
					zap.String("ip", ip),
					zap.String("path", r.URL.Path),
					zap.String("method", r.Method),
				)

				w.Header().Set("X-RateLimit-Limit", "100")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", time.Now().Add(time.Minute).Format(time.RFC1123))
				w.Header().Set("Retry-After", "60")

				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"Too many requests"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
