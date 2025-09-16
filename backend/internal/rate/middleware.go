package rate

import (
    "net/http"
    "sync"
    "time"
)

type tokenBucket struct {
    tokens float64
    last time.Time
}

type Limiter struct {
    rps float64
    mu sync.Mutex
    buckets map[string]*tokenBucket
}

func NewLimiter(rps int) *Limiter {
    return &Limiter{rps: float64(rps), buckets: make(map[string]*tokenBucket)}
}

func (l *Limiter) Allow(key string) bool {
    l.mu.Lock()
    defer l.mu.Unlock()
    b := l.buckets[key]
    now := time.Now()
    if b == nil { b = &tokenBucket{tokens: 1, last: now}; l.buckets[key] = b }
    // refill
    elapsed := now.Sub(b.last).Seconds()
    b.tokens += elapsed * l.rps
    if b.tokens > l.rps { b.tokens = l.rps }
    b.last = now
    if b.tokens >= 1 {
        b.tokens -= 1
        return true
    }
    return false
}

func (l *Limiter) Middleware(getKey func(*http.Request) string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            key := getKey(r)
            if key == "" { key = r.RemoteAddr }
            if !l.Allow(key) {
                w.Header().Set("Retry-After", "1")
                http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}



