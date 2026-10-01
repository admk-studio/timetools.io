package server

import (
	"container/list"
	"fmt"
	"net"
	"net/http"
	"net/netip"
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
	order   *list.List // least recently used first
}

type bucket struct {
	tokens float64
	last   time.Time
	entry  *list.Element
}

// maxBuckets is a hard bound. Only fully replenished idle buckets may be
// replaced, so rotating identities cannot reset active clients' limits.
const maxBuckets = 8192

func newLimiter(rpm, burst int, now func() time.Time) *limiter {
	return &limiter{
		buckets: make(map[string]*bucket),
		order:   list.New(),
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
		if len(l.buckets) >= maxBuckets {
			oldest := l.order.Front()
			old := l.buckets[oldest.Value.(string)]
			if now.Sub(old.last).Seconds()*l.rate < l.burst {
				return false
			}
			delete(l.buckets, oldest.Value.(string))
			l.order.Remove(oldest)
		}
		b = &bucket{tokens: l.burst, last: now, entry: l.order.PushBack(key)}
		l.buckets[key] = b
	}

	l.order.MoveToBack(b.entry)
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

// parseTrustedProxies requires explicit peers whenever forwarded headers are enabled.
func parseTrustedProxies(cfg Config) ([]netip.Prefix, error) {
	if !cfg.TrustProxy {
		return nil, nil
	}
	if strings.TrimSpace(cfg.TrustedProxies) == "" {
		return nil, fmt.Errorf("TT_TRUST_PROXY requires TT_TRUSTED_PROXIES (proxy IPs or CIDRs)")
	}
	var prefixes []netip.Prefix
	for _, value := range strings.Split(cfg.TrustedProxies, ",") {
		value = strings.TrimSpace(value)
		if ip, err := netip.ParseAddr(value); err == nil {
			ip = ip.Unmap()
			prefixes = append(prefixes, netip.PrefixFrom(ip, ip.BitLen()))
		} else if prefix, err := netip.ParsePrefix(value); err == nil {
			prefixes = append(prefixes, prefix.Masked())
		} else {
			return nil, fmt.Errorf("invalid trusted proxy %q", value)
		}
	}
	return prefixes, nil
}

func trustedIP(ip netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

func remoteIP(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip, _ := netip.ParseAddr(host)
	return ip.Unmap()
}

// Walk from the connected peer toward the client, stopping at the first
// untrusted address. An untrusted leftmost header value cannot override it.
// Malformed chains fall back to the connected peer, never a supplied identity.
func clientIP(r *http.Request, prefixes []netip.Prefix) string {
	peer := remoteIP(r)
	if !peer.IsValid() {
		return "unknown"
	}
	ip := peer
	values := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(values) - 1; i >= 0 && trustedIP(ip, prefixes); i-- {
		next, err := netip.ParseAddr(strings.TrimSpace(values[i]))
		if err != nil {
			return peer.String()
		}
		ip = next.Unmap()
	}
	return ip.String()
}
