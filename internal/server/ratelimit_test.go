package server

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"
)

func TestForwardedClientTrust(t *testing.T) {
	trusted, err := parseTrustedProxies(Config{TrustProxy: true, TrustedProxies: "10.0.0.0/24, ::1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ peer, header, want string }{
		{"10.0.0.1:80", "203.0.113.99, 198.51.100.10", "198.51.100.10"},
		{"10.0.0.1:80", "203.0.113.99, 198.51.100.10, 10.0.0.2", "198.51.100.10"},
		{"198.51.100.10:80", "203.0.113.99", "198.51.100.10"},
		{"10.0.0.1:80", "203.0.113.99, invalid", "10.0.0.1"},
		{"10.0.0.1:80", "invalid, 198.51.100.10", "198.51.100.10"},
		{"10.0.0.1:80", "", "10.0.0.1"},
		{"[::1]:80", "::ffff:198.51.100.10", "198.51.100.10"},
		{"[::1]:80", "2001:0db8:0:0::1", "2001:db8::1"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Forwarded-For", tc.header)
		if got := clientIP(r, trusted); got != tc.want {
			t.Errorf("peer=%s header=%q: got %s want %s", tc.peer, tc.header, got, tc.want)
		}
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:80"
	r.Header.Add("X-Forwarded-For", "203.0.113.99")
	r.Header.Add("X-Forwarded-For", "198.51.100.10")
	if got := clientIP(r, trusted); got != "198.51.100.10" {
		t.Errorf("multiple headers: %s", got)
	}
}

func TestTrustedProxyConfig(t *testing.T) {
	for _, value := range []string{"", "bad", "10.0.0.1,", "10.0.0.0/99"} {
		if _, err := parseTrustedProxies(Config{TrustProxy: true, TrustedProxies: value}); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	if prefixes, err := parseTrustedProxies(Config{TrustedProxies: "10.0.0.1"}); err != nil || len(prefixes) != 0 {
		t.Fatal("proxy trust should default to disabled")
	}
}

func TestLimiterCapacityAndExpiry(t *testing.T) {
	now := testInstant
	l := newLimiter(120, 3, func() time.Time { return now })
	for i := 0; i < maxBuckets; i++ {
		if !l.allow(fmt.Sprint(i)) {
			t.Fatalf("rejected initial client %d", i)
		}
	}
	// Consume a client's remaining burst. Overflow must not evict its debt.
	l.allow("0")
	l.allow("0")
	for i := 0; i < 100; i++ {
		if l.allow("overflow" + fmt.Sprint(i)) {
			t.Fatal("allowed overflow before expiry")
		}
	}
	if l.allow("0") {
		t.Fatal("overflow reset exhausted client")
	}
	if len(l.buckets) != maxBuckets || l.order.Len() != maxBuckets {
		t.Fatal("capacity exceeded")
	}
	now = now.Add(1499 * time.Millisecond)
	if l.allow("new") {
		t.Fatal("expired before fractional refill interval")
	}
	now = now.Add(time.Millisecond)
	if !l.allow("new") {
		t.Fatal("idle bucket was not reclaimed")
	}
	if len(l.buckets) != maxBuckets || l.order.Len() != maxBuckets {
		t.Fatal("capacity changed")
	}
}

func TestUntrustedPeerCannotRedirect(t *testing.T) {
	s := testServer(t)
	var err error
	s.trustedProxies, err = parseTrustedProxies(Config{TrustProxy: true, TrustedProxies: "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://example.com/tokyo", nil)
	r.Header.Set("User-Agent", browserUA)
	r.Header.Set("Accept", htmlAccept)
	r.Header.Set("X-Forwarded-Proto", "http")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	if rec.Code != 200 {
		t.Errorf("untrusted peer triggered status %d", rec.Code)
	}
}
