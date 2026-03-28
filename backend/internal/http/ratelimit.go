package http

import (
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limits map[string]*rate.Limiter
	mu     sync.RWMutex
	r      rate.Limit
	b      int
}

func NewRateLimiter(rps int, burst int) *RateLimiter {
	return &RateLimiter{
		limits: make(map[string]*rate.Limiter),
		r:      rate.Limit(rps),
		b:      burst,
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limits[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.r, rl.b)
		rl.limits[ip] = limiter
	}
	return limiter
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for k := range rl.limits {
		delete(rl.limits, k)
	}
}

func RateLimitMiddleware(rps int, burst int) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(rps, burst)

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			limiter.cleanup()
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
