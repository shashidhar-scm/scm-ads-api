package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	window  time.Duration
	max     int
	mu      sync.Mutex
	clients map[string]*rateEntry
}

type rateEntry struct {
	count   int
	expires time.Time
}

func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	if max <= 0 || window <= 0 {
		return nil
	}
	return &RateLimiter{
		window:  window,
		max:     max,
		clients: make(map[string]*rateEntry),
	}
}

func (l *RateLimiter) allow(key string) bool {
	if l == nil {
		return true
	}

	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.clients[key]
	if !ok || now.After(entry.expires) {
		l.clients[key] = &rateEntry{count: 1, expires: now.Add(l.window)}
		return true
	}

	if entry.count >= l.max {
		return false
	}

	entry.count++
	return true
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	if l == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientKey(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientKey(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}

	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
