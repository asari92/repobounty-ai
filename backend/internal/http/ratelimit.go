package http

import (
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	entries map[string]*ipEntry
	mu      sync.Mutex
	r       rate.Limit
	b       int
}

func NewRateLimiter(rps int, burst int) *RateLimiter {
	return &RateLimiter{
		entries: make(map[string]*ipEntry),
		r:       rate.Limit(rps),
		b:       burst,
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.entries[ip]
	if !exists {
		entry = &ipEntry{
			limiter:  rate.NewLimiter(rl.r, rl.b),
			lastSeen: time.Now(),
		}
		rl.entries[ip] = entry
	} else {
		entry.lastSeen = time.Now()
	}
	return entry.limiter
}

func (rl *RateLimiter) cleanup(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, entry := range rl.entries {
		if now.Sub(entry.lastSeen) > maxAge {
			delete(rl.entries, ip)
		}
	}
}

func RateLimitMiddleware(rps int, burst int) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(rps, burst)

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			limiter.cleanup(10 * time.Minute)
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
