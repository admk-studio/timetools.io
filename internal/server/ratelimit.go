package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// limiter is a per-client token bucket. State is in memory: a restart
// forgives everyone, which is the right trade for a service like this.
type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens added per second
	burst   float64 // bucket capacity
	now     func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// sweepThreshold bounds memory: when the map grows past this we drop
// buckets that have been idle long enough to be full again anyway.
const sweepThreshold = 8192

func newLimiter(rpm, burst int, now func() time.Time) *limiter {
	return &limiter{
		buckets: make(map[string]*bucket),
		rate:    float64(rpm) / 60,
		burst:   float64(burst),
		now:     now,
	}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= sweepThreshold {
			l.sweepLocked(now)
		}
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *limiter) sweepLocked(now time.Time) {
	idle := time.Duration(l.burst/l.rate) * time.Second
	for key, b := range l.buckets {
		if now.Sub(b.last) > idle {
			delete(l.buckets, key)
		}
	}
}

// clientIP identifies the caller for rate limiting. Behind a reverse
// proxy the remote address is the proxy itself, so trust the first entry
// of X-Forwarded-For — but only when configured to, because that header
// is attacker-controlled on a directly exposed server.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			if ip := strings.TrimSpace(first); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
