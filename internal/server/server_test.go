package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var testInstant = time.Date(2024, 7, 16, 13, 41, 7, 0, time.UTC)

func testServer(t *testing.T) *Server {
	t.Helper()
	cfg := FromEnv()
	cfg.Version = "test"
	s, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return testInstant }
	return s
}

// get performs a request as a given kind of client and returns the
// response and body.
func get(t *testing.T, s *Server, path, userAgent, accept string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	resp := rec.Result()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(body)
}

const (
	curlUA     = "curl/8.5.0"
	browserUA  = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36"
	htmlAccept = "text/html,application/xhtml+xml,*/*;q=0.8"
)

func TestCityText(t *testing.T) {
	s := testServer(t)
	resp, body := get(t, s, "/tokyo", curlUA, "*/*")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content type = %q", ct)
	}
	for _, want := range []string{"Tokyo", "Asia/Tokyo", "22:41:07", "UTC+09:00"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
}

func TestCityHTML(t *testing.T) {
	s := testServer(t)
	resp, body := get(t, s, "/tokyo", browserUA, htmlAccept)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content type = %q", ct)
	}
	for _, want := range []string{"<!doctype html>", "Tokyo", "Asia/Tokyo", "data-tz=\"Asia/Tokyo\""} {
		if !strings.Contains(body, want) {
			t.Errorf("html missing %q", want)
		}
	}
	if resp.Header.Get("Content-Security-Policy") == "" {
		t.Error("HTML page missing CSP header")
	}
}

func TestCityJSON(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/tokyo?format=json", "/tokyo"} {
		accept := "*/*"
		if !strings.Contains(path, "format") {
			accept = "application/json"
		}
		resp, body := get(t, s, path, curlUA, accept)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		var parsed struct {
			Locations []struct {
				Timezone string `json:"timezone"`
				Unix     int64  `json:"unix"`
			} `json:"locations"`
		}
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			t.Fatalf("bad JSON for %s: %v", path, err)
		}
		if len(parsed.Locations) != 1 || parsed.Locations[0].Timezone != "Asia/Tokyo" {
			t.Errorf("unexpected payload: %s", body)
		}
		if parsed.Locations[0].Unix != testInstant.Unix() {
			t.Errorf("unix = %d, want %d", parsed.Locations[0].Unix, testInstant.Unix())
		}
		if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
			t.Error("JSON response missing CORS header")
		}
	}
}

func TestMultiCity(t *testing.T) {
	s := testServer(t)
	_, body := get(t, s, "/nyc/london/tokyo", curlUA, "*/*")
	for _, want := range []string{"New York", "London", "Tokyo", "working hours"} {
		if !strings.Contains(body, want) {
			t.Errorf("comparison missing %q:\n%s", want, body)
		}
	}
}

func TestFullIANAPath(t *testing.T) {
	// A path with a slash that is itself one zone must not be split.
	s := testServer(t)
	_, body := get(t, s, "/america/new_york", curlUA, "*/*")
	if !strings.Contains(body, "America/New_York") {
		t.Errorf("IANA path not resolved as one zone:\n%s", body)
	}
	if strings.Contains(body, "working hours") {
		t.Error("single zone rendered as comparison table")
	}
}

func TestNotFound(t *testing.T) {
	s := testServer(t)
	resp, body := get(t, s, "/tokio", curlUA, "*/*")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if !strings.Contains(body, "tokyo") {
		t.Errorf("404 body should suggest tokyo:\n%s", body)
	}

	resp, body = get(t, s, "/tokio", curlUA, "application/json")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("json status = %d, want 404", resp.StatusCode)
	}
	if !strings.Contains(body, `"suggestions"`) {
		t.Errorf("json 404 missing suggestions: %s", body)
	}
}

func TestSpecialEndpoints(t *testing.T) {
	s := testServer(t)

	if _, body := get(t, s, "/unix", curlUA, "*/*"); strings.TrimSpace(body) != "1721137267" {
		t.Errorf("/unix = %q", body)
	}
	if _, body := get(t, s, "/utc", curlUA, "*/*"); strings.TrimSpace(body) != "2024-07-16T13:41:07Z" {
		t.Errorf("/utc = %q", body)
	}
	if resp, body := get(t, s, "/health", curlUA, "*/*"); resp.StatusCode != 200 || strings.TrimSpace(body) != "ok" {
		t.Errorf("/health = %d %q", resp.StatusCode, body)
	}
	if _, body := get(t, s, "/version", curlUA, "*/*"); strings.TrimSpace(body) != "test" {
		t.Errorf("/version = %q", body)
	}
	if _, body := get(t, s, "/", curlUA, "*/*"); !strings.Contains(body, "curl timetools.io/tokyo") {
		t.Errorf("root help missing examples:\n%s", body)
	}
	if _, body := get(t, s, "/", browserUA, htmlAccept); !strings.Contains(body, "<!doctype html>") {
		t.Error("root for a browser should be HTML")
	}
}

func TestZones(t *testing.T) {
	s := testServer(t)
	_, body := get(t, s, "/zones?q=india", curlUA, "*/*")
	if !strings.Contains(body, "Asia/Kolkata") {
		t.Errorf("/zones?q=india missing Asia/Kolkata:\n%s", body)
	}
	resp, _ := get(t, s, "/zones?q=zzzz", curlUA, "*/*")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("/zones with no matches = %d, want 404", resp.StatusCode)
	}
	_, all := get(t, s, "/zones", curlUA, "*/*")
	if !strings.Contains(all, "312 zones") {
		t.Errorf("/zones should list all 312:\n%s", all[len(all)-40:])
	}
}

func TestMethodNotAllowed(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodPost, "/tokyo", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d, want 405", rec.Code)
	}
}

func TestTooManyCities(t *testing.T) {
	s := testServer(t)
	resp, _ := get(t, s, "/"+strings.Repeat("tokyo/", 11)+"tokyo", curlUA, "*/*")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("11+ cities = %d, want 404", resp.StatusCode)
	}
}

func TestPlainOption(t *testing.T) {
	s := testServer(t)
	_, colored := get(t, s, "/tokyo", curlUA, "*/*")
	if !strings.Contains(colored, "\x1b[") {
		t.Error("default output should use color")
	}
	_, plain := get(t, s, "/tokyo?plain", curlUA, "*/*")
	if strings.Contains(plain, "\x1b[") {
		t.Error("?plain output must not use color")
	}
}

func TestNegotiate(t *testing.T) {
	tests := []struct {
		ua, accept, query string
		want              format
	}{
		{curlUA, "*/*", "", formatText},
		{"Wget/1.21", "*/*", "", formatText},
		{browserUA, htmlAccept, "", formatHTML},
		{browserUA, htmlAccept, "format=text", formatText},
		{curlUA, "application/json", "", formatJSON},
		{curlUA, "*/*", "format=json", formatJSON},
		{browserUA, htmlAccept, "format=json", formatJSON},
		// Unknown clients get text: mangled text in a browser beats
		// HTML soup in a terminal.
		{"SomeNewTool/1.0", "*/*", "", formatText},
	}
	for _, tt := range tests {
		url := "/tokyo"
		if tt.query != "" {
			url += "?" + tt.query
		}
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("User-Agent", tt.ua)
		req.Header.Set("Accept", tt.accept)
		if got := negotiate(req); got != tt.want {
			t.Errorf("negotiate(ua=%q accept=%q query=%q) = %d, want %d",
				tt.ua, tt.accept, tt.query, got, tt.want)
		}
	}
}

func TestRateLimit(t *testing.T) {
	clock := testInstant
	l := newLimiter(60, 5, func() time.Time { return clock })

	// The burst allows the first five, then we're dry.
	for i := 0; i < 5; i++ {
		if !l.allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	if l.allow("1.2.3.4") {
		t.Error("burst exhausted, should be denied")
	}
	// Other clients are unaffected.
	if !l.allow("5.6.7.8") {
		t.Error("second client should be allowed")
	}
	// One second at 60 rpm refills one token.
	clock = clock.Add(time.Second)
	if !l.allow("1.2.3.4") {
		t.Error("token should have refilled")
	}
	if l.allow("1.2.3.4") {
		t.Error("only one token should have refilled")
	}
}

func TestRateLimitEndToEnd(t *testing.T) {
	cfg := FromEnv()
	cfg.RateRPM, cfg.RateBurst = 60, 3
	s, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return testInstant }
	s.limiter = newLimiter(cfg.RateRPM, cfg.RateBurst, s.now)

	var last *http.Response
	for i := 0; i < 4; i++ {
		last, _ = get(t, s, "/tokyo", curlUA, "*/*")
	}
	if last.StatusCode != http.StatusTooManyRequests {
		t.Errorf("4th request = %d, want 429", last.StatusCode)
	}
	// Health stays reachable even when throttled.
	if resp, _ := get(t, s, "/health", curlUA, "*/*"); resp.StatusCode != http.StatusOK {
		t.Errorf("health while throttled = %d, want 200", resp.StatusCode)
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:5555"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")

	if ip := clientIP(req, false); ip != "10.0.0.1" {
		t.Errorf("untrusted proxy: ip = %q, want 10.0.0.1", ip)
	}
	if ip := clientIP(req, true); ip != "203.0.113.9" {
		t.Errorf("trusted proxy: ip = %q, want 203.0.113.9", ip)
	}
}
