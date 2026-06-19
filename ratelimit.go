package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateLimiter is a simple in-memory, per-IP fixed-window rate limiter.
// Suitable for a single-instance self-hosted deployment. For multi-instance
// setups, front it with Nginx limit_req (see deploy/nginx.conf).
type rateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	max     int
	window  time.Duration
	lastGC  time.Time
}

func newRateLimiter(max int, window time.Duration) gin.HandlerFunc {
	rl := &rateLimiter{
		hits:   make(map[string][]time.Time),
		max:    max,
		window: window,
		lastGC: time.Now(),
	}
	return rl.middleware
}

func (rl *rateLimiter) middleware(c *gin.Context) {
	ip := c.ClientIP()
	now := time.Now()

	rl.mu.Lock()
	// Occasional garbage collection of stale entries
	if now.Sub(rl.lastGC) > rl.window {
		for k, times := range rl.hits {
			if len(times) == 0 || now.Sub(times[len(times)-1]) > rl.window {
				delete(rl.hits, k)
			}
		}
		rl.lastGC = now
	}

	// Keep only hits within the window
	cutoff := now.Add(-rl.window)
	recent := rl.hits[ip][:0]
	for _, t := range rl.hits[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= rl.max {
		rl.mu.Unlock()
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests — please try again in a minute"})
		c.Abort()
		return
	}

	recent = append(recent, now)
	rl.hits[ip] = recent
	rl.mu.Unlock()

	c.Next()
}
